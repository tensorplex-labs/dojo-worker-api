package blockchain

import (
	"os"
	"strconv"
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
	SubnetId         int
	ValidatorHotkeys []string
	MinerHotkeys     []string
	AxonInfos        []AxonInfo
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

func (s *SubnetStateSubscriber) GetSubnetState(subnetId int) *SubnetState {
	// execute once then enter go routine
	axonInfos, err := s.substrateService.GetAllAxons(subnetId)
	if err != nil {
		log.Error().Err(err).Msg("Error getting all axons")
		return &SubnetState{}
	}

	subnetState := SubnetState{SubnetId: subnetId, AxonInfos: axonInfos}
	// overkill to have 256 slots, but it's fine
	minerHotkeys := make([]string, 1024)
	validatorHotkeys := make([]string, 1024)
	for _, axonInfo := range axonInfos {
		if axonInfo.Hotkey == "" {
			log.Warn().Msgf("AxonInfo empty hotkey, %+v", axonInfo)
			continue
		}
		stake, err := s.substrateService.TotalHotkeyStake(axonInfo.Hotkey)
		if err != nil {
			log.Error().Err(err).Msg("Error getting total hotkey stake")
			continue
		}
		s.GlobalState.HotkeyStakes[axonInfo.Hotkey] = stake

		if stake > float64(ValidatorMinStake) {
			validatorHotkeys = append(validatorHotkeys, axonInfo.Hotkey)
		} else {
			minerHotkeys = append(minerHotkeys, axonInfo.Hotkey)
		}
	}
	subnetState.ValidatorHotkeys = validatorHotkeys
	subnetState.MinerHotkeys = minerHotkeys
	return &subnetState
}

func (s *SubnetStateSubscriber) SubscribeSubnetState(subnetId int) error {
	ticker := time.NewTicker(112 * BlockTimeInSeconds * time.Second)
	s.SubnetState = s.GetSubnetState(subnetId)

	go func() {
		for range ticker.C {
			s.SubnetState = s.GetSubnetState(subnetId)
		}
	}()
	return nil
}
