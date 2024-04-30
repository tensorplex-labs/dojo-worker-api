package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"dojo-api/pkg/blockchain"

	"github.com/rs/zerolog/log"
)

func main() {

	// TODO write unit tests??
	// service := blockchain.NewSubstrateService()
	// fmt.Println(service.GetMaxUID(1))
	// fmt.Println(service.GetMaxUID(2))
	// fmt.Println(service.GetAllAxons(21))
	// fmt.Println(service.CheckIsRegistered(21, "***REMOVED***"))
	// service.SubscribeAxonInfos(21)
	// fmt.Println(service.TotalHotkeyStake("5F4tQyWrhfGVcNhoqeiNsR6KjD4wMZ2kfhLj4oHYuyHbZAc3"))
	subnetSubscriber := blockchain.GetSubnetStateSubscriberInstance()

	data, err := json.MarshalIndent(subnetSubscriber.GetSubnetState(21), "", "  ")
	if err != nil {
		log.Error().Err(err).Msg("")
	}

	fmt.Println(string(data))

	fmt.Println(subnetSubscriber.FindValidatorHotkeyIndex("5F4tQyWrhfGVcNhoqeiNsR6KjD4wMZ2kfhLj4oHYuyHbZAc3"))
	fmt.Println(subnetSubscriber.FindMinerHotkeyIndex("***REMOVED***"))
	fmt.Println(subnetSubscriber.FindMinerHotkeyIndex("***REMOVED***"))
	fmt.Println(subnetSubscriber.FindMinerHotkeyIndex("***REMOVED***"))
	fmt.Println(subnetSubscriber.FindMinerHotkeyIndex("***REMOVED***"))
	fmt.Println(subnetSubscriber.FindMinerHotkeyIndex("***REMOVED***"))

	// wait for interrupt signal to gracefully shutdown the program
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info().Msgf("Received signal: %s. Shutting down...", sig)
}
