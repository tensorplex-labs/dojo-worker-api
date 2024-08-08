package main

import (
	"dojo-api/pkg/blockchain"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog/log"
)

func main() {
	substrateService := blockchain.NewSubstrateService()
	substrateService.GetLatestFinalizedBlock()
	// a bit lower than last finalized block on testnet
	const initialBlockId = 1_923_000
	substrateService.GetLatestUnFinalizedBlock(initialBlockId)

	// TODO write unit tests??
	// service := blockchain.NewSubstrateService()
	// fmt.Println(service.GetMaxUID(1))
	// fmt.Println(service.GetMaxUID(2))
	// fmt.Println(service.GetAllAxons(21))
	// fmt.Println(service.CheckIsRegistered(21, "***REMOVED***"))
	// service.SubscribeAxonInfos(21)
	// fmt.Println(service.TotalHotkeyStake("5F4tQyWrhfGVcNhoqeiNsR6KjD4wMZ2kfhLj4oHYuyHbZAc3"))
	subnetSubscriber := blockchain.GetSubnetStateSubscriberInstance()

	fmt.Println(subnetSubscriber.FindValidatorHotkeyIndex("5F4tQyWrhfGVcNhoqeiNsR6KjD4wMZ2kfhLj4oHYuyHbZAc3"))
	// fmt.Println(subnetSubscriber.FindMinerHotkeyIndex("***REMOVED***"))
	// fmt.Println(subnetSubscriber.FindMinerHotkeyIndex("***REMOVED***"))
	// fmt.Println(subnetSubscriber.FindMinerHotkeyIndex("***REMOVED***"))
	// fmt.Println(subnetSubscriber.FindMinerHotkeyIndex("***REMOVED***"))
	// fmt.Println(subnetSubscriber.FindMinerHotkeyIndex("***REMOVED***"))

	// Deleting
	subnetSubscriber.OnNonRegisteredFound("5F4tQyWrhfGVcNhoqeiNsR6KjD4wMZ2kfhLj4oHYuyHbZAc3")
	fmt.Println(subnetSubscriber.FindValidatorHotkeyIndex("5F4tQyWrhfGVcNhoqeiNsR6KjD4wMZ2kfhLj4oHYuyHbZAc3"))

	// wait for interrupt signal to gracefully shutdown the program
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info().Msgf("Received signal: %s. Shutting down...", sig)
}
