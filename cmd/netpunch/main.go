package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"text/template"

	"github.com/michurin/netpunch/netpunchlib"
)

const defaultTemplate = "LADDR/LHOST/LPORT/RADDR/RHOST/RPORT: {{.LocalAddr}} {{.LocalIP}} {{.LocalPort}} {{.RemoteAddr}} {{.RemoteIP}} {{.RemotePort}}\n"

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
	command     string
	commandArgs []cliArgument
)

type cliArgument struct {
	template *template.Template // nil if it is raw string
	raw      string
	split    bool // have to be split after substitutions
}

func setupVersion() {
	if bi, ok := debug.ReadBuildInfo(); ok {
		version += "-" + bi.Main.Path + "@" + bi.Main.Sum + "/" + bi.Main.Version
	}
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
	flag.StringVar(&templateFile, "template-file", "", "template file; see -template")
	flag.StringVar(&templateText, "template", "", "template text; see -template-file")
	flag.StringVar(&command, "command", "", "command to execute right after the hole gets ready;\nsee -arg, -fields and -raw")
	flag.Func("arg", "specify argument to command; considered as template;\nsee -command, -template", func(v string) error {
		t, err := template.New("main").Parse(v)
		if err != nil {
			return err
		}
		commandArgs = append(commandArgs, cliArgument{template: t}) //nolint:exhaustruct
		return nil
	})
	flag.Func("fields", "specify space-separated command's arguments; considered as template;\nsee -command, -arg", func(v string) error {
		t, err := template.New("main").Parse(v)
		if err != nil {
			return err
		}
		commandArgs = append(commandArgs, cliArgument{template: t, split: true}) //nolint:exhaustruct
		return nil
	})
	flag.Func("raw", "like -arg, but without templates", func(v string) error {
		commandArgs = append(commandArgs, cliArgument{raw: v}) //nolint:exhaustruct
		return nil
	})
	defaultUsage := flag.Usage
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Version: %s\n", version)
		defaultUsage()
		fmt.Fprintf(flag.CommandLine.Output(), `Examples:
Control mode (run at 2.3.3.3):
        %[1]s -secret TheSecretWord -local :7777
First peer: peer mode (run in private network, peer a):
        %[1]s -peer a -secret TheSecretWord -remote 2.3.3.3:7777 -local :1194
Second peer: peer mode (run in private network, peer b):
        %[1]s -peer b -secret TheSecretWord -remote 2.3.3.3:7777 -local :1194
`, path.Base(os.Args[0]))
		fmt.Fprintf(flag.CommandLine.Output(), "Default template is:\n        %s\n", strings.TrimSpace(defaultTemplate))
		fmt.Fprintln(flag.CommandLine.Output(), "Project home: https://github.com/michurin/netpunch")
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
	s, err := os.ReadFile(fn)
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

type templateDTO struct {
	LocalAddr  string
	LocalIP    string
	LocalPort  string
	RemoteAddr string
	RemoteIP   string
	RemotePort string
}

func buildTemplateDTO(laddr, addr *net.UDPAddr) templateDTO {
	return templateDTO{
		LocalAddr:  laddr.String(),
		LocalIP:    safeIP(laddr.IP),
		LocalPort:  strconv.Itoa(laddr.Port),
		RemoteAddr: addr.String(),
		RemoteIP:   safeIP(addr.IP),
		RemotePort: strconv.Itoa(addr.Port),
	}
}

func executeCommand(logger *log.Logger, dto templateDTO) error {
	if command == "" {
		return nil
	}
	binary, err := exec.LookPath(command)
	if err != nil {
		return err
	}
	args := []string{binary} // we do not know final length of this slice
	for _, v := range commandArgs {
		if v.template == nil {
			args = append(args, v.raw)
			continue
		}
		b := new(strings.Builder)
		err = v.template.Execute(b, dto)
		if err != nil {
			return err
		}
		if v.split {
			args = append(args, strings.Fields(b.String())...)
		} else {
			args = append(args, b.String())
		}
	}
	logger.Print("Exec args:")
	for i, v := range args {
		logger.Printf("%d: %q", i, v)
	}
	err = syscall.Exec(binary, args, os.Environ())
	if err != nil {
		return err
	}
	return nil
}

func printResult(dto templateDTO) error {
	return templateObj.Execute(os.Stdout, dto)
}

func logWriter() io.Writer {
	if silentMode {
		return io.Discard
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
	setupVersion()
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
		dto := buildTemplateDTO(laddr, addr)
		helpAndExitIfError(printResult(dto))
		helpAndExitIfError(executeCommand(logger, dto))
	}
}
