package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net/mail"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

type JSONMessage struct {
	From        string      `json:"from"`
	To          []string    `json:"to"`
	DeliveredTo string      `json:"delivered_to"`
	Subject     string      `json:"subject"`
	Headers     mail.Header `json:"headers"`
	Body        string      `json:"body"`
}

func ParseBody(message mail.Message) string {
	headers := message.Header
	var body string
	mediaType, params, err := mime.ParseMediaType(headers.Get("Content-Type"))
	if err != nil {
		log.Fatal(err)
		return ""
	}

	switch {
	case mediaType == "" || mediaType == "text/plain":
		b, err := ioutil.ReadAll(message.Body)
		if headers.Get("Content-Transfer-Encoding") == "7bit" || headers.Get("Content-Transfer-Encoding") == "" {
			if err == nil {
				body = string(b)
			}
		} else if headers.Get("Content-Transfer-Encoding") == "base64" {
			data, err := base64.StdEncoding.DecodeString(string(b))
			if err != nil {
				log.Fatal(err)
				return ""
			}
			body = string(data)
		}
	case strings.HasPrefix(mediaType, "multipart/"):
		mr := multipart.NewReader(message.Body, params["boundary"])
		for {
			p, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatal(err)
				return ""
			}

			partMediaType, _, err := mime.ParseMediaType(p.Header.Get("Content-Type"))
			if partMediaType == "" || partMediaType == "text/plain" {
				b, err := ioutil.ReadAll(p)
				if headers.Get("Content-Transfer-Encoding") == "7bit" || headers.Get("Content-Transfer-Encoding") == "" {
					if err == nil {
						body = string(b)
					}
				} else if headers.Get("Content-Transfer-Encoding") == "base64" {
					data, err := base64.StdEncoding.DecodeString(string(b))
					if err != nil {
						log.Fatal(err)
						return ""
					}
					body = string(data)
				}
			}
		}
	}
	return body
}

func CreateJSONMail(raw_message MessageStruct) []byte {
	message := raw_message.Mail
	headers := message.Header
	var from_address string
	var to_addresses []string
	if parse, err := mail.ParseAddress(headers.Get("From")); err == nil {
		from_address = parse.String()
	} else {
		from_address = headers.Get("From")
	}

	if parse, err := mail.ParseAddressList(headers.Get("To")); err == nil {
		for _, a := range parse {
			to_addresses = append(to_addresses, a.String())
		}
	} else {
		to_addresses = []string{raw_message.To.String()}
	}

	json_message := JSONMessage{
		From:        from_address,
		To:          to_addresses,
		DeliveredTo: raw_message.To.String(),
		Subject:     headers.Get("Subject"),
		Headers:     message.Header,
		Body:        ParseBody(message),
	}

	content, err := json.Marshal(json_message)
	check(err)
	return content
}

func Listen(ch chan MessageStruct, config Settings) {
	for {
		message := <-ch

		maildir := path.Join(config.MailboxDir, message.To.Host, message.To.User)
		filename := path.Join(maildir, fmt.Sprintf("%d.eml", time.Now().UnixNano()))

		check(os.MkdirAll(maildir, 0700))

		ioutil.WriteFile(filename, message.RawMail.Bytes(), 0600)

		cmd := exec.Command("./fwdtelegram.py")
		cmd.Env = append(os.Environ(), "TELESMTP_MAIL_FILE="+filename)

		stdin, err := cmd.StdinPipe()
		if err != nil {
			fmt.Println(err)
		}

		var out []byte
		go func() {
			stdin.Write(CreateJSONMail(message))
			stdin.Close()
			out, err = cmd.CombinedOutput()
			if err != nil {
				fmt.Println(err)
			}
			fmt.Printf("fwdtelegram.py: %s\n", out)
		}()
	}
}
