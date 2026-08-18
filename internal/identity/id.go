package identity

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

type Generator interface {
	NewID(prefix string) string
}

type Random struct{}

func (Random) NewID(prefix string) string {
	var data [12]byte
	if _, err := rand.Read(data[:]); err != nil {
		panic(fmt.Sprintf("generate id: %v", err))
	}
	return prefix + "_" + hex.EncodeToString(data[:])
}

type Sequence struct{ Next int }

func (s *Sequence) NewID(prefix string) string {
	s.Next++
	return fmt.Sprintf("%s_%06d", prefix, s.Next)
}
