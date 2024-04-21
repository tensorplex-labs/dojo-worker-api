package blockchain

import (
	"dojo-api/utils"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

const (
	BlockTimeInSeconds = 12
)

type StorageResponse struct {
	At struct {
		Hash   string `json:"hash"`
		Height string `json:"height"`
	} `json:"at"`
	Pallet      string      `json:"pallet"`
	PalletIndex string      `json:"palletIndex"`
	StorageItem string      `json:"storageItem"`
	Value       interface{} `json:"value"`
}

type SubnetInfo struct {
	SubnetId         int
	ValidatorHotkeys []string
	MinerHotkeys     []string
	AxonInfos        []AxonInfo
}

type SubstrateService struct {
	substrateApiUrl string
	SubnetInfos     map[int]SubnetInfo
}

type AxonInfo struct {
	Block        string `json:"block"`
	Version      string `json:"version"`
	IpDecimal    string `json:"ip"`
	Port         string `json:"port"`
	IpType       string `json:"ipType"`
	Protocol     string `json:"protocol"`
	Placeholder1 string `json:"placeholder1"`
	Placeholder2 string `json:"placeholder2"`

	// additional fields we need, to store it in a more organized manner
	IpAddress string `json:"-"`
	Hotkey    string `json:"-"`
	Uid       int    `json:"-"`
}

func NewSubstrateService() *SubstrateService {
	err := godotenv.Load()
	if err != nil {
		log.Fatal().Msg("Error loading .env file")
	}

	substrateApiHost := os.Getenv("SUBSTRATE_API_URL")
	if substrateApiHost == "" {
		log.Fatal().Msg("SUBSTRATE_API_URL must be set")
	}

	return &SubstrateService{substrateApiUrl: substrateApiHost}
}

func DoGetRequest(path string, params url.Values) (*StorageResponse, error) {
	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	req.URL.RawQuery = params.Encode()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var storageResponse StorageResponse
	err = json.Unmarshal(body, &storageResponse)
	if err != nil {
		return nil, err
	}
	return &storageResponse, nil
}

func (s *SubstrateService) GetMaxUID(subnetId int) (int, error) {
	path := fmt.Sprintf("http://%s/pallets/subtensorModule/storage/SubnetworkN", s.substrateApiUrl)
	params := url.Values{}
	params.Add("keys[]", strconv.Itoa(subnetId))
	storageResponse, err := DoGetRequest(path, params)
	if err != nil {
		log.Error().Err(err).Msgf("Error getting max UID for subnet %d", subnetId)
		return 0, err
	}
	valueStr := fmt.Sprintf("%v", storageResponse.Value)
	maxUID, err := strconv.Atoi(valueStr)
	if err != nil {
		log.Error().Err(err).Msgf("Error converting max UID to int for subnet %d", subnetId)
		return 0, err
	}
	return maxUID, nil
}

func (s *SubstrateService) GetHotkeyByUid(subnetId int, uid int) (string, error) {
	path := fmt.Sprintf("http://%s/pallets/subtensorModule/storage/Keys", s.substrateApiUrl)
	params := url.Values{}
	params.Add("keys[]", strconv.Itoa(subnetId))
	params.Add("keys[]", strconv.Itoa(uid))
	storageResponse, err := DoGetRequest(path, params)
	if err != nil {
		log.Error().Err(err).Msgf("Error getting hotkey for uid %d", uid)
		return "", err
	}

	valueStr := fmt.Sprintf("%v", storageResponse.Value)
	log.Info().Msgf("Hotkey for uid %d: %s", uid, valueStr)

	return valueStr, nil
}

func (s *SubstrateService) GetAxonInfo(subnetId int, hotkey string) (*AxonInfo, error) {
	if hotkey == "" {
		return nil, errors.New("hotkey is empty")
	}

	path := fmt.Sprintf("http://%s/pallets/subtensorModule/storage/Axons", s.substrateApiUrl)
	params := url.Values{}
	params.Add("keys[]", strconv.Itoa(subnetId))
	params.Add("keys[]", hotkey)
	storageResponse, err := DoGetRequest(path, params)
	if err != nil {
		log.Error().Err(err).Msgf("Error getting axon info for hotkey %s", hotkey)
		return nil, err
	}

	if storageResponse.Value == nil {
		log.Error().Msgf("Value is nil for hotkey %s, means they are not serving an axon", hotkey)
		return nil, errors.New("value is nil")
	}

	valueBytes, err := json.Marshal(storageResponse.Value)
	if err != nil {
		return nil, err
	}

	var axonInfoValue AxonInfo
	err = json.Unmarshal(valueBytes, &axonInfoValue)
	if err != nil {
		log.Error().Err(err).Msgf("Error unmarshalling axon info value for hotkey %s", hotkey)
		return nil, err
	}

	log.Info().Msgf("Axon info for hotkey %s: %+v", hotkey, axonInfoValue)
	return &axonInfoValue, nil
}

func (s *SubstrateService) GetAllAxons(subnetId int) ([]AxonInfo, error) {
	maxUid, err := s.GetMaxUID(subnetId)
	if err != nil {
		return nil, err
	}

	var axonInfos []AxonInfo = make([]AxonInfo, maxUid)
	// TODO figure out if there's a way to batch this call
	for uid := 0; uid < maxUid; uid++ {
		// perform actions here
		hotkey, err := s.GetHotkeyByUid(subnetId, uid)
		if err != nil {
			log.Error().Err(err).Msgf("Error getting hotkey for uid %d", uid)
			continue
		}
		axonInfo, err := s.GetAxonInfo(subnetId, hotkey)
		if err != nil {
			log.Error().Err(err).Msgf("Error getting axon info for hotkey %s", hotkey)
			continue
		}

		// process our axon infos here
		axonInfo.IpAddress = utils.IpDecimalToDotted(axonInfo.IpDecimal)
		axonInfo.Hotkey = hotkey
		axonInfo.Uid = uid

		axonInfos = append(axonInfos, *axonInfo)

		log.Info().Msgf("Axon info for uid %d: %+v", uid, axonInfo)
		// add sleep... be nice
		time.Sleep(time.Millisecond * 500)
	}

	return axonInfos, nil
}

func (s *SubstrateService) CheckIsRegistered(subnetUid int, hotkey string) (bool, error) {
	path := fmt.Sprintf("http://%s/pallets/subtensorModule/storage/IsNetworkMember", s.substrateApiUrl)
	params := url.Values{}
	params.Add("keys[]", hotkey)
	params.Add("keys[]", strconv.Itoa(subnetUid))
	storageResponse, err := DoGetRequest(path, params)
	if err != nil {
		log.Error().Err(err).Msgf("Error checking if hotkey %s is registered", hotkey)
		return false, err
	}
	storageResponseValue, ok := storageResponse.Value.(bool)
	if !ok {
		log.Error().Msgf("Error converting storage response value to bool for hotkey %s", hotkey)
		return false, fmt.Errorf("error converting storage response value to bool for hotkey %s", hotkey)
	}
	return storageResponseValue, nil
}

func (s *SubstrateService) SubscribeAxonInfos(subnetId int) error {
	ticker := time.NewTicker(112 * BlockTimeInSeconds * time.Second)
	// execute once then enter go routine
	subnetInfos := make(map[int]SubnetInfo)
	axonInfos, err := s.GetAllAxons(subnetId)
	subnetInfos[subnetId] = SubnetInfo{SubnetId: subnetId, AxonInfos: axonInfos}
	s.SubnetInfos = subnetInfos

	if err != nil {
		log.Error().Err(err).Msg("Error getting all axons")
		return err
	}

	go func() {
		for range ticker.C {
			axonInfos, err := s.GetAllAxons(subnetId)
			if err != nil {
				log.Error().Err(err).Msg("Error getting all axons")
				return
			}

			// update subnet info
			s.SubnetInfos[subnetId] = SubnetInfo{SubnetId: subnetId, AxonInfos: axonInfos}
		}
	}()
	return nil
}
