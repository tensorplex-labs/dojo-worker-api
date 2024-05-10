package main 

import (
	"fmt"
	
	schnorrkel "github.com/ChainSafe/go-schnorrkel"
)

func main() {
	msg := []byte("hello friends")
	signingCtx := []byte("example")

	signingTranscript := schnorrkel.NewSigningContext(signingCtx, msg)
	verifyTranscript := schnorrkel.NewSigningContext(signingCtx, msg)

	priv, pub, err := schnorrkel.GenerateKeypair()
	if err != nil {
		panic(err)
	}

	sig, err := priv.Sign(signingTranscript)
	if err != nil {
		panic(err)
	}

	ok, _ := pub.Verify(sig, verifyTranscript)
	if !ok {
		fmt.Println("failed to verify signature")
		return
	}

	fmt.Println("verified signature")
}