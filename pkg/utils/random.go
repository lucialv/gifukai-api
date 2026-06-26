package utils

import (
	"crypto/rand"
	"encoding/hex"
)

// RandomHex returns n random bytes hex-encoded :3
func RandomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
