package main

import (
	    "os"

	    "github.com/rs/zerolog"
	    "github.com/rs/zerolog/log"
	    "golang.org/x/crypto/ssh/terminal"
)

func initLog() {
	    logfmt := os.Getenv("LOGFMT")
	    if logfmt != "json" {
		    logfmt = "text"
		    log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, NoColor: !terminal.IsTerminal(int(os.Stdout.Fd()))})
	    }

	    level, _ := zerolog.ParseLevel(os.Getenv("LOGLVL"))
	    if level == zerolog.NoLevel {
		    level = zerolog.InfoLevel
	    }
	    // log.Info().Str("FMT", logfmt).Str("LVL", level.String()).Msg("log initialized")
	    zerolog.SetGlobalLevel(level)
}
