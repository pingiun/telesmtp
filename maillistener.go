package main

import (
	"fmt"
	"io/ioutil"
	"encoding/json"
	"net/mail"
	"path"
	"time"
	"os"
	"os/exec"
)

type JSONMessage struct {
	From string `json:"from"`
	Subject string `json:"subject"`
	Headers mail.Header `json:"headers"`
	Body string `body:"body"`
}

func CreateJSONMail(message mail.Message) []byte {
	headers := message.Header
	var from_adress string 
	if parse, err := mail.ParseAddress(headers.Get("From")); err == nil {
		from_adress = parse.String()
	} else {
		from_adress = headers.Get("From")
	}

	var body string
	if b, err := ioutil.ReadAll(message.Body); err == nil {
	    body = string(b)
	}
	json_message := JSONMessage{
		From: from_adress,
		Subject: headers.Get("Subject"),
		Headers: message.Header,
		Body: body,
	}
	content, _ := json.Marshal(json_message)
	return content
}

func Listen(ch chan MessageStruct, config Settings) {
	for {
		message := <-ch
		
		maildir := path.Join(config.MailboxDir, message.To.Host, message.To.User)
		filename := path.Join(maildir, fmt.Sprintf("%d.eml", time.Now().UnixNano()))

		check(os.MkdirAll(maildir, 0700))
		
		ioutil.WriteFile(filename,
			message.RawMail.Bytes(), 0600)

		cmd := exec.Command("./fwdtelegram.py")
		cmd.Env = append(os.Environ(), "TELESMTP_MAIL_FILE="+filename)

		stdin, err := cmd.StdinPipe()
		if err != nil {
			fmt.Println(err)
		}

		var out []byte
		go func() {
			stdin.Write(CreateJSONMail(message.Mail))
			stdin.Close()
			out, err = cmd.CombinedOutput()
			if err != nil {
				fmt.Println(err)
			}
			fmt.Printf("fwdtelegram.py: %s\n", out)
		}()
	}
}
