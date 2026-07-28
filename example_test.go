package riblt_test

import (
	"fmt"
	"github.com/TheFellow/go-riblt"
)

type codec struct{}

func (codec) Zero() uint64           { return 0 }
func (codec) IsZero(v uint64) bool   { return v == 0 }
func (codec) XOR(a, b uint64) uint64 { return a ^ b }
func hash(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}
func (codec) MappingHash(x uint64) uint64 { return hash(x ^ 1) }
func (codec) Checksum(x uint64) uint64    { return hash(x ^ 2) }

func Example() {
	alice := []uint64{1, 2, 3, 4, 5}
	bob := []uint64{1, 3, 4, 5, 11}
	enc, _ := riblt.NewEncoder[uint64](codec{})
	dec, _ := riblt.NewDecoder[uint64](codec{})
	for _, v := range alice {
		_ = enc.Add(v)
	}
	for _, v := range bob {
		_ = dec.AddLocal(v)
	}
	sent := 0
	for !dec.Complete() {
		coded, _ := enc.Next()
		sent++
		_ = dec.AddCoded(coded)
		_ = dec.TryDecode()
	}
	fmt.Println(dec.Remote()[0].Symbol, "only Alice has")
	fmt.Println(dec.Local()[0].Symbol, "only Bob has")
	fmt.Println(sent, "coded symbols")
	// Output:
	// 2 only Alice has
	// 11 only Bob has
	// 3 coded symbols
}
