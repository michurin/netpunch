package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"syscall"
	"text/template"

	"github.com/michurin/netpunch/netpunchlib"
)

var (
	// go build -ldflags "-X main.gitCommit=$(git rev-list --abbrev-commit -1 HEAD)" ./cmd/...
	gitCommit = ""
	version   = "0.2" // tweaked in init

	// CLI flags.
	role        string
	secret      string
	remoteAddr  string
	localAddr   string
	showVersion bool
	silentMode  bool
	rawMode     bool
	templateObj *template.Template // won't be nil after setupFlags()
)

const defaultTemplate = "LADDR/LHOST/LPORT/RADDR/RHOST/RPORT: {{.LocalAddr}} {{.LocalIP}} {{.LocalPort}} {{.RemoteAddr}} {{.RemoteIP}} {{.RemotePort}}\n"

func init() {
	if gitCommit != "" {
		version += "-" + gitCommit
	}
}

func setupFlags() error {
	var err error
	var secretFile, templateFile, templateText string

	flag.CommandLine.SetOutput(os.Stderr)
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.BoolVar(&silentMode, "silent", false, "silent mode")
	flag.BoolVar(&rawMode, "raw-logging", false, "log raw messages, including cryptography signatures")
	flag.StringVar(&role, "peer", "", `role of peer: a-z
it is linking a and b, c and d and so on up to y and z
if peer not specified, we run in control mode`)
	flag.StringVar(&secret, "secret", "", "shared secret to sign messages")
	flag.StringVar(&secretFile, "secret-file", "", "get shared secret from file")
	flag.StringVar(&remoteAddr, "remote", "", "public address of control node; for peer-mode only")
	flag.StringVar(&localAddr, "local", "", `local address
in control mode it is listening address
in peer mode it is outgoing address`)
	flag.StringVar(&templateFile, "template-file", "", "template file")
	flag.StringVar(&templateText, "template", "", "template text")
	defaultUsage := flag.Usage
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Version: %s\n", version)
		defaultUsage()
		fmt.Fprintf(flag.CommandLine.Output(), `Examples:
Control mode (run at 2.3.3.3):
        %[1]s -secret TheSecretWord -local :7777
Peer mode (run in private network, peer a):
        %[1]s -peer a -secret TheSecretWord -remote 2.3.3.3:7777 -local :1194
`, path.Base(os.Args[0]))
		fmt.Fprintf(flag.CommandLine.Output(), "Default template is:\n        %s\n", defaultTemplate)
	}

	flag.Parse()

	secret, err = readFile(secretFile, secret)
	if err != nil {
		return err
	}
	if templateText == "" {
		templateText, err = readFile(templateFile, defaultTemplate)
		if err != nil {
			return err
		}
	}
	templateObj, err = template.New("main").Parse(templateText)
	if err != nil {
		return err
	}
	return nil
}

func readFile(fn, def string) (string, error) {
	if fn == "" {
		return def, nil
	}
	s, err := ioutil.ReadFile(fn)
	if err != nil {
		return "", err
	}
	return string(s), nil
}

func checkFlags() error {
	messages := []string(nil)
	if role == "" && remoteAddr != "" {
		messages = append(messages, "you do not have to specify remote address in control mode")
	}
	if role != "" && remoteAddr == "" {
		messages = append(messages, fmt.Sprintf("you have to specify remote address in peer mode role %q", role))
	}
	if secret == "" {
		messages = append(messages, "you have to specify secret")
	}
	if localAddr == "" {
		messages = append(messages, "you have to specify local address")
	}
	if messages != nil {
		return errors.New(strings.Join(messages, "; "))
	}
	return nil
}

func helpAndExitIfError(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "Error:", err.Error())
	os.Exit(1)
}

func safeIP(ip net.IP) string {
	switch len(ip) {
	case net.IPv4len, net.IPv6len:
		return ip.String()
	default:
		return "n/a"
	}
}

func printResult(laddr, addr *net.UDPAddr) error {
	return templateObj.Execute(os.Stdout, struct {
		LocalAddr  string
		LocalIP    string
		LocalPort  string
		RemoteAddr string
		RemoteIP   string
		RemotePort string
	}{
		LocalAddr:  laddr.String(),
		LocalIP:    safeIP(laddr.IP),
		LocalPort:  strconv.Itoa(laddr.Port),
		RemoteAddr: addr.String(),
		RemoteIP:   safeIP(addr.IP),
		RemotePort: strconv.Itoa(addr.Port),
	})
}

func logWriter() io.Writer {
	if silentMode {
		return ioutil.Discard
	}
	return os.Stderr
}

func connectionOptions(loggingMiddleware, signingMiddleware netpunchlib.ConnectionMiddleware) netpunchlib.Option {
	if rawMode {
		return netpunchlib.ConnOption(loggingMiddleware, signingMiddleware) // put logging first
	} else { //nolint:revive
		return netpunchlib.ConnOption(signingMiddleware, loggingMiddleware) // put logging last
	}
}

func main() {
	helpAndExitIfError(setupFlags())
	if showVersion {
		fmt.Println(version)
		return
	}

	helpAndExitIfError(checkFlags())

	logger := log.New(logWriter(), "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lmsgprefix)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exit := make(chan os.Signal, 1) // we need to reserve to buffer size 1, so the notifier are not blocked
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-exit
		sigID, _ := sig.(syscall.Signal)
		logger.Printf("[info] Shutting down due to signal: %s (0x%02X)", sig.String(), int(sigID))
		cancel()
	}()

	connOption := connectionOptions(
		netpunchlib.LoggingMiddleware(logger),
		netpunchlib.SigningMiddleware([]byte(secret)))

	if role == "" {
		logger.SetPrefix(fmt.Sprintf("[%d] ", os.Getpid()))
		logger.Print("[info] Start in control mode on " + localAddr)
		err := netpunchlib.Server(ctx, localAddr, connOption)
		helpAndExitIfError(err)
	} else {
		logger.SetPrefix(fmt.Sprintf("[%d] [%s] ", os.Getpid(), role))
		logger.Print("[info] Start in peer mode on " + localAddr + " to server at " + remoteAddr)
		laddr, addr, err := netpunchlib.Client(ctx, role, localAddr, remoteAddr, connOption) // btw, abstraction leaking (role: arg->payload)
		helpAndExitIfError(err)
		helpAndExitIfError(printResult(laddr, addr))
	}
}
