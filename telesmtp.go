package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/mail"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/fatih/color"

	"gopkg.in/yaml.v2"
)

const TELESMTP_VERSION = "0.0.1"

type Mode int

const (
	ModeInitial        Mode = iota // Initial mode
	ModeIdentified                 // Mode after identification
	ModeMail                       // MAIL command has been sent
	ModeRcpt                       // RCPT command has been sent
	ModeData                       // User is sending mail data
	ModeTLSNegotiation             // Mode during tls negotiation
)

type Command int

const (
	CommandNOOP Command = iota
	CommandEHLO
	CommandHELO
	CommandHELP
	CommandQUIT
	CommandMAIL
	CommandRCPT
	CommandDATA
	CommandRSET
	CommandUnknown
)

func (c Command) String() string {
	switch c {
	case CommandNOOP:
		return "NOOP"
	case CommandEHLO:
		return "EHLO"
	case CommandHELO:
		return "HELO"
	case CommandHELP:
		return "HELP"
	case CommandQUIT:
		return "QUIT"
	case CommandMAIL:
		return "MAIL"
	case CommandRCPT:
		return "RCPT"
	case CommandDATA:
		return "DATA"
	case CommandRSET:
		return "RSET"
	default:
		return ""
	}
}

type MessageStruct struct {
	Mail    mail.Message
	RawMail bytes.Buffer
	To      Address
	From    Address
}

type Address struct {
	User string
	Host string
}

func (a Address) String() string {
	return fmt.Sprintf("%s@%s", a.User, a.Host)
}

type Client struct {
	Mode     Mode
	Addr     net.Addr
	Host     string
	Identity string
	Ip       string
	From     Address
	To       Address
	Mail     bytes.Buffer
}

type Settings struct {
	Hostname     string
	Port         uint16
	ValidDomains []string `yaml:"valid_domains"`
	MailboxDir   string   `yaml:"mailbox_dir"`
}

