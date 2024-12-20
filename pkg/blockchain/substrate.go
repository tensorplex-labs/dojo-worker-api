package blockchain

import (
	"dojo-api/pkg/cache"
	"dojo-api/utils"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

const (
	BlockTimeInSeconds                = 12
	CacheKeyRuntimeSpec        string = "worker_api:runtime_spec"
	CacheKeyMaxUID             string = "worker_api:max_uid"
	CacheKeyHotkeyTemplate     string = "worker_api:sn%d_uid%d_hotkey"
	CacheKeyAxonInfoTemplate   string = "worker_api:sn%d_hotkey%s_axon_info"
	CacheKeyTotalStakeTemplate string = "worker_api:hotkey%s_total_stake"
	maxRetries                        = 5
	baseDelay                         = 2 * time.Second
	maxDelay                          = 10 * time.Second
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
	httpClient      *http.Client
	cache           *cache.Cache
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
}

type Participant struct {
	Axon   AxonInfo `json:"axon"`
	Hotkey string   `json:"hotkey"`
	Uid    int      `json:"uid"`
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

	httpClient := &http.Client{
		Timeout:   30 * time.Second,
		Transport: &http.Transport{},
	}

	return &SubstrateService{substrateApiUrl: substrateApiHost, httpClient: httpClient, cache: cache.GetCacheInstance()}
}

func getCachedData[T any](cache *cache.Cache, cacheKey string) (*T, error) {
	if cache == nil {
		return nil, errors.New("cache is nil")
	}

	cachedData, err := cache.Get(cacheKey)
	if err != nil {
		return nil, err
	}
	if cachedData != "" {
		var data T
		if reflect.TypeOf(data) == reflect.TypeOf("") {
			_, ok := interface{}(&data).(string)
			if ok {
				log.Debug().Msgf("Cache hit for %s", cacheKey)
				return &data, nil
			}
		} else {
			// For other types, use JSON unmarshaling
			err = json.Unmarshal([]byte(cachedData), &data)
			if err == nil {
				log.Debug().Msgf("Cache hit for %s", cacheKey)
				return &data, nil
			} else {
				log.Error().Err(err).Msgf("Error unmarshalling cached %s, querying again.", cacheKey)
			}
		}
	}
	return nil, err
}

func (s *SubstrateService) GetStorageRequest(path string, params url.Values) (*StorageResponse, error) {
	var lastErr error

	// Exponential backoff retry
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Calculate base delay with exponential backoff, capped at maxDelay
		delay := time.Duration(math.Min(
			float64(baseDelay)*math.Pow(2, float64(attempt)),
			float64(maxDelay),
		))

		// Add random jitter between 0 and 3 seconds
		jitter := time.Duration(float64(3*time.Second) * rand.Float64())
		totalDelay := delay + jitter

		response, err := s.executeStorageRequest(path, params)
		if err == nil {
			return response, nil
		}

		lastErr = err

		// Don't sleep on the last attempt
		if attempt < maxRetries {
			log.Warn().
				Err(err).
				Int("attempt", attempt+1).
				Float64("totalDelay_seconds", totalDelay.Seconds()).
				Str("path", path).
				Msg("Request failed, retrying...")

			time.Sleep(totalDelay)
		}
	}

	return nil, fmt.Errorf("all retry attempts failed after %d attempts: %w", maxRetries, lastErr)
}

