package api

import (
	"net/http"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog/log"
	"math/big"
    
	"context"
	"fmt"
	"os"
	"time"
)

// AuthMiddleware checks if the request is authenticated
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		jwtSecret := os.Getenv("JWT_SECRET")
		token := c.GetHeader("Authorization")
		if token == "" {
			log.Error().Msg("No Authorization token provided")
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "unauthorized"})
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
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "invalid token"})
			c.Abort()
			return
		}

		if claims.ExpiresAt.Unix() < time.Now().Unix() {
			log.Error().Msg("Token expired")
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "token expired"})
			c.Abort()
			return
		}

		log.Info().Msg("Token authenticated successfully")
		// Pass the claims to the next middleware/handler
		c.Set("userInfo", claims)
		c.Next()
	}
}

func LoginMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var requestMap map[string]string
		if err := c.BindJSON(&requestMap); err != nil {
			log.Error().Err(err).Msg("Invalid request body")
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request body"})
			c.Abort()
			return
		}
		walletAddress, walletExists := requestMap["walletAddress"]
		chainId, chainIdExists := requestMap["chainId"]

		if !walletExists {
			log.Error().Msg("walletAddress is required")
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "walletAddress is required"})
			c.Abort()
			return
		}

		if !chainIdExists {
			log.Error().Msg("chainId is required")
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "chainId is required"})
			c.Abort()
			return
		}

		valid, err := verifyEthereumAddress(walletAddress)
		if err != nil {
			log.Error().Err(err).Msg("Error verifying Ethereum address")
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Error verifying Ethereum address"})
			c.Abort()
			return
		}

		if !valid {
			log.Error().Msg("Invalid Ethereum address")
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Invalid Ethereum address"})
			c.Abort()
			return
		}

		// Generate JWT token
		token, err := generateJWT(walletAddress)
		if err != nil {
			log.Error().Err(err).Msg("Failed to generate token")
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to generate token"})
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
	claims := &jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
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

	// Convert balance to a big.Int to check if it's greater than 0
	balanceBigInt, ok := new(big.Int).SetString(balance[2:], 16) // Remove the 0x prefix and parse
	if !ok {
		log.Error().Msg("Failed to parse balance")
		return false, fmt.Errorf("failed to parse balance")
	}

	if balanceBigInt.Cmp(big.NewInt(0)) < 0 {
		log.Error().Msg("Address has a negative Ether balance")
		return false, fmt.Errorf("address has a negative Ether balance")
	}

	log.Info().Str("address", address).Msg("Ethereum address verified successfully")
	// If balance retrieval was successful and the balance is equal to or greater than 0, the address is considered valid
	return true, nil
}