package blockchain

import (
	"time"

	"github.com/rs/zerolog/log"
)

type SubnetState struct {
	SubnetId         int
	ValidatorHotkeys []string
	MinerHotkeys     []string
	AxonInfos        []AxonInfo
}

type SubnetStateSubscriber struct {
	substrateService *SubstrateService
	SubnetStateMap   map[int]SubnetState // subnet id to subnet state
}

func NewSubnetStateSubscriber() *SubnetStateSubscriber {
	return &SubnetStateSubscriber{
		substrateService: NewSubstrateService(),
		SubnetStateMap:   make(map[int]SubnetState),
	}
}

func (s *SubnetStateSubscriber) SubscribeAxonInfos(subnetId int) error {
	ticker := time.NewTicker(112 * BlockTimeInSeconds * time.Second)
	// execute once then enter go routine
	axonInfos, err := s.substrateService.GetAllAxons(subnetId)
	subnetStates := make(map[int]SubnetState)
	subnetStates[subnetId] = SubnetState{SubnetId: subnetId, AxonInfos: axonInfos}
	s.SubnetStateMap = subnetStates

	if err != nil {
		log.Error().Err(err).Msg("Error getting all axons")
		return err
	}

	go func() {
		for range ticker.C {
			axonInfos, err := s.substrateService.GetAllAxons(subnetId)
			if err != nil {
				log.Error().Err(err).Msg("Error getting all axons")
				return
			}

			// update subnet info
			s.SubnetStateMap[subnetId] = SubnetState{SubnetId: subnetId, AxonInfos: axonInfos}
		}
	}()
	return nil
}
