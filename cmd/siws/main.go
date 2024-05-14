package main

import (
	"dojo-api/pkg/blockchain/siws"
	"fmt"
)

func main() {
	// signatureFromFE := "***REMOVED***"
	// message := "siws.xyz wants you to sign in with your Bittensor account:\n***REMOVED***\n\nWelcome to SIWS! Sign in to see how it works.\n\nURI: https://siws.xyz\nNonce: 3333dae4-57de-40d7-873d-3b81a0847ad5\nIssued At: 2024-05-10T16:36:48.530Z\nExpiration Time: 2024-05-10T16:38:48.530Z"
	signatureFromFE := "0x90d765d7dcbe0fd703156801783eca2e9069dbefd9866cf69da9561504b08a3d5c6fbbc79a444d60e4c1f34c5eb2ff888373d35e493441c44d596fad4a849084"
	// SIWS message with version (0.0.9)
	// message := "siws.xyz wants you to sign in with your Polkadot account:\n12QPhT1S2hdUqv9b9RXMVHNizq7xhNnh5WG8Gudf6zJjPtwj\n\nWelcome to SIWS! Sign in to see how it works.\n\nURI: https://siws.xyz\nNonce: 03dfea8d-f17e-42a2-b044-bffdc6e22bc9\nIssued At: 2024-05-13T06:54:13.185Z\nExpiration Time: 2024-05-13T06:56:13.183Z"
	// SIWS message with version (0.0.18) and above
	message := "siws.xyz wants you to sign in with your Substrate account:\n12QPhT1S2hdUqv9b9RXMVHNizq7xhNnh5WG8Gudf6zJjPtwj\n\nWelcome to SIWS! Sign in to see how it works.\n\nURI: https://siws.xyz\nVersion: 1.0.0\nNonce: 11c4b0b8-4d9f-4341-b095-bdfc1c219182\nIssued At: 2024-05-13T06:58:43.959Z\nExpiration Time: 2024-05-13T07:00:43.958Z"

	// both cases work
	// substrate address
	// ss58address := "***REMOVED***"
	// polkadot address
	ss58address := "kGiayU6xGtpQKXDaJk3cjJgG3AphQeQ86AXcYnnNFFb8j9Ps5"

	isVerified, err := siws.SS58VerifySignature(message, ss58address, signatureFromFE)
	if err != nil {
		fmt.Println("Error verifying signature")
	}
	fmt.Println("Is Verified:", isVerified)

	// example on how to parse message
	_, err = siws.ParseMessage(message)
	if err != nil {
		fmt.Println("Error parsing message")
	}
}
