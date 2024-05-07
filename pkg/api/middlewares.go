package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spruceid/siwe-go"

	"dojo-api/pkg/blockchain"
	"dojo-api/pkg/orm"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
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
		var requestMap map[string]string
		if err := c.BindJSON(&requestMap); err != nil {
			log.Error().Err(err).Msg("Invalid request body")
			c.JSON(http.StatusBadRequest, defaultErrorResponse("invalid request body"))
			c.Abort()
			return
		}
		walletAddress, walletExists := requestMap["walletAddress"]
		chainId, chainIdExists := requestMap["chainId"]
		signature, signatureExists := requestMap["signature"]
		message, messageExists := requestMap["message"]
		timestamp, timestampExists := requestMap["timestamp"]

		if !timestampExists {
			log.Error().Msg("Timestamp is missing")
			c.JSON(http.StatusBadRequest, defaultErrorResponse("timestamp is missing"))
			c.Abort()
			return
		}

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

		if !walletExists {
			log.Error().Msg("walletAddress is required")
			c.JSON(http.StatusBadRequest, defaultErrorResponse("walletAddress is required"))
			c.Abort()
			return
		}

		if !chainIdExists {
			log.Error().Msg("chainId is required")
			c.JSON(http.StatusBadRequest, defaultErrorResponse("chainId is required"))
			c.Abort()
			return
		}

		if !messageExists {
			log.Error().Msg("message is required")
			c.JSON(http.StatusBadRequest, defaultErrorResponse("message is required"))
			c.Abort()
			return
		}

		if !signatureExists {
			log.Error().Msg("signature is required")
			c.JSON(http.StatusBadRequest, defaultErrorResponse("signature is required"))
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

// login middleware for miner user
func MinerLoginMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var requestMap map[string]string
		if err := c.BindJSON(&requestMap); err != nil {
			log.Error().Err(err).Msg("Invalid request body")
			c.JSON(http.StatusBadRequest, defaultErrorResponse("invalid request body"))
			c.Abort()
			return
		}

		hotkey, hotkeyExists := requestMap["hotkey"]
		if !hotkeyExists {
			log.Error().Msg("hotkey is required")
			c.JSON(http.StatusBadRequest, defaultErrorResponse("hotkey is required"))
			c.Abort()
			return
		}

		subnetSubscriber := blockchain.GetSubnetStateSubscriberInstance()
		_, found := subnetSubscriber.FindMinerHotkeyIndex(hotkey)
		var verified bool
		var apiKey string
		var expiry time.Time
		if found {
			verified = true
			minerUserORM := orm.NewMinerUserORM()
			minerUser, err := minerUserORM.GetUserByAPIKey(hotkey)
			if err != nil || minerUser == nil || minerUser.APIKeyExpireAt.Before(time.Now()) {
				apiKey, expiry, err = generateRandomApiKey()
				if err != nil {
					log.Error().Err(err).Msg("Failed to generate random API key")
					c.JSON(http.StatusInternalServerError, defaultErrorResponse("failed to generate random API key"))
					c.Abort()
					return
				}
			} else {
				apiKey = minerUser.APIKey
				expiry = minerUser.APIKeyExpireAt
			}
		} else {
			verified, apiKey, expiry = false, "", time.Time{}
		}

		c.Set("verified", verified)
		c.Set("hotkey", hotkey)
		c.Set("apiKey", apiKey)
		c.Set("expiry", expiry)
		c.Set("email", requestMap["email"])
		c.Set("organisationName", requestMap["organisationName"])
		c.Next()
	}
}

func MinerVerificationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var requestMap map[string]string
		if err := c.BindJSON(&requestMap); err != nil {
			log.Error().Err(err).Msg("Invalid request body")
			c.JSON(http.StatusBadRequest, defaultErrorResponse("invalid request body"))
			c.Abort()
			return
		}

		hotkey, ok := requestMap["hotkey"]
		if !ok {
			log.Error().Msg("hotkey is required")
			c.JSON(http.StatusBadRequest, defaultErrorResponse("hotkey is required"))
			c.Abort()
			return
		}

		if _, ok := requestMap["email"]; !ok {
			log.Error().Msg("email is required")
			c.JSON(http.StatusBadRequest, defaultErrorResponse("email is required"))
			c.Abort()
			return
		}

		subnetSubscriber := blockchain.GetSubnetStateSubscriberInstance()
		_, found := subnetSubscriber.FindMinerHotkeyIndex(hotkey)
		if !found {
			log.Error().Msg("Hotkey is not registered")
			c.JSON(http.StatusUnauthorized, defaultErrorResponse("hotkey is not registered"))
			c.Abort()
			return
		}

		c.Set("requestMap", requestMap)
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

	cache := GetCacheInstance()
	addressNonce, err := cache.Get(walletAddress)
	if err != nil {
		log.Error().Str("walletAddress", walletAddress).Err(err).Msg("Failed to retrieve nonce from cache")
		return false, fmt.Errorf("failed to retrieve nonce from cache: %v", err)
	}

	nonceFromMessage := messageDomain.GetNonce()
	addressNonceStr, ok := addressNonce.(string)
	if !ok || nonceFromMessage != addressNonceStr {
		log.Error().Str("expectedNonce", addressNonceStr).Str("actualNonce", nonceFromMessage).Msg("Nonce mismatch")
		return false, fmt.Errorf("nonce mismatch: expected %s, got %s", addressNonceStr, nonceFromMessage)
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
		minerUserORM := orm.NewMinerUserORM()
		user, err := minerUserORM.GetUserByAPIKey(apiKey)
		if err != nil {
			log.Error().Err(err).Msg("Failed to retrieve user by API key")
			c.JSON(http.StatusInternalServerError, defaultErrorResponse("Failed to retrieve user by API key"))
			c.Abort()
			return
		}

		if user.APIKeyExpireAt.Before(time.Now()) {
			log.Error().Msg("API key has expired")
			c.JSON(http.StatusUnauthorized, defaultErrorResponse("API key has expired"))
			c.Abort()
			return
		}

		c.Set("minerUser", user)

		log.Info().Msg("Miner user authenticated successfully")

		c.Next()
	}
}

func generateRandomApiKey() (string, time.Time, error) {
	apiKey, err := uuid.NewRandom()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to generate UUID: %v", err)
	}
	expiry := time.Now().Add(24 * time.Hour)
	return "sk-" + apiKey.String(), expiry, nil
}

func isTimestampValid(requestTimestamp int64) bool {
	const tolerance = 15 * 60 // 15 minutes in seconds
	currentTime := time.Now().Unix()
	return requestTimestamp <= currentTime && currentTime-requestTimestamp <= tolerance
}
