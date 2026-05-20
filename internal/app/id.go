package app

import (
	"crypto/rand"
	"encoding/hex"
)

type RandomIDGenerator struct{}

func (g RandomIDGenerator) NewID() string {
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return "cell-id-unavailable"
	}
	return hex.EncodeToString(data[:])
}
