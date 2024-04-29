package blockchain

import (
	"dojo-api/utils"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"

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

type SubstrateService struct {
	substrateApiUrl string
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
	IpAddress string `json:"ipAddress"`
	Hotkey    string `json:"hotkey"`
	Uid       int    `json:"uid"`
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

	log.Debug().Msgf("Axon info for hotkey %s: %+v", hotkey, axonInfoValue)
	return &axonInfoValue, nil
}

func (s *SubstrateService) GetAllAxons(subnetId int) ([]AxonInfo, error) {
	maxUid, err := s.GetMaxUID(subnetId)
	if err != nil {
		return nil, err
	}

	var allAxonInfos []AxonInfo = make([]AxonInfo, 0)
	axonInfoChan := make(chan AxonInfo)
	go func() {
		wg := sync.WaitGroup{}
		for uid := 0; uid < maxUid; uid++ {
			wg.Add(1)
			go func(neuronUid int) {
				defer wg.Done()

				currAxonInfo := AxonInfo{}
				hotkey, err := s.GetHotkeyByUid(subnetId, neuronUid)
				if err != nil {
					log.Error().Err(err).Msgf("Error getting hotkey for uid %d", neuronUid)
					return
				}
				axonInfo, _ := s.GetAxonInfo(subnetId, hotkey)
				// no axon info so avoid putting it onto the channel
				if axonInfo == nil {
					return
				}
				currAxonInfo = *axonInfo
				currAxonInfo.Hotkey = hotkey
				currAxonInfo.Uid = neuronUid

				// place it in the channel
				axonInfoChan <- currAxonInfo
			}(uid)
		}
		wg.Wait()
		close(axonInfoChan)
	}()

	for axonInfo := range axonInfoChan {
		ipAddress := utils.IpDecimalToDotted(axonInfo.IpDecimal)
		if ipAddress != "" {
			axonInfo.IpAddress = ipAddress
		}
		log.Debug().Msgf("Axon info for uid %d: %+v", axonInfo.Uid, axonInfo)
		allAxonInfos = append(allAxonInfos, axonInfo)
	}
	return allAxonInfos, nil
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

func (s *SubstrateService) TotalHotkeyStake(hotkey string) (float64, error) {
	path := fmt.Sprintf("http://%s/pallets/subtensorModule/storage/TotalHotkeyStake", s.substrateApiUrl)
	params := url.Values{}
	params.Add("keys[]", hotkey)
	storageResponse, err := DoGetRequest(path, params)
	if err != nil {
		log.Error().Err(err).Msgf("Error getting total hotkey stake for hotkey %s", hotkey)
		return 0, err
	}
	totalHotkeyStake, err := strconv.Atoi(storageResponse.Value.(string))
	if err != nil {
		log.Error().Err(err).Msgf("Error converting total hotkey stake to int for hotkey %s", hotkey)
		return 0, err
	}

	runtimeSpec, err := s.RuntimeSpec()
	if err != nil {
		log.Error().Err(err).Msg("Error getting runtime spec")
		return 0, err
	}

	for i, tokenSymbol := range runtimeSpec.Properties.TokenSymbol {
		if tokenSymbol == "TAO" {
			tokenDecimals, err := strconv.Atoi(runtimeSpec.Properties.TokenDecimals[i])
			if err != nil {
				log.Error().Err(err).Msg("Error converting token decimals to int")
				return 0, err
			}
			parsedStake := float64(totalHotkeyStake) / math.Pow10(tokenDecimals)
			log.Debug().Msgf("Hotkey: %+v, raw stake: %+v, parsed stake: %+v", hotkey, totalHotkeyStake, parsedStake)
			return parsedStake, nil
		}
	}
	return 0, errors.New("TAO token not found in runtime spec")
}

type ChainType struct {
	Live interface{} `json:"live"`
}

type Properties struct {
	IsEthereum    bool     `json:"isEthereum"`
	Ss58Format    string   `json:"ss58Format"`
	TokenDecimals []string `json:"tokenDecimals"`
	TokenSymbol   []string `json:"tokenSymbol"`
}

type At struct {
	Height string `json:"height"`
	Hash   string `json:"hash"`
}

type RuntimeSpec struct {
	At                 At         `json:"at"`
	AuthoringVersion   string     `json:"authoringVersion"`
	TransactionVersion string     `json:"transactionVersion"`
	ImplVersion        string     `json:"implVersion"`
	SpecName           string     `json:"specName"`
	SpecVersion        string     `json:"specVersion"`
	ChainType          ChainType  `json:"chainType"`
	Properties         Properties `json:"properties"`
}

func (s *SubstrateService) RuntimeSpec() (*RuntimeSpec, error) {
	path := fmt.Sprintf("http://%s/runtime/spec", s.substrateApiUrl)
	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var runtimeSpec RuntimeSpec
	err = json.Unmarshal(body, &runtimeSpec)
	if err != nil {
		log.Error().Err(err).Msg("Error unmarshalling runtime spec")
		return nil, err
	}
	return &runtimeSpec, nil
}
