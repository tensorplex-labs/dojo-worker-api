package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"dojo-api/pkg/auth"
	"dojo-api/pkg/blockchain"
	"dojo-api/pkg/cache"
	"dojo-api/pkg/orm"
	"dojo-api/pkg/worker"

	"github.com/gorilla/securecookie"
	"github.com/redis/go-redis/v9"
	"github.com/spruceid/siwe-go"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog/log"
)

// AuthMiddleware checks if the request is authenticated
func WorkerAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Info().Msg("Authenticating token")

		jwtSecret := os.Getenv("JWT_SECRET")
		token := c.GetHeader("Authorization")
		if token == "" {
			log.Error().Msg("No Authorization token provided")
			c.JSON(http.StatusUnauthorized, defaultErrorResponse("unauthorized"))
			c.Abort()
			return
		}
		tokenString := token[7:]
		claims := &jwt.RegisteredClaims{}
		parsedToken, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})

		if err != nil || !parsedToken.Valid {
			log.Error().Err(err).Msg("Invalid token")
			c.JSON(http.StatusUnauthorized, defaultErrorResponse("invalid token"))
			c.Abort()
			return
		}

		if claims.ExpiresAt.Unix() < time.Now().Unix() {
			log.Error().Msg("Token expired")
			c.JSON(http.StatusUnauthorized, defaultErrorResponse("token expired"))
			c.Abort()
			return
		}

		log.Info().Msg("Token authenticated successfully")

		c.Set("userInfo", claims)
		c.Next()
	}
}

func WorkerLoginMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var requestBody worker.WorkerLoginRequest
		if err := c.BindJSON(&requestBody); err != nil {
			log.Error().Err(err).Msg("Invalid request body")
			c.JSON(http.StatusBadRequest, defaultErrorResponse("invalid request body"))
			c.Abort()
			return
		}

		walletAddress := requestBody.WalletAddress
		chainId := requestBody.ChainId
		signature := requestBody.Signature
		message := requestBody.Message
		timestamp := requestBody.Timestamp

		timestampInt, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil {
			log.Error().Err(err).Msg("Invalid timestamp format")
			c.JSON(http.StatusBadRequest, defaultErrorResponse("invalid timestamp format"))
			c.Abort()
			return
		}

		if !isTimestampValid(timestampInt) {
			log.Error().Msg("Timestamp is invalid or expired")
			c.JSON(http.StatusBadRequest, defaultErrorResponse("Bad request"))
			c.Abort()
			return
		}

		verified, err := verifySignature(walletAddress, message, signature)
		if err != nil || !verified {
			log.Error().Err(err).Msg("Invalid signature")
			c.JSON(http.StatusUnauthorized, defaultErrorResponse("Invalid signature"))
			c.Abort()
			return
		}

		valid, err := verifyEthereumAddress(walletAddress)
		if err != nil {
			log.Error().Err(err).Msg("Error verifying Ethereum address")
			c.JSON(http.StatusUnauthorized, defaultErrorResponse("Error verifying Ethereum address"))
			c.Abort()
			return
		}

		if !valid {
			log.Error().Msg("Invalid Ethereum address")
			c.JSON(http.StatusUnauthorized, defaultErrorResponse("Invalid Ethereum address"))
			c.Abort()
			return
		}

		// Generate JWT token
		token, err := generateJWT(walletAddress)
		if err != nil {
			log.Error().Err(err).Msg("Failed to generate token")
			c.JSON(http.StatusInternalServerError, defaultErrorResponse("failed to generate token"))
			c.Abort()
			return
		}

		log.Info().Str("walletAddress", walletAddress).Msg("Ethereum address verified and JWT token generated successfully")
		c.Set("JWTToken", token)
		c.Set("WalletAddress", walletAddress)
		c.Set("ChainId", chainId)
		c.Next()
	}
}

func generateJWT(walletAddress string) (string, error) {
	jwtSecret := os.Getenv("JWT_SECRET")
	tokenExpiry, _ := strconv.Atoi(os.Getenv("TOKEN_EXPIRY"))
	claims := &jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(tokenExpiry) * time.Hour)),
		Issuer:    "dojo-api",
		Subject:   walletAddress,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		log.Error().Err(err).Msg("Error signing JWT token")
		return "", err
	}
	log.Info().Str("walletAddress", walletAddress).Msg("JWT token generated")
	return signedToken, nil
}

