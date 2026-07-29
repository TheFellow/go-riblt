// Package demo contains a deliberately small symbol and codec used by the
// executable examples. Applications should define these around their own IDs.
package demo

import (
	"fmt"

	"github.com/TheFellow/go-riblt"
)

// Symbol is an application-level value. It is comparable, but riblt does not
// require comparability: all algebra and hashing are supplied by Codec.
type Symbol struct {
	ID uint64
}

func (s Symbol) String() string { return fmt.Sprintf("item-%d", s.ID) }

// Codec makes Symbol an XOR group and domain-separates placement from
// singleton validation. For untrusted peers, replace Checksum with a keyed
// hash and authenticate the protocol messages.
type Codec struct{}

func (Codec) Zero() Symbol              { return Symbol{} }
func (Codec) IsZero(s Symbol) bool      { return s == (Symbol{}) }
func (Codec) Clone(s Symbol) Symbol     { return s }
func (Codec) Equal(a, b Symbol) bool    { return a == b }
func (Codec) Validate(Symbol) error     { return nil }
func (Codec) CompatibilityID() [32]byte { return [32]byte{1} }

func (Codec) XOR(a, b Symbol) Symbol { return Symbol{ID: a.ID ^ b.ID} }

func (Codec) MappingHash(s Symbol) uint64 { return mix(s.ID ^ 0x6d617070696e6721) }

func (Codec) Checksum(s Symbol) uint64 { return mix(s.ID ^ 0x636865636b73756d) }

func mix(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

func NewEncoder(ids ...uint64) *riblt.Encoder[Symbol] {
	e, err := riblt.NewEncoder[Symbol](Codec{})
	if err != nil {
		panic(err)
	}
	for _, id := range ids {
		if err := e.Add(Symbol{ID: id}); err != nil {
			panic(err)
		}
	}
	return e
}

func NewDecoder(ids ...uint64) *riblt.Decoder[Symbol] {
	d, err := riblt.NewDecoder[Symbol](Codec{})
	if err != nil {
		panic(err)
	}
	for _, id := range ids {
		if err := d.AddLocal(Symbol{ID: id}); err != nil {
			panic(err)
		}
	}
	return d
}

func IDs(values []riblt.HashedSymbol[Symbol]) []uint64 {
	result := make([]uint64, len(values))
	for i, value := range values {
		result[i] = value.Symbol.ID
	}
	return result
}
