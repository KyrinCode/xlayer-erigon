package secp256r1

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"
	"testing"
)

func BenchmarkVerify(b *testing.B) {
	N := b.N
	hashs := make([][]byte, N)
	pubKeys := make([]ecdsa.PublicKey, N)
	rs := make([]*big.Int, N)
	ss := make([]*big.Int, N)
	b.StopTimer()
	for i := 0; i < N; i++ {
		privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			panic(err)
		}

		pubKey := &privKey.PublicKey
		pubKeys[i] = *pubKey
		// 2. Message to sign
		message := fmt.Sprintf("hello world %d", i)
		hash := sha256.Sum256([]byte(message))
		hashs[i] = hash[:]
		// 3. Sign the hashed message
		r, s, err := ecdsa.Sign(rand.Reader, privKey, hash[:])
		if err != nil {
			panic(err)
		}
		rs[i] = r
		ss[i] = s
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		// 4. Verify the signature
		valid := ecdsa.Verify(&pubKeys[i], hashs[i], rs[i], ss[i])
		if !valid {
			b.Fatal("verify error")
		}
	}
}