func verifyEthereumAddress(address string) (bool, error) {
	ethereumNode := os.Getenv("ETHEREUM_NODE")
	client, err := rpc.DialContext(context.Background(), ethereumNode)
	if err != nil {
		log.Error().Err(err).Msg("Failed to dial Ethereum node")
		return false, err
	}
	defer client.Close()

	account := common.HexToAddress(address)
	if !common.IsHexAddress(address) {
		log.Error().Msg("Invalid Ethereum address format")
		return false, fmt.Errorf("invalid Ethereum address format")
	}

	var balance string
	err = client.CallContext(context.Background(), &balance, "eth_getBalance", account, "latest")
	if err != nil {
		log.Error().Err(err).Msg("Error calling eth_getBalance")
		return false, err
	}

	log.Info().Str("address", address).Msg("Ethereum address verified successfully")
	// If balance retrieval was successful and the balance is equal to or greater than 0, the address is considered valid
	return true, nil
}

func verifySignature(walletAddress string, message string, signatureHex string) (bool, error) {
	messageDomain, err := siwe.ParseMessage(message)
	if err != nil {
		log.Error().Err(err).Msg("Failed to parse SIWE message")
		return false, fmt.Errorf("failed to parse SIWE message: %v", err)
	}
	log.Info().Str("SIWE Message", messageDomain.String()).Msg("SIWE message parsed successfully")

	cache := cache.GetCacheInstance()
	addressNonce, err := cache.Get(walletAddress)
	if err != nil {
		log.Error().Str("walletAddress", walletAddress).Err(err).Msg("Failed to retrieve nonce from cache")
		return false, fmt.Errorf("failed to retrieve nonce from cache: %v", err)
	}

	nonceFromMessage := messageDomain.GetNonce()
	if nonceFromMessage != addressNonce {
		log.Error().Str("cachedNonce", addressNonce).Str("messageNonce", nonceFromMessage).Msg("Nonce mismatch")
		return false, fmt.Errorf("nonce mismatch: expected %s, got %s", addressNonce, nonceFromMessage)
	}

	verifiedPublicKey, err := messageDomain.Verify(signatureHex, nil, nil, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to verify signature with SIWE")
		return false, fmt.Errorf("failed to verify signature with SIWE: %v", err)
	}

	if verifiedPublicKey == nil {
		log.Error().Msg("Signature verification failed")
		return false, fmt.Errorf("signature verification failed")
	}

	recoveredAddr := crypto.PubkeyToAddress(*verifiedPublicKey).Hex()
	if !strings.EqualFold(recoveredAddr, walletAddress) {
		log.Error().Str("recoveredAddress", recoveredAddr).Str("walletAddress", walletAddress).Msg("Recovered address does not match wallet address")
		return false, fmt.Errorf("recovered address does not match wallet address")
	}
	log.Info().Str("walletAddress", walletAddress).Msg("Signature verified successfully with SIWE")
	return true, nil
}

func MinerAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Info().Msg("Authenticating miner user")

		apiKey := c.GetHeader("X-API-KEY")
		if apiKey == "" {
			log.Error().Msg("API key is required")
			c.JSON(http.StatusBadRequest, defaultErrorResponse("API key is required"))
			c.Abort()
			return
		}
		foundApiKey, err := orm.NewApiKeyORM().GetByApiKey(apiKey)
		if err != nil {
			log.Error().Err(err).Msg("Failed to retrieve user by API key")
			c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to retrieve user by API key"))
			c.Abort()
			return
		}

		if foundApiKey == nil {
			log.Error().Msg("API key not found")
			c.JSON(http.StatusUnauthorized, defaultErrorResponse("API key not found"))
			c.Abort()
			return
		}

		if foundApiKey.IsDelete {
			log.Error().Msg("API key has been disabled")
			c.AbortWithStatusJSON(http.StatusUnauthorized, defaultErrorResponse("Invalid API Key"))
			return
		}

		subnetState := blockchain.GetSubnetStateSubscriberInstance()
		_, isFound := subnetState.FindMinerHotkeyIndex(foundApiKey.MinerUser().Hotkey)

		if !isFound {
			log.Error().Msg("Miner hotkey is deregistered")
			c.AbortWithStatusJSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
			return
		}

		c.Set("minerUser", foundApiKey.MinerUser())
		log.Info().Msg("Miner user authenticated successfully")

		c.Next()
	}
}

