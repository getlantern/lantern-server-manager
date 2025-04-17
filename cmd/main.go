package main

import (
	"errors"
	"fmt"
	"github.com/alexflint/go-arg"
	"github.com/charmbracelet/log"
	"os"
)

// LogLevel is a wrapper around charmbracelet/log.Level to allow
// parsing log levels directly from command-line arguments using go-arg.
type LogLevel struct {
	log.Level
}

// UnmarshalText implements the encoding.TextUnmarshaler interface for LogLevel.
// This allows go-arg to parse log level strings (e.g., "debug", "info", "error").
func (ll *LogLevel) UnmarshalText(b []byte) error {
	l, err := log.ParseLevel(string(b))
	if err != nil {
		return err
	}
	ll.Level = l
	return nil
}

// args holds the command-line arguments parsed by go-arg.
var args struct {
	LogLevel LogLevel `arg:"-l,--log-level" help:"set log level" default:"info"`
	DataDir  string   `arg:"-d" help:"data directory" default:"./data"`
	APIPort  int      `arg:"--api-port" help:"API port"`
	VPNPort  int      `arg:"--vpn-port" help:"VPN port"`

	Serve *ServeCmd `arg:"subcommand:serve" help:"start the server"`
	Init  *InitCmd  `arg:"subcommand:init" help:"generate initial configuration"`
}

// main is the entry point of the application.
// It parses command-line arguments, sets the log level, ensures the data directory exists,
// and dispatches execution to the appropriate subcommand (serve or init).
func main() {
	var err error
	p := arg.MustParse(&args)
	log.SetLevel(args.LogLevel.Level)
	ensureDataDirectoryExists()
	switch {
	case args.Serve != nil:
		err = args.Serve.Run()
	case args.Init != nil:
		err = args.Init.Run()
	default:
		p.WriteHelp(os.Stderr)
	}
	if err != nil {
		if errors.Is(err, arg.ErrHelp) {
			_ = p.WriteHelpForSubcommand(os.Stderr, p.SubcommandNames()...)
		} else {
			_, _ = fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}

// ensureDataDirectoryExists checks if the data directory specified by args.DataDir exists.
// If it doesn't exist, it creates the directory with 0755 permissions.
// If the path exists but is not a directory, it logs a fatal error.
func ensureDataDirectoryExists() {
	if fi, err := os.Stat(args.DataDir); err != nil {
		if os.IsNotExist(err) {
			log.Debug("data directory does not exist, creating", "path", args.DataDir)
			err = os.MkdirAll(args.DataDir, 0755)
			if err != nil {
				log.Fatalf("Unable to create data folder %v", err)
			}
		} else if !fi.IsDir() {
			log.Fatal("data directory is not a directory")
		}
	} else {
		if !fi.IsDir() {
			log.Fatal("data directory is not a directory")
		}
	}
}
