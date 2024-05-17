package blockchain

import (
	"encoding/json"
	"os"
	"strconv"
	"sync"
	"time"

	"dojo-api/db"
	"dojo-api/pkg/orm"
	"dojo-api/utils"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

var ValidatorMinStake = GetValidatorMinStake()

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
	ActiveValidatorHotkeys map[int]string
	ActiveMinerHotkeys     map[int]string
	ActiveParticipants     []Participant
}

type GlobalState struct {
	HotkeyStakes map[string]float64
}

type SubnetStateSubscriber struct {
	substrateService *SubstrateService
	SubnetState      *SubnetState // meant for only tracking our subnet state
	GlobalState      *GlobalState
	initialised      bool
	mutex            sync.RWMutex
}

var (
	instance *SubnetStateSubscriber
	once     sync.Once
)

func GetSubnetStateSubscriberInstance() *SubnetStateSubscriber {
	once.Do(func() {
		instance = &SubnetStateSubscriber{
			substrateService: NewSubstrateService(),
			SubnetState:      &SubnetState{},
			GlobalState:      &GlobalState{HotkeyStakes: make(map[string]float64)},
			initialised:      false,
		}
		subnetUidStr := utils.LoadDotEnv("SUBNET_UID")
		subnetUid, err := strconv.Atoi(subnetUidStr)
		if err != nil {
			log.Fatal().Err(err).Msg("Error parsing SUBNET_UID, failed to start subscriber")
		}
		instance.SubscribeSubnetState(subnetUid)
	})
	return instance
}

func (s *SubnetStateSubscriber) OnNonRegisteredFound(hotkey string) {
	if hotkey == "" {
		log.Fatal().Msg("Hotkey is empty, cannot remove from active validators/miners/axons")
		return
	}

	// clear from active validators if found
	for key, vhotkey := range s.SubnetState.ActiveValidatorHotkeys {
		if hotkey == vhotkey {
			delete(s.SubnetState.ActiveValidatorHotkeys, key)
			break
		}
	}

	// clear from active miners if found
	for key, mhotkey := range s.SubnetState.ActiveMinerHotkeys {
		if hotkey == mhotkey {
			delete(s.SubnetState.ActiveMinerHotkeys, key)
			break
		}
	}
	// clear from axon infos
	for i, axonInfo := range s.SubnetState.ActiveParticipants {
		if hotkey == axonInfo.Hotkey {
			s.SubnetState.ActiveParticipants = append(s.SubnetState.ActiveParticipants[:i], s.SubnetState.ActiveParticipants[i+1:]...)
			break
		}
	}

	minerUserORM := orm.NewMinerUserORM()
	if err := minerUserORM.DeregisterMiner(hotkey); err != nil {
		log.Error().Err(err).Msg("Error deregistering miner")
	}
}

func (s *SubnetStateSubscriber) OnRegisteredFound(hotkey string) {
	if hotkey == "" {
		log.Fatal().Msg("Hotkey is empty, cannot add to active validators/miners/axons")
		return
	}

	minerUserORM := orm.NewMinerUserORM()
	minerUser, err := minerUserORM.GetUserByHotkey(hotkey)
	if err != nil {
		if err == db.ErrNotFound {
			log.Info().Msg("User not found, continuing...")
			return
		}
		log.Error().Err(err).Msg("Error getting user by hotkey")
		return
	}

	if !minerUser.IsVerified {
		if err := minerUserORM.ReregisterMiner(hotkey); err != nil {
			log.Error().Err(err).Msg("Error reregistering miner")
		}
	}
}

