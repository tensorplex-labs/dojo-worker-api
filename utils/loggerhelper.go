package utils

import (
	"flag"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
)

var (
    debug *bool
    trace *bool
)

func init() {
    debug = flag.Bool("debug", false, "sets log level to debug")
    trace = flag.Bool("trace", false, "sets log level to trace")
    flag.Parse()
}

func GetLogger() *zerolog.Logger {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else if *trace {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	}
	return &log.Logger
}