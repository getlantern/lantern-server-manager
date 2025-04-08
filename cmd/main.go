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
	LogLevel LogLevel  `arg:"-l,--log-level" help:"set log level" default:"info"`
	Serve    *ServeCmd `arg:"subcommand:serve" help:"start the server"`
	Init     *InitCmd  `arg:"subcommand:init" help:"generate initial configuration"`
}

func main() {
	var err error
	p := arg.MustParse(&args)
	log.SetLevel(args.LogLevel.Level)
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