func (s *SubstrateService) executeStorageRequest(path string, params url.Values) (*StorageResponse, error) {
	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.URL.RawQuery = params.Encode()

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	// Ensure the response body is closed
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// check if the body is empty
	if len(body) == 0 {
		log.Error().Msg("Empty response body")
		return nil, fmt.Errorf("empty response body")
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var storageResponse StorageResponse
	if err := json.Unmarshal(body, &storageResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w, body: %s", err, string(body))
	}

	return &storageResponse, nil
}

func (s *SubstrateService) GetMaxUID(subnetId int) (int, error) {
	cachedMaxUID, err := getCachedData[int](s.cache, CacheKeyMaxUID)
	if err == nil && cachedMaxUID != nil {
		return *cachedMaxUID, nil
	}

	path := fmt.Sprintf("%s/pallets/subtensorModule/storage/SubnetworkN", s.substrateApiUrl)
	params := url.Values{}
	params.Add("keys[]", strconv.Itoa(subnetId))
	storageResponse, err := s.GetStorageRequest(path, params)
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

	s.cache.SetWithExpire(CacheKeyMaxUID, valueStr, 5*BlockTimeInSeconds*time.Second)
	return maxUID, nil
}

func (s *SubstrateService) GetHotkeyByUid(subnetId int, uid int) (string, error) {
	cacheKey := fmt.Sprintf(CacheKeyHotkeyTemplate, subnetId, uid)
	cachedHotkey, err := getCachedData[string](s.cache, cacheKey)
	if err == nil && cachedHotkey != nil {
		return *cachedHotkey, nil
	}

	path := fmt.Sprintf("%s/pallets/subtensorModule/storage/Keys", s.substrateApiUrl)
	params := url.Values{}
	params.Add("keys[]", strconv.Itoa(subnetId))
	params.Add("keys[]", strconv.Itoa(uid))
	storageResponse, err := s.GetStorageRequest(path, params)
	if err != nil {
		log.Error().Err(err).Msgf("Error getting hotkey for uid %d", uid)
		return "", err
	}

	valueStr := fmt.Sprintf("%v", storageResponse.Value)
	log.Debug().Msgf("Hotkey for uid %d: %s", uid, valueStr)

	// expire for every block
	s.cache.SetWithExpire(cacheKey, string(valueStr), BlockTimeInSeconds*time.Second)
	return valueStr, nil
}

func (s *SubstrateService) GetAxonInfo(subnetId int, hotkey string) (*AxonInfo, error) {
	if hotkey == "" {
		return nil, errors.New("hotkey is empty")
	}

	cacheKey := fmt.Sprintf(CacheKeyAxonInfoTemplate, subnetId, hotkey)
	cachedAxonInfo, err := getCachedData[AxonInfo](s.cache, cacheKey)
	if err == nil && cachedAxonInfo != nil {
		return cachedAxonInfo, nil
	}

	path := fmt.Sprintf("%s/pallets/subtensorModule/storage/Axons", s.substrateApiUrl)
	params := url.Values{}
	params.Add("keys[]", strconv.Itoa(subnetId))
	params.Add("keys[]", hotkey)
	storageResponse, err := s.GetStorageRequest(path, params)
	if err != nil {
		log.Error().Err(err).Msgf("Error getting axon info for hotkey %s", hotkey)
		return nil, err
	}

	if storageResponse.Value == nil {
		log.Debug().Msgf("Value is nil for hotkey %s, means they are not serving an axon", hotkey)
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
	// expire every minute
	s.cache.SetWithExpire(cacheKey, string(valueBytes), 5*BlockTimeInSeconds*time.Second)
	return &axonInfoValue, nil
}

func (s *SubstrateService) GetAllParticipants(subnetId int) ([]Participant, error) {
	maxUid, err := s.GetMaxUID(subnetId)
	log.Info().Msgf("Max UID for subnet %d: %d", subnetId, maxUid)
	if err != nil {
		return nil, err
	}

	var allParticipants []Participant = make([]Participant, 0)
	participantChan := make(chan Participant)
	go func() {
		wg := sync.WaitGroup{}
		for uid := 0; uid < maxUid; uid++ {
			wg.Add(1)
			go func(neuronUid int) {
				defer wg.Done()

				currParticipant := Participant{}
				hotkey, err := s.GetHotkeyByUid(subnetId, neuronUid)
				if err != nil {
					log.Error().Err(err).Msgf("Error getting hotkey for uid %d", neuronUid)
					return
				}
				axonInfo, _ := s.GetAxonInfo(subnetId, hotkey)
				// if axon is served we will store the data for the participant
				if axonInfo != nil {
					currParticipant.Axon = *axonInfo
				}
				currParticipant.Hotkey = hotkey
				currParticipant.Uid = neuronUid

				// place it in the channel
				participantChan <- currParticipant
			}(uid)
		}
		wg.Wait()
		close(participantChan)
	}()

	for participant := range participantChan {
		if participant.Axon != (AxonInfo{}) {
			ipAddress := utils.IpDecimalToDotted(participant.Axon.IpDecimal)
			if ipAddress != "" {
				participant.Axon.IpAddress = ipAddress
			}
		} else {
			log.Debug().Msgf("Axon info for uid %d is empty, skipping", participant.Uid)
		}
		log.Debug().Msgf("Axon info for uid %d: %+v", participant.Uid, participant)
		allParticipants = append(allParticipants, participant)
	}
	return allParticipants, nil
}

func (s *SubstrateService) CheckIsRegistered(subnetUid int, hotkey string) (bool, error) {
	path := fmt.Sprintf("%s/pallets/subtensorModule/storage/IsNetworkMember", s.substrateApiUrl)
	params := url.Values{}
	params.Add("keys[]", hotkey)
	params.Add("keys[]", strconv.Itoa(subnetUid))
	storageResponse, err := s.GetStorageRequest(path, params)
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
	cacheKey := fmt.Sprintf(CacheKeyTotalStakeTemplate, hotkey)
	cachedTotalStake, err := getCachedData[float64](s.cache, cacheKey)
	if err == nil && cachedTotalStake != nil {
		return *cachedTotalStake, nil
	}

	path := fmt.Sprintf("%s/pallets/subtensorModule/storage/TotalHotkeyStake", s.substrateApiUrl)
	params := url.Values{}
	params.Add("keys[]", hotkey)
	storageResponse, err := s.GetStorageRequest(path, params)
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
		if tokenSymbol == "TAO" || tokenSymbol == "testTAO" {
			tokenDecimals, err := strconv.Atoi(runtimeSpec.Properties.TokenDecimals[i])
			if err != nil {
				log.Error().Err(err).Msg("Error converting token decimals to int")
				return 0, err
			}
			parsedStake := float64(totalHotkeyStake) / math.Pow10(tokenDecimals)
			log.Debug().Msgf("Hotkey: %+v, raw stake: %+v, parsed stake: %+v", hotkey, totalHotkeyStake, parsedStake)

			// -1 == as many d.p. needed, 64 == float64, 'f' == format
			parsedStakeStr := strconv.FormatFloat(parsedStake, 'f', -1, 64)
			s.cache.SetWithExpire(cacheKey, parsedStakeStr, BlockTimeInSeconds*time.Second)
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
	cachedSpec, err := getCachedData[RuntimeSpec](s.cache, CacheKeyRuntimeSpec)
	if err == nil && cachedSpec != nil {
		return cachedSpec, nil
	}

	path := fmt.Sprintf("%s/runtime/spec", s.substrateApiUrl)
	req, err := http.NewRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	// Ensure the response body is closed
	defer resp.Body.Close()
	// close connection after request
	req.Close = true
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

	s.cache.SetWithExpire(CacheKeyRuntimeSpec, string(body), 24*time.Hour)
	return &runtimeSpec, nil
}
