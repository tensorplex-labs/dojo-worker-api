package siws

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/btcsuite/btcutil/base58"
	"golang.org/x/crypto/blake2b"

	"github.com/ChainSafe/go-schnorrkel"
	"github.com/rs/zerolog/log"
)

const (
	SigningContext = "substrate"
	SS58Prefix     = "SS58PRE"
)

// replicated from polkadot's util-crypto
func sshash(key []byte) []byte {
	SS58Prefix := []byte("SS58PRE")
	concatenated := append(SS58Prefix, key...)
	hasher, _ := blake2b.New512(nil)
	hasher.Write(concatenated)
	return hasher.Sum(nil)
}

// replicated from polkadot's util-crypto
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

// replicated from polkadot util-crypto's decodeAddress function
func SS58AddressToPublickey(address string) ([]byte, error) {
	decoded := base58.Decode(address)
	allowedDecodedLengths := []int{3, 4, 6, 10, 35, 36, 37, 38}
	isValidLength := false
	for _, length := range allowedDecodedLengths {
		if len(decoded) == length {
			isValidLength = true
			break
		}
	}
	if !isValidLength {
		return nil, fmt.Errorf("invalid decoded address length %d, allowed decoded lengths %v", len(decoded), allowedDecodedLengths)
	}

	isValid, endPos, ss58Length, pubkey := checkAddressChecksum(decoded)
	log.Info().Msgf("Checksum is valid: %v, endPos: %d, SS58 Length: %d, SS58 Decoded: %d", isValid, endPos, ss58Length, pubkey)
	pubkeyHex := decoded[ss58Length:endPos]
	log.Info().Msgf("Derived Public key %x from address %s", pubkeyHex, address)
	return pubkeyHex, nil
}

// SS58VerifySignature verifies a signature using the go-schnorrkel library,
// given a message, ss58 address and signature from frontend generated using polkadot.js
// This is meant to be used together with libraries like SIWS (https://github.com/TalismanSociety/siws)
func SS58VerifySignature(siwsMessage, ss58address, signatureFromFrontend string) (bool, error) {
	pubkeyBytes, err := SS58AddressToPublickey(ss58address)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to derive public key from address: %s", ss58address)
		return false, err
	}

	pubkeyStr := "0x" + strings.TrimPrefix(hex.EncodeToString(pubkeyBytes), "0x")
	publicKey, err := schnorrkel.NewPublicKeyFromHex(pubkeyStr)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to create schnorrkel public key: %s", pubkeyStr)
		return false, err
	}

	if !strings.HasPrefix(signatureFromFrontend, "0x") {
		return false, fmt.Errorf("signature from frontend does not start with '0x'")
	}

	signature, err := schnorrkel.NewSignatureFromHex(signatureFromFrontend)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to create schnorrkel signature: %s", signatureFromFrontend)
		return false, err
	}

	// this is the context polkadot.js uses
	ctx := []byte(SigningContext)
	// frontend messages are always wrapped in <Bytes>...</Bytes> when calling signRaw in polkadot.js
	messageWrapped := "<Bytes>" + siwsMessage + "</Bytes>"
	transcript := schnorrkel.NewSigningContext(ctx, []byte(messageWrapped))

	ok, err := publicKey.Verify(signature, transcript)
	if err != nil {
		log.Error().Err(err).Msg("Failed to verify signature using schnorrkel")
		return false, err
	}
	log.Info().Msgf("Signature verification result: %v", ok)
	return ok, nil
}

// UNUSED for now, can't remember which was working
//
//lint:ignore U1000 Ignore unused function warning
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

// UNUSED for now, can't remember which was working
//
//lint:ignore U1000 Ignore unused function warning
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
