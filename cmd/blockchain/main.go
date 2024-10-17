package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"dojo-api/pkg/blockchain"

	"github.com/rs/zerolog/log"
)

func main() {
	substrateService := blockchain.NewSubstrateService()
	testUid := 13
	testSubnetId := 52
	testHotkey, err := substrateService.GetHotkeyByUid(testSubnetId, testUid)
	if err != nil {
		log.Error().Err(err).Msg("Error getting hotkey")
	}
	log.Info().Msgf("Hotkey: %s", testHotkey)
	substrateService.CheckIsRegistered(testSubnetId, testHotkey)
	stake, err := substrateService.TotalHotkeyStake(testHotkey)
	if err != nil {
		log.Error().Err(err).Msg("Error getting hotkey stake")
	}
	log.Info().Msgf("Stake: %f", stake)

	blockchain.GetSubnetStateSubscriberInstance().GetSubnetState(testSubnetId)

	// Fetch the latest finalized block
	substrateService.GetLatestFinalizedBlock()

	// Fetch the latest unfinalized block starting from a specific block ID
	const initialBlockId = 1_923_000
	substrateService.GetLatestUnFinalizedBlock(initialBlockId)

	// TODO: write unit testing??
	// Initialize a SubnetStateSubscriber instance
	subnetSubscriber := blockchain.GetSubnetStateSubscriberInstance()

	// Example usage of SubnetStateSubscriber
	validatorHotkey := "***REMOVED***"
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
