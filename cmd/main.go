package main

import (
	"errors"
	"fmt"
	"github.com/alexflint/go-arg"
	"github.com/charmbracelet/log"
	"os"
)

type LogLevel struct {
	log.Level
}

func (ll *LogLevel) UnmarshalText(b []byte) error {
	l, err := log.ParseLevel(string(b))
	if err != nil {
		return err
	}
	ll.Level = l
	return nil
}

var args struct {
	LogLevel LogLevel `arg:"-l,--log-level" help:"set log level" default:"info"`
	DataDir  string   `arg:"-d" help:"data directory" default:"./data"`

	Serve *ServeCmd `arg:"subcommand:serve" help:"start the server"`
	Init  *InitCmd  `arg:"subcommand:init" help:"generate initial configuration"`
}

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
