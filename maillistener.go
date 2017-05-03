package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/mail"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

type JSONMessage struct {
	From    string      `json:"from"`
	To      []string    `json:"to"`
	Subject string      `json:"subject"`
	Headers mail.Header `json:"headers"`
	Body    string      `json:"body"`
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

	var body string
	if (headers.Get("Content-Type") == "" || strings.HasPrefix(headers.Get("Content-Type"), "text/plain")) &&
		(headers.Get("Content-Transfer-Encoding") == "7bit" || headers.Get("Content-Transfer-Encoding") == "") {

		b, err := ioutil.ReadAll(message.Body)
		if err == nil {
			body = string(b)
		}
	}

	json_message := JSONMessage{
		From:    from_address,
		To:      to_addresses,
		Subject: headers.Get("Subject"),
		Headers: message.Header,
		Body:    body,
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
