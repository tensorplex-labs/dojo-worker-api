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

	// Fetch the latest finalized block
	substrateService.GetLatestFinalizedBlock()

	// Fetch the latest unfinalized block starting from a specific block ID
	const initialBlockId = 1_923_000
	substrateService.GetLatestUnFinalizedBlock(initialBlockId)

	// TODO: write unit testing??
	// Initialize a SubnetStateSubscriber instance
	subnetSubscriber := blockchain.GetSubnetStateSubscriberInstance()

	// Example usage of SubnetStateSubscriber
	validatorHotkey := "5F4tQyWrhfGVcNhoqeiNsR6KjD4wMZ2kfhLj4oHYuyHbZAc3"
	fmt.Println(subnetSubscriber.FindValidatorHotkeyIndex(validatorHotkey))

	// Handling non-registered found case
	subnetSubscriber.OnNonRegisteredFound(validatorHotkey)
	fmt.Println(subnetSubscriber.FindValidatorHotkeyIndex(validatorHotkey))

	// Wait for interrupt signal to gracefully shutdown the program
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Info().Msgf("Received signal: %s. Shutting down...", sig)
}