func getCommand(input string) (command Command, args []string) {
	split := strings.Split(strings.TrimSpace(input), " ")

	switch strings.ToUpper(split[0]) {
	case "NOOP":
		return CommandNOOP, nil
	case "EHLO":
		return CommandEHLO, split[1:]
	case "HELO":
		return CommandHELO, split[1:]
	case "HELP":
		return CommandHELP, split[1:]
	case "QUIT":
		return CommandQUIT, nil
	case "MAIL":
		return CommandMAIL, split[1:]
	case "RCPT":
		return CommandRCPT, split[1:]
	case "DATA":
		return CommandDATA, nil
	case "RSET":
		return CommandRSET, nil
	}
	return CommandUnknown, nil
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func write(c net.Conn, fstring string, args ...interface{}) {
	fmt.Println(fstring)
	c.Write([]byte(fmt.Sprintf(fstring, args...)))
	c.Write([]byte("\r\n"))
}

func parseDomain(args []string) (ip string, err error) {
	return ip, nil
	// return nil, errors.New("parseDomain: invalid domain")
}

func handleHELP(c net.Conn, status *Client, config Settings, command Command, args []string) {
	if len(args) == 0 {
		write(c, "214-This is telesmtp version %s", TELESMTP_VERSION)
		write(c, "214-HELP topics:")
		write(c, "214-\tHELO\tEHLO")
		write(c, "214-For more info use \"HELP <topic>\".")
		write(c, "214-To report bugs in the implementation see")
		write(c, "214-\thttps://pingiun.com/post/telesmtp")
		write(c, "214-For local information send email to Postmaster at your site.")
		write(c, "214 End of HELP info")
	} else {
		write(c, "501 No info available for %s", args[0])
	}
}

func handleHELO(c net.Conn, status *Client, config Settings, command Command, args []string) {
	identity, err := parseDomain(args)
	if len(args) == 0 {
		write(c, "501 %s requires domain address", command)
	} else if err != nil {
		write(c, "501 Invalid domain name")
	} else {
		status.Identity = identity
		status.Mode = ModeIdentified
		if command == CommandEHLO {
			write(c, "250-%s here, welcome %s [%s], pleased to meet you", config.Hostname, status.Host, status.Addr)
			write(c, "250 HELP")
		} else {
			write(c, "250 %s at your service", config.Hostname)
		}
	}
}

func handleQUIT(c net.Conn, config Settings) {
	write(c, "221 %s closing connection", config.Hostname)
	c.Close()
}

func handleRSET(c net.Conn, status *Client) {
	status.Mode = ModeInitial
	write(c, "250 Reset state")
}

func handleModeInitial(c net.Conn, status *Client, config Settings, line string) {
	command, args := getCommand(line)

	switch command {
	case CommandNOOP:
		write(c, "250 OK")
	case CommandEHLO:
		handleHELO(c, status, config, command, args)
	case CommandHELO:
		handleHELO(c, status, config, command, args)
	case CommandHELP:
		handleHELP(c, status, config, command, args)
	case CommandQUIT:
		handleQUIT(c, config)
	case CommandRSET:
		handleRSET(c, status)
	case CommandUnknown:
		write(c, "500 Unrecognized command.")
	default:
		write(c, "503 EHLO/HELO first.")
	}
}

func parseAddress(args []string, from bool) (address Address, err error) {
	if len(args) == 0 {
		return Address{}, errors.New("parseAddress: no arguments")
	}
	re := regexp.MustCompile(`<\s*(.+)@(.+)\s*>`)
	if len(args) == 1 {
		split := strings.SplitN(args[0], ":", 2)
		if split[0] != "FROM" && from {
			return Address{}, errors.New("parseAddress: invalid arguments")
		}
		if split[0] != "TO" && !from {
			return Address{}, errors.New("parseAddress: invalid arguments")
		}

		parts := re.FindStringSubmatch(split[1])
		if len(parts) != 3 {
			return Address{}, errors.New("parseAddress: invalid email")
		}
		return Address{User: parts[1], Host: parts[2]}, nil
	}
	if len(args) == 2 {
		if args[1] != "FROM:" && from {
			return Address{}, errors.New("parseAddress: invalid arguments")
		}
		if args[1] != "TO:" && !from {
			return Address{}, errors.New("parseAddress: invalid arguments")
		}

		parts := re.FindStringSubmatch(args[1])
		if len(parts) != 3 {
			panic("Fatal email error")
		}
		return Address{User: parts[1], Host: parts[2]}, nil
	}
	return Address{}, errors.New("parseAddress: too many arguments")
}

func validDomain(domain string, config Settings) bool {
	valid := false
	for _, d := range config.ValidDomains {
		if d == domain {
			return true
		}
	}
	return valid
}

func handleMAIL(c net.Conn, status *Client, config Settings, command Command, args []string) {
	email, err := parseAddress(args, true)
	if err != nil {
		write(c, "501 Invalid parameters")
		return
	}
	status.From = email
	status.Mode = ModeMail
	write(c, "250 <%s@%s> sender OK", email.User, email.Host)
}

func handleModeIdentified(c net.Conn, status *Client, config Settings, line string) {
	command, args := getCommand(line)

	switch command {
	case CommandNOOP:
		write(c, "250 OK")
	case CommandEHLO:
		handleHELO(c, status, config, command, args)
	case CommandHELO:
		handleHELO(c, status, config, command, args)
	case CommandHELP:
		handleHELP(c, status, config, command, args)
	case CommandQUIT:
		handleQUIT(c, config)
	case CommandRSET:
		handleRSET(c, status)
	case CommandMAIL:
		handleMAIL(c, status, config, command, args)
	case CommandUnknown:
		write(c, "500 Unrecognized command.")
	default:
		write(c, "503 Need MAIL before %s", command)
	}
}

func handleRCPT(c net.Conn, status *Client, config Settings, command Command, args []string) {
	email, err := parseAddress(args, false)
	if err != nil {
		write(c, "501 Invalid parameters")
		return
	}
	if validDomain(email.Host, config) {
		status.To = email
		status.Mode = ModeRcpt
		write(c, "250 <%s@%s> recipient OK", email.User, email.Host)
	} else {
		write(c, "550 No such user: %s@%s", email.User, email.Host)
	}
}

func handleModeMail(c net.Conn, status *Client, config Settings, line string) {
	command, args := getCommand(line)

	switch command {
	case CommandNOOP:
		write(c, "250 OK")
	case CommandEHLO:
		handleHELO(c, status, config, command, args)
	case CommandHELO:
		handleHELO(c, status, config, command, args)
	case CommandHELP:
		handleHELP(c, status, config, command, args)
	case CommandQUIT:
		handleQUIT(c, config)
	case CommandRSET:
		handleRSET(c, status)
	case CommandMAIL:
		write(c, "503 Sender already specified")
	case CommandRCPT:
		handleRCPT(c, status, config, command, args)
	case CommandUnknown:
		write(c, "500 Unrecognized command.")
	default:
		write(c, "503 Need RCPT before %s", command)
	}
}

func word_wrap(text string, lineWidth int) string {
	words := strings.Fields(strings.TrimSpace(text))
	if len(words) == 0 {
		return text
	}
	wrapped := words[0]
	spaceLeft := lineWidth - len(wrapped)
	for _, word := range words[1:] {
		if len(word)+1 > spaceLeft {
			wrapped += "\r\n        " + word
			spaceLeft = lineWidth - len(word)
		} else {
			wrapped += " " + word
			spaceLeft -= 1 + len(word)
		}
	}

	return wrapped

}

func handleModeRcpt(c net.Conn, status *Client, config Settings, line string) {
	command, args := getCommand(line)

	switch command {
	case CommandNOOP:
		write(c, "250 OK")
	case CommandEHLO:
		handleHELO(c, status, config, command, args)
	case CommandHELO:
		handleHELO(c, status, config, command, args)
	case CommandHELP:
		handleHELP(c, status, config, command, args)
	case CommandQUIT:
		handleQUIT(c, config)
	case CommandRSET:
		handleRSET(c, status)
	case CommandMAIL:
		write(c, "503 Sender already specified")
	case CommandRCPT:
		write(c, "503 Recipient already specified")
	case CommandDATA:
		status.Mode = ModeData
		status.Mail.WriteString(word_wrap(
			fmt.Sprintf("Received: from %s (%s) by %s with telesmtp; %s",
				status.Host, status.Addr, config.Hostname, time.Now().Format("Mon, 2 Jan 2006 15:04:05 -0700")), 78))
		status.Mail.WriteString("\r\n")
		write(c, "354 Go ahead, end your message with a single \".\"")
	case CommandUnknown:
		write(c, "500 Unrecognized command.")
	}
}

func handleModeData(c net.Conn, ch chan string, mailchan chan MessageStruct, status *Client, config Settings, line string) {
	if line == "." {
		message, err := mail.ReadMessage(bytes.NewReader(status.Mail.Bytes()))
		if err != nil {
			status.Mode = ModeIdentified
			write(c, "541 Could not parse your message, rejected to reduce spam")
			return
		}

		header := message.Header

		if header.Get("From") == "" || header.Get("Subject") == "" {
			status.Mode = ModeIdentified
			write(c, "541 Please supply From and Subject headers, rejected to reduce spam")
			fmt.Println(status.Mail.String())
			return
		}

		status.Mode = ModeIdentified
		mailchan <- MessageStruct{Mail: *message, RawMail: status.Mail, To: status.To, From: status.From}
		status.Mail.Reset()
		write(c, "250 Message accepted for delivery")
	} else {
		status.Mail.WriteString(line)
		status.Mail.WriteString("\r\n")
	}
}

func handler(c net.Conn, ch chan string, mail chan MessageStruct, config Settings) {
	defer c.Close()

	addr := c.RemoteAddr()
	ip, _, _ := net.SplitHostPort(addr.String())
	hosts, err := net.LookupAddr(ip)
	var host string
	if err != nil {
		ch <- fmt.Sprintf("Error looking up host for %s", ip)
		host = "unkown"
	} else {
		host = hosts[0]
	}
	status := Client{Mode: ModeInitial, Addr: addr, Host: host, Ip: ip}

	defer func() { ch <- fmt.Sprintf("Closing connection from %s", status.Addr.String()) }()

	ch <- fmt.Sprintf("Accepting connection from %s", status.Addr.String())

	scanner := bufio.NewScanner(c)

	write(c, "220 %s Running telesmtp", config.Hostname)

	for scanner.Scan() {
		ch <- fmt.Sprintf("%s: %s", c.RemoteAddr(), scanner.Text())

		switch status.Mode {
		case ModeInitial:
			handleModeInitial(c, &status, config, scanner.Text())
		case ModeIdentified:
			handleModeIdentified(c, &status, config, scanner.Text())
		case ModeMail:
			handleModeMail(c, &status, config, scanner.Text())
		case ModeRcpt:
			handleModeRcpt(c, &status, config, scanner.Text())
		case ModeData:
			handleModeData(c, ch, mail, &status, config, scanner.Text())
		}
	}
}

func server(l net.Listener, ch chan string, mail chan MessageStruct, config Settings) {
	for {
		c, err := l.Accept()
		if err != nil {
			continue
		}
		go handler(c, ch, mail, config)
	}
}

func logger(ch chan string) {
	grey := color.New(color.Faint).SprintFunc()
	for {
		fmt.Printf("%s: %s\n", grey(time.Now().Format("2006-01-02T15:04:05")), <-ch)
	}
}

func main() {
	data, err := ioutil.ReadFile("config.yaml")
	check(err)

	config := Settings{}

	err = yaml.Unmarshal(data, &config)
	check(err)

	if len(os.Args) == 2 {
		config.Hostname = os.Args[1]
	} else if config.Hostname == "" {
		hostname, err := os.Hostname()

		if err != nil {
			fmt.Println("Could not get hostname and not supplied on command line")
			os.Exit(1)
		}
		config.Hostname = hostname
	}

	l, err := net.Listen("tcp", fmt.Sprintf(":%d", config.Port))
	if err != nil {
		panic(err)
	}

	fmt.Printf("telesmtp listening on %s\n", l.Addr())

	ch := make(chan string)
	mail := make(chan MessageStruct)

	go logger(ch)
	go server(l, ch, mail, config)

	go Listen(mail, config)

	for {
		time.Sleep(10 * time.Second)
	}
}
