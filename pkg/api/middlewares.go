package api

import (
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

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

// login middleware for network user
func MinerLoginMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var requestMap map[string]string
		if err := c.BindJSON(&requestMap); err != nil {
			log.Error().Err(err).Msg("Invalid request body")
			c.JSON(http.StatusBadRequest, defaultErrorResponse("invalid request body"))
			c.Abort()
			return
		}

		coldkey, coldkeyExists := requestMap["coldkey"]
		if !coldkeyExists {
			log.Error().Msg("coldkey is required")
			c.JSON(http.StatusBadRequest, defaultErrorResponse("coldkey is required"))
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

		subnetSubscriber := blockchain.NewSubnetStateSubscriber()
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
		c.Set("coldkey", coldkey)
		c.Set("apiKey", apiKey)
		c.Set("expiry", expiry)
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

func verifySignature(walletAddress, message, signatureHex string) (bool, error) {
	// Remove the 0x prefix if present
	signatureHex = strings.TrimPrefix(signatureHex, "0x")

	// Decode the hex-encoded signature
	signatureBytes, err := hex.DecodeString(signatureHex)
	if err != nil {
		log.Error().Err(err).Str("signatureHex", signatureHex).Msg("Failed to decode signature")
		return false, fmt.Errorf("failed to decode signature: %v", err)
	}

	// Adjust the V value in the signature (last byte) to be 0 or 1
	if signatureBytes[64] >= 27 {
		signatureBytes[64] -= 27
	}

	// Hash the message to get the message digest as expected by SigToPub
	msgHash := crypto.Keccak256Hash([]byte(message))

	// Recover the public key from the signature
	pubKey, err := crypto.SigToPub(msgHash.Bytes(), signatureBytes)
	if err != nil {
		log.Error().Err(err).Str("messageHash", msgHash.Hex()).Msg("Failed to recover public key")
		return false, fmt.Errorf("failed to recover public key: %v", err)
	}

	// Convert the recovered public key to an Ethereum address
	recoveredAddr := crypto.PubkeyToAddress(*pubKey).Hex()

	// Compare the recovered address with the wallet address
	if !strings.EqualFold(recoveredAddr, walletAddress) {
		log.Error().Str("recoveredAddress", recoveredAddr).Str("walletAddress", walletAddress).Msg("Recovered address does not match wallet address")
		return false, fmt.Errorf("recovered address %s does not match wallet address %s", recoveredAddr, walletAddress)
	}

	log.Info().Str("walletAddress", walletAddress).Msg("Signature verified successfully")
	return true, nil
}

func MinerAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
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
		c.Set("user", user)
		c.Next()
	}
}

func generateRandomApiKey() (string, time.Time, error) {
	apiKey, err := uuid.NewRandom()
	if err != nil {
		return "", time.Time{}, fmt.Errorf("failed to generate UUID: %v", err)
	}
	expiry := time.Now().Add(24 * time.Hour)
	return apiKey.String(), expiry, nil
}

func isTimestampValid(requestTimestamp int64) bool {
    const tolerance = 15 * 60 // 15 minutes in seconds
    currentTime := time.Now().Unix()
    return requestTimestamp <= currentTime && currentTime - requestTimestamp <= tolerance
}