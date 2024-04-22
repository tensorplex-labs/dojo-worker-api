package blockchain

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

var (
	ValidatorMinStake = GetValidatorMinStake()
)

func GetValidatorMinStake() int {
	err := godotenv.Load()
	if err != nil {
		log.Fatal().Msg("Error loading .env file")
	}

	validatorMinStake := os.Getenv("VALIDATOR_MIN_STAKE")
	if validatorMinStake == "" {
		log.Fatal().Msg("VALIDATOR_MIN_STAKE must be set")
	}

	intValue, err := strconv.Atoi(validatorMinStake)
	if err != nil {
		log.Fatal().Err(err).Msg("Error converting VALIDATOR_MIN_STAKE to int")
	}

	return intValue
}

// TODO this is only applicable to whatever subnet has the same definition of validator min stake
type SubnetState struct {
	SubnetId               int
	ActiveValidatorHotkeys []string
	ActiveMinerHotkeys     []string
	ActiveAxonInfos        []AxonInfo
}

type GlobalState struct {
	HotkeyStakes map[string]float64
}

type SubnetStateSubscriber struct {
	substrateService *SubstrateService
	SubnetState      *SubnetState // meant for only tracking our subnet state
	GlobalState      *GlobalState
}

func NewSubnetStateSubscriber() *SubnetStateSubscriber {
	return &SubnetStateSubscriber{
		substrateService: NewSubstrateService(),
		SubnetState:      &SubnetState{},
		GlobalState:      &GlobalState{HotkeyStakes: make(map[string]float64)},
	}
}

func (s *SubnetStateSubscriber) OnNonRegisteredFound(hotkey string) {
	if hotkey == "" {
		log.Fatal().Msg("Hotkey is empty, cannot remove from active validators/miners/axons")
		return
	}

	// clear from active validators if found
	for i, vhotkey := range s.SubnetState.ActiveValidatorHotkeys {
		if hotkey == vhotkey {
			s.SubnetState.ActiveValidatorHotkeys = append(s.SubnetState.ActiveValidatorHotkeys[:i], s.SubnetState.ActiveValidatorHotkeys[i+1:]...)
			break
		}
	}

	// clear from active miners if found
	for i, mhotkey := range s.SubnetState.ActiveMinerHotkeys {
		if hotkey == mhotkey {
			s.SubnetState.ActiveMinerHotkeys = append(s.SubnetState.ActiveMinerHotkeys[:i], s.SubnetState.ActiveMinerHotkeys[i+1:]...)
			break
		}
	}

	// clear from axon infos
	for i, axonInfo := range s.SubnetState.ActiveAxonInfos {
		if hotkey == axonInfo.Hotkey {
			s.SubnetState.ActiveAxonInfos = append(s.SubnetState.ActiveAxonInfos[:i], s.SubnetState.ActiveAxonInfos[i+1:]...)
			break
		}
	}
}

func (s *SubnetStateSubscriber) GetSubnetState(subnetId int) *SubnetState {
	axonInfos, err := s.substrateService.GetAllAxons(subnetId)
	if err != nil {
		log.Error().Err(err).Msg("Error getting all axons")
		return &SubnetState{}
	}

	subnetState := SubnetState{SubnetId: subnetId, ActiveAxonInfos: axonInfos}
	minerHotkeys := make([]string, 0)
	validatorHotkeys := make([]string, 0)

	hotkeyToStake := make(map[string]float64)
	hotkeyToIsRegistered := make(map[string]bool)

	var wg sync.WaitGroup
	var mutex sync.Mutex

	for _, axonInfo := range axonInfos {
		wg.Add(1)
		go func(currAxonInfo AxonInfo) {
			defer wg.Done()
			if currAxonInfo.Hotkey == "" {
				log.Trace().Msgf("AxonInfo empty hotkey, %+v", currAxonInfo)
				return
			}
			stake, err := s.substrateService.TotalHotkeyStake(currAxonInfo.Hotkey)
			if err != nil {
				log.Error().Err(err).Msg("Error getting total hotkey stake")
				return
			}

			isRegistered, err := s.substrateService.CheckIsRegistered(subnetId, currAxonInfo.Hotkey)
			if err != nil {
				log.Error().Err(err).Msg("Error checking if hotkey is registered")
				return
			}

			mutex.Lock()
			hotkeyToStake[currAxonInfo.Hotkey] = stake
			hotkeyToIsRegistered[currAxonInfo.Hotkey] = isRegistered
			mutex.Unlock()

			if !isRegistered {
				log.Warn().Msgf("Hotkey %s is not registered", currAxonInfo.Hotkey)
				s.OnNonRegisteredFound(currAxonInfo.Hotkey)
			}
		}(axonInfo)
	}
	wg.Wait()

	for _, axonInfo := range axonInfos {
		if axonInfo.Hotkey == "" {
			continue
		}

		stake := hotkeyToStake[axonInfo.Hotkey]

		if stake > float64(ValidatorMinStake) {
			validatorHotkeys = append(validatorHotkeys, axonInfo.Hotkey)
		} else {
			minerHotkeys = append(minerHotkeys, axonInfo.Hotkey)
		}
	}

	subnetState.ActiveValidatorHotkeys = validatorHotkeys
	subnetState.ActiveMinerHotkeys = minerHotkeys
	return &subnetState
}

func (s *SubnetStateSubscriber) SubscribeSubnetState(subnetId int) error {
	ticker := time.NewTicker(112 * BlockTimeInSeconds * time.Second)
	s.SubnetState = s.GetSubnetState(subnetId)

	prettySubnetState, err := json.MarshalIndent(s.SubnetState, "", "  ")
	if err != nil {
		log.Error().Err(err).Msg("Error pretty printing subnet state")
	} else {
		fmt.Println("Subnet State:")
		fmt.Println(string(prettySubnetState))
	}

	go func() {
		for range ticker.C {
			s.SubnetState = s.GetSubnetState(subnetId)
		}
	}()
	return nil
}
