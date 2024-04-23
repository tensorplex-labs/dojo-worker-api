package api

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"net/http"

	"bytes"
	"context"
	"encoding/hex"
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
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		tokenString := token[7:]
		claims := &jwt.RegisteredClaims{}
		parsedToken, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})

		if err != nil || !parsedToken.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		if claims.ExpiresAt.Unix() < time.Now().Unix() {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token expired"})
			c.Abort()
			return
		}

		// Pass the claims to the next middleware/handler
		c.Set("userInfo", claims)
		c.Next()
	}
}

func LoginMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		var requestMap map[string]string
		if err := c.BindJSON(&requestMap); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			c.Abort()
			return
		}
		walletAddress, walletExists := requestMap["walletAddress"]
		signature, signatureExists := requestMap["signature"]

		if !walletExists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "walletAddress is required"})
			c.Abort()
			return
		}

		if !signatureExists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "signature is required"})
			c.Abort()
			return
		}

		isValid, err := validateSignature(walletAddress, signature)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to validate signature"})
			c.Abort()
			return
		}

		if !isValid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
			c.Abort()
			return
		}

		valid, err := verifyEthereumAddress(walletAddress)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Error verifying Ethereum address"})
			c.Abort()
			return
		}

		if !valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid Ethereum address"})
			c.Abort()
			return
		}

		// Generate JWT token
		token, err := generateJWT(walletAddress)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
			c.Abort()
			return
		}

		c.Set("JWTToken", token)
		c.Set("WalletAddress", walletAddress)
		c.Set("Signature", signature)
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
	return token.SignedString([]byte(jwtSecret))
}

func validateSignature(walletAddress string, signature string) (bool, error) {
	// Convert the wallet address to a public key hash
	addressBytes, err := hex.DecodeString(walletAddress)
	if err != nil {
		return false, fmt.Errorf("failed to decode wallet address: %w", err)
	}

	// Convert the signature to bytes
	sigBytes, err := hex.DecodeString(signature)
	if err != nil {
		return false, fmt.Errorf("failed to decode signature: %w", err)
	}

	// The signature should be 65 bytes long
	if len(sigBytes) != 65 {
		return false, fmt.Errorf("invalid signature length")
	}

	// Hash the data to get the message digest
	message := "Authentication"
	msgHash := crypto.Keccak256Hash([]byte(message))

	// Extract the public key from the signature
	sigPublicKeyECDSA, err := crypto.SigToPub(msgHash.Bytes(), sigBytes)
	if err != nil {
		return false, fmt.Errorf("failed to get public key from signature: %w", err)
	}

	// Convert the public key to an address
	recoveredAddr := crypto.PubkeyToAddress(*sigPublicKeyECDSA)

	// Compare the recovered address with the provided wallet address
	if bytes.Equal(addressBytes, recoveredAddr.Bytes()) {
		return true, nil
	}

	return false, nil
}

func verifyEthereumAddress(address string) (bool, error) {
	ethereumNode := os.Getenv("ETHEREUM_NODE")
	client, err := rpc.DialContext(context.Background(), ethereumNode)
	if err != nil {
		return false, err
	}
	defer client.Close()

	account := common.HexToAddress(address)
	var balance string
	err = client.CallContext(context.Background(), &balance, "eth_getBalance", account, "latest")
	if err != nil {
		return false, err
	}

	// If balance retrieval was successful, the address is considered valid
	return true, nil
}
