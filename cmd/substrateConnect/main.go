package main

import (
	"encoding/hex"
	"fmt"

	schnorrkel "github.com/ChainSafe/go-schnorrkel"
)

func main() {
	msg := []byte("siws.xyz wants you to sign in with your Polkadot account: 13SjBvGAYBmcEFfqqWzYzyHWFg23jgEMExurbFPK2bDjSNFQ Welcome to SIWS! Sign in to see how it works. URI: https://siws.xyz Nonce: 8b79baae-002d-4b0a-a0aa-694dbcf93728 Issued At: 2024-05-09T11:58:04.397Z Expiration Time: 2024-05-09T12:00:04.394Z")
	signingCtx := []byte("example")
	signatureHex := "0x66545c10145c183d3dbca3f44ce682a3bbc2b07ba1f2016ec1279c244a3eb628b0225f06923e997f5e4271f581f2e0b68e8be5e7a8a28a8cc6719dd70b87df8d"

	signatureBytes, err := hex.DecodeString(signatureHex[2:])
	if err != nil {
		fmt.Println("error decoding signature:", err)
		return
	}

	var signature [64]byte
	copy(signature[:], signatureBytes[:64])

	signatureObj := &schnorrkel.Signature{}
	err = signatureObj.Decode(signature)
	if err != nil {
		fmt.Println("error decoding signature:", err)
		return
	}

	verifyTranscript := schnorrkel.NewSigningContext(signingCtx, msg)

	_, pub, err := schnorrkel.GenerateKeypair()
	if err != nil {
		panic(err)
	}

	ok, err := pub.Verify(signatureObj, verifyTranscript)
	if err != nil {
		fmt.Println("error verifying signature:", err)
		return
	}
	if !ok {
		fmt.Println("failed to verify signature")
		return
	}

	fmt.Println("verified signature")
}