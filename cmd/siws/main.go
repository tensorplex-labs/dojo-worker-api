package main

import (
	"dojo-api/pkg/blockchain/siws"
	"fmt"
)

func main() {
	// signatureFromFE := "***REMOVED***"
	// message := "siws.xyz wants you to sign in with your Bittensor account:\n***REMOVED***\n\nWelcome to SIWS! Sign in to see how it works.\n\nURI: https://siws.xyz\nNonce: 3333dae4-57de-40d7-873d-3b81a0847ad5\nIssued At: 2024-05-10T16:36:48.530Z\nExpiration Time: 2024-05-10T16:38:48.530Z"
	signatureFromFE := "0x3ec017c1107815bd2bf01725b65406b7e2beda496ae6c4ba0d5602a70dc01368fe744f6810f75d40b66ecf123bb044dfb2608d6006aa5233551e12dbc2743c8a"
	message := "siws.xyz wants you to sign in with your Polkadot account:\n12QPhT1S2hdUqv9b9RXMVHNizq7xhNnh5WG8Gudf6zJjPtwj\n\nWelcome to SIWS! Sign in to see how it works.\n\nURI: https://siws.xyz\nNonce: 833a9f02-14d3-4fe9-a0d3-d93db5aedb5a\nIssued At: 2024-05-12T07:46:36.931Z\nExpiration Time: 2024-05-12T07:48:36.931Z"

	// both cases work
	// substrate address
	// ss58address := "***REMOVED***"
	// polkadot address
	ss58address := "12QPhT1S2hdUqv9b9RXMVHNizq7xhNnh5WG8Gudf6zJjPtwj"
	isVerified, err := siws.SS58VerifySignature(message, ss58address, signatureFromFE)
	if err != nil {
		fmt.Println("Error verifying signature")
	}
	fmt.Println("Is Verified:", isVerified)

	// example on how to parse message
	parsed, err := siws.ParseMessage(message)
	if err != nil {
		fmt.Println("Error parsing message")
	}
	fmt.Println("Parsed Message:", parsed)
}
