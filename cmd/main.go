package main

import (
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcutil/base58"
	"golang.org/x/crypto/blake2b"

	"github.com/ChainSafe/go-schnorrkel"
	"github.com/ChainSafe/gossamer/lib/crypto/sr25519"
)

func sshash(key []byte) []byte {
	SS58Prefix := []byte("SS58PRE")
	concatenated := append(SS58Prefix, key...)
	hasher, _ := blake2b.New512(nil)
	hasher.Write(concatenated)
	return hasher.Sum(nil)
}

func checkAddressChecksum(decoded []byte) (bool, int, int, int) {

	ss58Length := 1
	if decoded[0]&0b0100_0000 != 0 {
		ss58Length = 2
	}
	ss58Decoded := int(decoded[0])
	if ss58Length != 1 {
		ss58Decoded = (int(decoded[0]&0b0011_1111) << 2) | (int(decoded[1] >> 6)) | (int(decoded[1]&0b0011_1111) << 8)
	}

	// 32/33 bytes public + 2 bytes checksum + prefix
	isPublicKey := len(decoded) == 34+ss58Length || len(decoded) == 35+ss58Length
	length := len(decoded) - 2
	if !isPublicKey {
		length++
	}

	// calculate the hash and do the checksum byte checks
	hash := sshash(decoded[:length])
	isValid := decoded[0]&0b1000_0000 == 0 && decoded[0] != 46 && decoded[0] != 47 && (isPublicKey && decoded[len(decoded)-2] == hash[0] && decoded[len(decoded)-1] == hash[1] || !isPublicKey && decoded[len(decoded)-1] == hash[0])

	return isValid, length, ss58Length, ss58Decoded
}

func publicKeyToPolkadotAddress(pubKeyBytes []byte, networkPrefix int) (string, error) {
	// Prepend the network prefix
	prefixedKey := append([]byte{byte(networkPrefix)}, pubKeyBytes...)

	// Create a blake2b hash of the prefixed public key
	hasher, err := blake2b.New(64, nil) // Create a hasher
	if err != nil {
		return "", err
	}
	hasher.Write(prefixedKey) // Write data to hasher
	hash := hasher.Sum(nil)   // Get hash

	// Take the first two bytes of the hash as the checksum
	checksum := hash[:2]

	// Append the checksum to the prefixed public key
	fullAddress := append(prefixedKey, checksum...)

	// Encode the result using Base58
	address := base58.Encode(fullAddress)
	return address, nil
}

// returns public key
func ss58AddressToPubkey(address string) []byte {
	//   allowedEncodedLengths: [3, 4, 6, 10, 35, 36, 37, 38],
	decoded := base58.Decode(address)
	fmt.Println("Decoded length", len(decoded))
	isValid, endPos, ss58Length, pubkey := checkAddressChecksum(decoded)
	fmt.Printf("Is valid: %v, endPos: %d, SS58 Length: %d, SS58 Decoded: %d\n", isValid, endPos, ss58Length, pubkey)
	fmt.Printf("Public key %v\n", hex.EncodeToString(decoded[ss58Length:endPos]))
	return decoded[ss58Length:endPos]
}

const signatureFromFE = "***REMOVED***"
const message = "siws.xyz wants you to sign in with your Bittensor account:\n***REMOVED***\n\nWelcome to SIWS! Sign in to see how it works.\n\nURI: https://siws.xyz\nNonce: 3333dae4-57de-40d7-873d-3b81a0847ad5\nIssued At: 2024-05-10T16:36:48.530Z\nExpiration Time: 2024-05-10T16:38:48.530Z"

func pubkeyToPolkadot(chainId int, pubkeyBytes []byte) (string, error) {
	var input []byte
	if chainId < 64 {
		input = append(input, byte(chainId))
	} else {
		input = append(input, byte(((chainId&0b0000_0000_1111_1100)>>2)|0b0100_0000))
		input = append(input, byte((chainId>>8)|((chainId&0b0000_0000_0000_0011)<<6)))
	}
	input = append(input, pubkeyBytes...)

	var checksumLength int
	if len(pubkeyBytes) == 32 || len(pubkeyBytes) == 33 {
		checksumLength = 2
	} else {
		checksumLength = 1
	}

	hash := sshash(input)
	input = append(input, hash[:checksumLength]...)

	address := base58.Encode(input)
	return address, nil
}

func main() {
	ss58address := "***REMOVED***"
	pubkeyBytes := ss58AddressToPubkey(ss58address)

	// remove 0x prefix
	sigBytes, err := hex.DecodeString(signatureFromFE[2:])
	if err != nil {
		fmt.Println("Failed to decode signature, err:", err)
		return
	}
	fmt.Println("Signature length:", len(sigBytes))
	pk, err := sr25519.NewPublicKey(pubkeyBytes)
	if err != nil {
		fmt.Println("Failed to create public key, err:", err)
		return
	}

	// hash message
	hasher, _ := blake2b.New512(nil)
	hasher.Write([]byte("siws.xyz wants you to sign in with your bittensor account:\n5du6z7knavn1qp95bnumm8ya9d8k15ez11xe7cejyuhddfkm\n\nwelcome to siws! sign in to see how it works.\n\nuri: https://siws.xyz\nnonce: 3333dae4-57de-40d7-873d-3b81a0847ad5\nissued at: 2024-05-10t16:36:48.530z\nexpiration time: 2024-05-10t16:38:48.530z"))
	messagehash := hasher.Sum(nil)

	isVerified, err := pk.Verify(messagehash, sigBytes)
	fmt.Println("Is Verified:", isVerified)
	fmt.Println("Err", err)

	// TRYING FROM GITHUB GIST

	pubkeyString := "0x" + hex.EncodeToString(pubkeyBytes)
	sPk, err := schnorrkel.NewPublicKeyFromHex(pubkeyString)
	if err != nil {
		fmt.Println("Failed to create public key, err:", err)
		return
	}
	sig, err := schnorrkel.NewSignatureFromHex(signatureFromFE)
	if err != nil {
		fmt.Println("Failed to create signature, err:", err)
		return
	}
	// this is the ctx polkadot-js uses
	ctx := []byte("substrate")
	// process signature
	messageWrapped := "<Bytes>" + message + "</Bytes>"
	transcript := schnorrkel.NewSigningContext(ctx, []byte(messageWrapped))

	ok, err := sPk.Verify(sig, transcript)
	if err != nil {
		fmt.Println("Failed to verify signature, err:", err)
		return
	}
	fmt.Println("Is Verified:", ok)

	// // msgBytes, _ := hex.DecodeString(message)
	// err = sr25519.VerifySignature(pubkeyBytes, sigBytes, []byte(message))

	// if err != nil {
	// 	fmt.Println("Failed to verify signature, err:", err)
	// 	fmt.Println("Failed to verify signature, err:", err)
	// 	fmt.Println("Failed to verify signature, err:", err)
	// 	fmt.Println("Failed to verify signature, err:", err)
	// }

	// kp, err := sr25519.GenerateKeypair()
	// sig, _ := kp.Sign([]byte(message))
	// fmt.Println("Signature with just message:", hex.EncodeToString(sig))
	// // sig2, _ := kp.Sign(msgBytes)
	// // fmt.Println("Signature with structured message:", hex.EncodeToString(sig2))
	// fmt.Println("Length in bytes", len(sig))
	// // testVerified, err := kp.Public().Verify([]byte(message), sig)
	// err = sr25519.VerifySignature(kp.Public().Encode(), sig, []byte(message))
	// fmt.Println("Is Verified:", err == nil)
	// fmt.Println("Err", err)
}