func generateRandomApiKey() (string, time.Time, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		log.Error().Stack().Err(err).Msg("Error generating random bytes")
		return "", time.Time{}, err
	}
	key := hex.EncodeToString(b)
	key = "sk-" + key

	expiry := time.Now().Add(time.Hour * 24)
	return key, expiry, nil
}

func isTimestampValid(requestTimestamp int64) bool {
	const tolerance = 15 * 60 // 15 minutes in seconds
	currentTime := time.Now().Unix()
	return requestTimestamp <= currentTime && currentTime-requestTimestamp <= tolerance
}

func MinerCookieAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		cookie, err := c.Cookie(auth.CookieName)
		if err != nil {
			log.Error().Err(err).Msgf("Failed to retrieve named cookie %v", auth.CookieName)
			c.AbortWithStatusJSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
			return
		}

		cache := cache.GetCacheInstance()
		result, err := cache.Redis.Get(c.Request.Context(), cookie).Result()
		if err != nil {
			if err == redis.Nil {
				log.Error().Err(err).Msg("Cookie not found in cache")
				c.AbortWithStatusJSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
				return
			}
			log.Error().Err(err).Msg("Failed to retrieve cookie from cache")
			c.AbortWithStatusJSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
			return
		}

		var session auth.SecureCookieSession
		if err := json.Unmarshal([]byte(result), &session); err != nil {
			log.Error().Err(err).Msg("Failed to unmarshal redis data from JSON")
			c.AbortWithStatusJSON(http.StatusInternalServerError, defaultErrorResponse("Failed to process authentication data"))
			return
		}

		blockKey := session.BlockKey
		hashKey := session.HashKey
		// reconstruct secure cookie
		s := securecookie.New(hashKey, blockKey)
		var cookieData auth.CookieData
		if err = s.Decode(auth.CookieName, cookie, &cookieData); err != nil {
			log.Error().Err(err).Msg("Failed to decode cookie")
			c.AbortWithStatusJSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
			return
		}

		if session.CookieData.Hotkey != cookieData.Hotkey || session.CookieData.SessionId != cookieData.SessionId {
			log.Error().Str("expectedHotkey", session.CookieData.Hotkey).Str("actualHotkey", cookieData.Hotkey).
				Str("expectedSessionId", session.CookieData.SessionId).Str("actualSessionId", cookieData.SessionId).
				Msg("Cookie data mismatch")
			c.AbortWithStatusJSON(http.StatusUnauthorized, defaultErrorResponse("Unauthorized"))
			return
		}

		log.Info().Msgf("Cookie validated successfully for hotkey %v, session id %v", session.Hotkey, session.SessionId)
		c.Set("session", session)
		c.Next()
	}
}

func ResourceProfiler() gin.HandlerFunc {
	return func(c *gin.Context) {
		var startMemStats runtime.MemStats
		runtime.ReadMemStats(&startMemStats)

		startTime := time.Now()
		c.Next()

		var endMemStats runtime.MemStats
		runtime.ReadMemStats(&endMemStats)

		duration := time.Since(startTime)

		var memoryAllocated uint64
		if endMemStats.Alloc > startMemStats.Alloc {
			memoryAllocated = endMemStats.Alloc - startMemStats.Alloc
		} else {
			memoryAllocated = endMemStats.Alloc
		}

		memoryAllocatedMB := float64(memoryAllocated) / (1048576.0)

		log.Info().Msgf("Request %s took %s and allocated %.2f MB of memory", c.Request.URL.Path, duration, memoryAllocatedMB)

		// see if we wanna store in context
		c.Set("executionTime", duration)
		c.Set("memoryAllocated", memoryAllocated)
	}
}
