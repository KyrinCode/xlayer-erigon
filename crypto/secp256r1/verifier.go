package secp256r1

import (
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"
	"sync"
)

var (
	verifyCache sync.Map
)

func makeCacheKey(hash []byte, r, s, x, y *big.Int) string {
	hashStr := hex.EncodeToString(hash)
	rStr := r.Text(16)
	sStr := s.Text(16)
	xStr := x.Text(16)
	yStr := y.Text(16)
	return hashStr + "_" + rStr + "_" + sStr + "_" + xStr + "_" + yStr
}

// Verifies the given signature (r, s) for the given hash and public key (x, y).
func Verify(hash []byte, r, s, x, y *big.Int) bool {

	key := makeCacheKey(hash, r, s, x, y)
	if cachedResult, ok := verifyCache.Load(key); ok {
		return cachedResult.(bool)
	}

	// Create the public key format
	publicKey := newPublicKey(x, y)

	// Check if they are invalid public key coordinates
	if publicKey == nil {
		verifyCache.Store(key, false)
		return false
	}

	// Verify the signature with the public key,
	// then return true if it's valid, false otherwise
	isValid := ecdsa.Verify(publicKey, hash, r, s)
	verifyCache.Store(key, isValid)

	return isValid
}
