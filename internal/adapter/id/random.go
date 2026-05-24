package id

import (
	"crypto/rand"
	"encoding/hex"
)

type RandomGenerator struct{}

func (g RandomGenerator) NewID() string {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "cell-id-unavailable"
	}
	return hex.EncodeToString(data[:])
}
