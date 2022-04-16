package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"

	"github.com/michurin/netpunch/netpunchlib"
)

var (
	// go build -ldflags "-X main.gitCommit=$(git rev-list --abbrev-commit -1 HEAD)" ./cmd/...
	gitCommit = ""
	version   = "0.1" // tweaked in init

	// CLI flags.
	role        string
	secret      string
	remoteAddr  string
	localAddr   string
	showVersion bool
)

func init() {
	if gitCommit != "" {
		version += "-" + gitCommit
	}
}

func setupFlags() error {
	var secretFile string
	flag.CommandLine.SetOutput(os.Stderr)
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.StringVar(&role, "peer", "", `role of peer: a or b
if peer not specified, we run in control mode`)
	flag.StringVar(&secret, "secret", "", "shared secret to sign messages")
	flag.StringVar(&secretFile, "secret-file", "", "get shared secret from file")
	flag.StringVar(&remoteAddr, "remote", "", "public address of control node; for peer-mode only")
	flag.StringVar(&localAddr, "local", "", `local address
in control mode it is listening address
in peer mode it is outgoing address`)
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
	}

	flag.Parse()

	if secretFile != "" {
		s, err := ioutil.ReadFile(secretFile)
		if err != nil {
			return err
		}
		secret = string(s)
	}
	return nil
}

func checkFlags() error {
	messages := []string(nil)
	switch role {
	case "a", "b":
		if remoteAddr == "" {
			messages = append(messages, fmt.Sprintf("you have to specify remote address in peer mode role %q", role))
		}
	case "":
		if remoteAddr != "" {
			messages = append(messages, "you do not have to specify remote address in control mode")
		}
	default:
		messages = append(messages, fmt.Sprintf("invalid role: %q", role))
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

func printResult(laddr, addr *net.UDPAddr) {
	fmt.Fprintln(
		os.Stdout,
		"LADDR/LHOST/LPORT/RADDR/RHOST/RPORT:",
		laddr,
		safeIP(laddr.IP),
		laddr.Port,
		addr,
		safeIP(addr.IP),
		addr.Port)
}

func main() {
	helpAndExitIfError(setupFlags())
	if showVersion {
		fmt.Println(version)
		return
	}

	helpAndExitIfError(checkFlags())

	logger := log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lmsgprefix)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exit := make(chan os.Signal, 1) // we need to reserve to buffer size 1, so the notifier are not blocked
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-exit
		sigID, _ := sig.(syscall.Signal)
		logger.Print(fmt.Sprintf("[info] Shutting down due to signal: %s (0x%02X)", sig.String(), int(sigID)))
		cancel()
	}()

	opts := netpunchlib.ConnOption(
		// netpunchlib.LoggingMiddleware(logger), // uncomment this if you like to see full debugging including signatores
		netpunchlib.SigningMiddleware([]byte(secret)),
		netpunchlib.LoggingMiddleware(logger))

	if role == "" {
		logger.SetPrefix(fmt.Sprintf("[%d] ", os.Getpid()))
		logger.Print("[info] Start in control mode on " + localAddr)
		err := netpunchlib.Server(ctx, localAddr, opts)
		helpAndExitIfError(err)
	} else {
		logger.SetPrefix(fmt.Sprintf("[%d] [%s] ", os.Getpid(), role))
		logger.Print("[info] Start in peer mode on " + localAddr + " to server at " + remoteAddr)
		laddr, addr, err := netpunchlib.Client(ctx, role, localAddr, remoteAddr, opts) // btw, abstraction leaking (role: arg->payload)
		helpAndExitIfError(err)
		printResult(laddr, addr)
	}
}
