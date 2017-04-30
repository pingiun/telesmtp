package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"github.com/fatih/color"
	"net"
	"os"
	"regexp"
	"strings"
	"time"
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

type Email struct {
	user string
	host string
}

type Client struct {
	mode     Mode
	addr     net.Addr
	host     string
	identity string
	from     Email
	to       Email
	mail     bytes.Buffer
}

type Settings struct {
	hostname      string
	valid_domains []string
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
		status.identity = identity
		status.mode = ModeIdentified
		if command == CommandEHLO {
			write(c, "250-%s here, welcome %s [%s], pleased to meet you", config.hostname, status.host, status.addr)
			write(c, "250 HELP")
		} else {
			write(c, "250 %s at your service", config.hostname)
		}
	}
}

func handleQUIT(c net.Conn, config Settings) {
	write(c, "221 %s closing connection", config.hostname)
	c.Close()
}

func handleRSET(c net.Conn, status *Client) {
	status.mode = ModeInitial
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

func parseAddress(args []string, from bool) (address Email, err error) {
	if len(args) == 0 {
		return Email{}, errors.New("parseAddress: no arguments")
	}
	re := regexp.MustCompile("<\\s*(.+)@(.+)\\s*>")
	if len(args) == 1 {
		split := strings.SplitN(args[0], ":", 2)
		if split[0] != "FROM" && from {
			return Email{}, errors.New("parseAddress: invalid arguments")
		}
		if split[0] != "TO" && !from {
			return Email{}, errors.New("parseAddress: invalid arguments")
		}

		parts := re.FindStringSubmatch(split[1])
		if len(parts) != 3 {
			panic("Fatal email error")
		}
		return Email{user: parts[1], host: parts[2]}, nil
	}
	if len(args) == 2 {
		if args[1] != "FROM:" && from {
			return Email{}, errors.New("parseAddress: invalid arguments")
		}
		if args[1] != "TO:" && !from {
			return Email{}, errors.New("parseAddress: invalid arguments")
		}

		parts := re.FindStringSubmatch(args[1])
		if len(parts) != 3 {
			panic("Fatal email error")
		}
		return Email{user: parts[1], host: parts[2]}, nil
	}
	return Email{}, errors.New("parseAddress: too many arguments")
}

func validDomain(domain string, config Settings) bool {
	valid := false
	for _, d := range config.valid_domains {
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
	status.from = email
	status.mode = ModeMail
	write(c, "250 <%s@%s> sender OK", email.user, email.host)
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
	if validDomain(email.host, config) {
		status.to = email
		status.mode = ModeRcpt
		write(c, "250 <%s@%s> recipient OK", email.user, email.host)
	} else {
		write(c, "550 No such user: %s@%s", email.user, email.host)
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
		write(c, "354 Go ahead, end your message with a single \".\"")
		status.mode = ModeData
	case CommandUnknown:
		write(c, "500 Unrecognized command.")
	}
}

func handleModeData(c net.Conn, ch chan string, status *Client, config Settings, line string) {
	if line == "." {
		ch <- "Sending mail:"
		ch <- status.mail.String()
		status.mode = ModeIdentified
		write(c, "250 Message accepted for delivery")
	} else {
		status.mail.WriteString(line)
		status.mail.WriteString("\r\n")
	}
}

func handler(c net.Conn, ch chan string, config Settings) {
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
	status := Client{mode: ModeInitial, addr: addr, host: host}

	ch <- fmt.Sprintf("Accepting connection from %s", status.addr.String())

	scanner := bufio.NewScanner(c)

	write(c, "220 %s Running telesmtp", config.hostname)

	for scanner.Scan() {
		ch <- fmt.Sprintf("%s: %s", c.RemoteAddr(), scanner.Text())

		switch status.mode {
		case ModeInitial:
			handleModeInitial(c, &status, config, scanner.Text())
		case ModeIdentified:
			handleModeIdentified(c, &status, config, scanner.Text())
		case ModeMail:
			handleModeMail(c, &status, config, scanner.Text())
		case ModeRcpt:
			handleModeRcpt(c, &status, config, scanner.Text())
		case ModeData:
			handleModeData(c, ch, &status, config, scanner.Text())
		}
	}
}

func server(l net.Listener, ch chan string, config Settings) {
	for {
		c, err := l.Accept()
		if err != nil {
			continue
		}
		go handler(c, ch, config)
	}
}

func logger(ch chan string) {
	grey := color.New(color.Faint).SprintFunc()
	for {
		fmt.Printf("%s: %s\n", grey(time.Now().Format("2006-01-02T15:04:05")), <-ch)
	}
}

func main() {
	var hostname string

	if len(os.Args) == 2 {
		hostname = os.Args[1]
	} else {
		var err error
		hostname, err = os.Hostname()

		if err != nil {
			fmt.Println("Could not get hostname and not supplied on command line")
			os.Exit(1)
		}
	}

	l, err := net.Listen("tcp", ":25")
	if err != nil {
		panic(err)
	}

	fmt.Printf("telesmtp listening on %s\n", l.Addr())

	ch := make(chan string)
	go logger(ch)
	go server(l, ch, Settings{hostname, []string{"ictrek.nl", "localhost", "127.0.0.1"}})

	for {
		time.Sleep(10 * time.Second)
	}
}