func (s *SubnetStateSubscriber) GetSubnetState(subnetId int) *SubnetState {
	participants, err := s.substrateService.GetAllParticipants(subnetId)
	if err != nil {
		log.Error().Err(err).Msg("Error getting all axons")
		return &SubnetState{}
	}

	subnetState := SubnetState{SubnetId: subnetId, ActiveParticipants: participants}

	hotkeyToStake := make(map[string]float64)
	hotkeyToIsRegistered := make(map[string]bool)

	var wg sync.WaitGroup
	var mutex sync.Mutex

	for _, participant := range participants {
		wg.Add(1)
		go func(currParticipant Participant) {
			defer wg.Done()
			if currParticipant.Hotkey == "" {
				log.Trace().Msgf("AxonInfo empty hotkey, %+v", currParticipant)
				return
			}
			stake, err := s.substrateService.TotalHotkeyStake(currParticipant.Hotkey)
			if err != nil {
				log.Error().Err(err).Msg("Error getting total hotkey stake")
				return
			}

			isRegistered, err := s.substrateService.CheckIsRegistered(subnetId, currParticipant.Hotkey)
			if err != nil {
				log.Error().Err(err).Msg("Error checking if hotkey is registered")
				return
			}

			mutex.Lock()
			hotkeyToStake[currParticipant.Hotkey] = stake
			hotkeyToIsRegistered[currParticipant.Hotkey] = isRegistered
			mutex.Unlock()
		}(participant)
	}
	wg.Wait()

	activeMinerHotkeys := make(map[int]string)
	activeValidatorHotkeys := make(map[int]string)
	// here we only consider active participants
	for _, participant := range participants {
		hotkey := participant.Hotkey
		if hotkey == "" {
			continue
		}
		isRegistered := hotkeyToIsRegistered[participant.Hotkey]
		if !isRegistered {
			log.Warn().Msgf("Hotkey %s is not registered", hotkey)
			continue
		}

		stake := hotkeyToStake[participant.Hotkey]
		if stake > float64(ValidatorMinStake) {
			activeValidatorHotkeys[participant.Uid] = participant.Hotkey
		} else {
			activeMinerHotkeys[participant.Uid] = participant.Hotkey
		}
	}

	// handle deregistrations
	for hotkey, isRegistered := range hotkeyToIsRegistered {
		if !isRegistered {
			s.OnNonRegisteredFound(hotkey)
		} else {
			s.OnRegisteredFound(hotkey)
		}
	}

	subnetState.ActiveValidatorHotkeys = activeValidatorHotkeys
	subnetState.ActiveMinerHotkeys = activeMinerHotkeys

	return &subnetState
}

func (s *SubnetStateSubscriber) IsInitialised() bool {
	return s.initialised
}

func (s *SubnetStateSubscriber) SubscribeSubnetState(subnetId int) error {
	ticker := time.NewTicker(5 * BlockTimeInSeconds * time.Second)
	s.mutex.Lock()
	s.SubnetState = s.GetSubnetState(subnetId)
	s.initialised = true
	s.mutex.Unlock()

	prettySubnetState, err := json.MarshalIndent(s.SubnetState, "", "  ")
	if err != nil {
		log.Error().Err(err).Msg("Error pretty printing subnet state")
	} else {
		log.Debug().Msgf("Subnet State:")
		log.Debug().Msgf(string(prettySubnetState))
	}

	go func() {
		for range ticker.C {
			s.mutex.Lock()
			s.SubnetState = s.GetSubnetState(subnetId)
			s.mutex.Unlock()
		}
	}()
	return nil
}

func (s *SubnetStateSubscriber) FindMinerHotkeyIndex(hotkey string) (int, bool) {
	for uid, mhotkey := range s.SubnetState.ActiveMinerHotkeys {
		if hotkey == mhotkey {
			return uid, true
		}
	}
	return -1, false
}

func (s *SubnetStateSubscriber) FindValidatorHotkeyIndex(hotkey string) (int, bool) {
	// TODO fix why validator hotkey changes so quickly, should be a bug
	for uid, vhotkey := range s.SubnetState.ActiveValidatorHotkeys {
		if hotkey == vhotkey {
			return uid, true
		}
	}
	return -1, false
}
