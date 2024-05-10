package main

import (
	"encoding/hex"
	"fmt"

	"github.com/ChainSafe/go-schnorrkel"
)

func main() {
	msg := []byte("some_guy")
	// signatureHex := "0xd6df2fc47361e8d745f8c00e9470677da7d72b7fa9d6a036b3ec725c010feb356e88ac02ecb9ec379f8bb5f9c3d0363ecf80adb2f3f696bd40f61e5c7d16f586"

	signatureHex, pub, _ :=  generateSignature()
	signingContext := []byte("substrate")
	verifyTranscript := schnorrkel.NewSigningContext(signingContext, msg)

	signatureBytes, err := hex.DecodeString(signatureHex[2:])
	if err != nil {
		fmt.Printf("error decoding signature: %s\n", err)
		return
	}

	var signature [64]byte
	copy(signature[:], signatureBytes)

	sig := &schnorrkel.Signature{}
	err = sig.Decode(signature)
	if err != nil {
		fmt.Printf("error decoding signature: %s\n", err)
		return
	}

	// Assuming publicKeyBytes is the public key in bytes that you have
	// For demonstration, this part needs the actual public key bytes to work
	// _, pub, err := schnorrkel.GenerateKeypair()
	// if err != nil {
	// 	panic(err)
	// }

	ok, err := pub.Verify(sig, verifyTranscript)
	if err != nil {
		fmt.Printf("error verifying signature: %s\n", err)
		return
	}

	if !ok {
		fmt.Println("failed to verify signature")
		return
	}

	fmt.Println("signature verified successfully")
}

func generateSignature() (string, *schnorrkel.PublicKey, error) {
	message := []byte("some_guy")
	signingContext := []byte("substrate")
	signingTranscript := schnorrkel.NewSigningContext(signingContext, message)

	priv, pub, err := schnorrkel.GenerateKeypair()
	if err != nil {
		return "", pub, fmt.Errorf("error generating keypair: %s", err)
	}

	// Sign the message
	signature, err := priv.Sign(signingTranscript)
	if err != nil {
		return "", pub, fmt.Errorf("error signing message: %s", err)
	}

	// Encode the signature to a slice of bytes
	sigBytes := signature.Encode()
	// Convert the signature bytes to a hex string
	signatureHex := hex.EncodeToString(sigBytes[:])
	signatureHex = "0x" + signatureHex

	return signatureHex, pub, nil
}
