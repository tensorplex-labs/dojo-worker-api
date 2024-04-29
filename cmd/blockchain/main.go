package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"dojo-api/pkg/blockchain"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	debug := flag.Bool("debug", false, "sets log level to debug")
	trace := flag.Bool("trace", false, "sets log level to trace")
	flag.Parse()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else if *trace {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	}

	// TODO write unit tests??
	// service := blockchain.NewSubstrateService()
	// fmt.Println(service.GetMaxUID(1))
	// fmt.Println(service.GetMaxUID(2))
	// fmt.Println(service.GetAllAxons(21))
	// fmt.Println(service.CheckIsRegistered(21, "***REMOVED***"))
	// service.SubscribeAxonInfos(21)
	// fmt.Println(service.TotalHotkeyStake("5F4tQyWrhfGVcNhoqeiNsR6KjD4wMZ2kfhLj4oHYuyHbZAc3"))
	subnetSubscriber := blockchain.NewSubnetStateSubscriber()
	subnetSubscriber.SubscribeSubnetState(21)

	// wait for interrupt signal to gracefully shutdown the program
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info().Msgf("Received signal: %s. Shutting down...", sig)
}
