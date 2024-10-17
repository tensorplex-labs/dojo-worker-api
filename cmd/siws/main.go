package main

import (
	"fmt"

	"dojo-api/pkg/blockchain/siws"
)

func main() {
	signatureFromFE := "0x90d765d7dcbe0fd703156801783eca2e9069dbefd9866cf69da9561504b08a3d5c6fbbc79a444d60e4c1f34c5eb2ff888373d35e493441c44d596fad4a849084"
	message := `siws.xyz wants you to sign in with your Substrate account:
12QPhT1S2hdUqv9b9RXMVHNizq7xhNnh5WG8Gudf6zJjPtwj

Welcome to SIWS! Sign in to see how it works.

URI: https://siws.xyz
Version: 1.0.0
Nonce: 11c4b0b8-4d9f-4341-b095-bdfc1c219182
Issued At: 2024-05-13T06:58:43.959Z
Expiration Time: 2024-05-13T07:00:43.958Z`

	ss58address := "kGiayU6xGtpQKXDaJk3cjJgG3AphQeQ86AXcYnnNFFb8j9Ps5"

	isVerified, err := siws.SS58VerifySignature(message, ss58address, signatureFromFE)
	if err != nil {
		fmt.Println("Error verifying signature:", err)
	} else {
		fmt.Println("Is Verified:", isVerified)
	}

	_, err = siws.ParseMessage(message)
	if err != nil {
		fmt.Println("Error parsing message:", err)
	}
}
