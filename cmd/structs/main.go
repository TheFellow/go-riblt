// Command structs reconciles complete, fixed-width application records.
package main

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log"
	"sort"

	"github.com/TheFellow/go-riblt"
)

const recordWidth = 64

// Record is the value carried by RIBLT. Fixed-width fields make every bit of
// the record part of a reversible XOR group.
type Record struct {
	ID       uint64
	Revision uint64
	Owner    [16]byte
	Digest   [32]byte
}

func newRecord(id, revision uint64, owner, body string) Record {
	r := Record{ID: id, Revision: revision, Digest: sha256.Sum256([]byte(body))}
	copy(r.Owner[:], owner)
	return r
}

// recordCodec adapts a domain struct to riblt.Codec. It delegates keyed,
// domain-separated hashing to the built-in fixed-width byte codec.
type recordCodec struct {
	bytes riblt.BytesCodec
	id    [32]byte
}

func newRecordCodec(key []byte) (recordCodec, error) {
	c, err := riblt.NewBytesCodec(recordWidth, key)
	if err != nil {
		return recordCodec{}, err
	}
	bytesID := c.CompatibilityID()
	idInput := append([]byte("record-codec/v1\x00"), bytesID[:]...)
	return recordCodec{bytes: c, id: sha256.Sum256(idInput)}, nil
}

func (recordCodec) Zero() Record                { return Record{} }
func (recordCodec) IsZero(r Record) bool        { return r == Record{} }
func (recordCodec) Clone(r Record) Record       { return r }
func (recordCodec) Equal(a, b Record) bool      { return a == b }
func (recordCodec) Validate(Record) error       { return nil }
func (c recordCodec) CompatibilityID() [32]byte { return c.id }
func (recordCodec) XOR(a, b Record) Record {
	r := Record{ID: a.ID ^ b.ID, Revision: a.Revision ^ b.Revision}
	for i := range r.Owner {
		r.Owner[i] = a.Owner[i] ^ b.Owner[i]
	}
	for i := range r.Digest {
		r.Digest[i] = a.Digest[i] ^ b.Digest[i]
	}
	return r
}
func (c recordCodec) MappingHash(r Record) uint64 { return c.bytes.MappingHash(marshal(r)) }
func (c recordCodec) Checksum(r Record) uint64    { return c.bytes.Checksum(marshal(r)) }

func marshal(r Record) []byte {
	b := make([]byte, recordWidth)
	binary.LittleEndian.PutUint64(b[0:8], r.ID)
	binary.LittleEndian.PutUint64(b[8:16], r.Revision)
	copy(b[16:32], r.Owner[:])
	copy(b[32:64], r.Digest[:])
	return b
}

func main() {
	key := []byte("example-only-key-change-in-production")
	codec, err := newRecordCodec(key)
	if err != nil {
		log.Fatal(err)
	}

	upstream := []Record{
		newRecord(1, 1, "Ada", "shared"),
		newRecord(2, 3, "Grace", "new upstream value"),
		newRecord(3, 1, "Linus", "upstream only"),
	}
	downstream := []Record{
		newRecord(1, 1, "Ada", "shared"),
		newRecord(2, 2, "Grace", "old downstream value"),
	}

	add, remove, cells, err := reconcile(codec, upstream, downstream)
	if err != nil {
		log.Fatal(err)
	}
	sort.Slice(add, func(i, j int) bool { return add[i].ID < add[j].ID })
	sort.Slice(remove, func(i, j int) bool { return remove[i].ID < remove[j].ID })

	fmt.Println("add or replace from upstream:")
	for _, r := range add {
		fmt.Printf("  id=%d revision=%d owner=%q digest=%x\n", r.ID, r.Revision, r.Owner[:ownerLen(r.Owner)], r.Digest[:4])
	}
	fmt.Println("remove downstream versions:")
	for _, r := range remove {
		fmt.Printf("  id=%d revision=%d owner=%q digest=%x\n", r.ID, r.Revision, r.Owner[:ownerLen(r.Owner)], r.Digest[:4])
	}
	fmt.Printf("decoded after %d coded cells\n", cells)
}

func ownerLen(owner [16]byte) int {
	for i, b := range owner {
		if b == 0 {
			return i
		}
	}
	return len(owner)
}

func reconcile(codec recordCodec, upstream, downstream []Record) ([]Record, []Record, int, error) {
	enc, err := riblt.NewEncoder[Record](codec)
	if err != nil {
		return nil, nil, 0, err
	}
	dec, err := riblt.NewDecoder[Record](codec)
	if err != nil {
		return nil, nil, 0, err
	}
	for _, r := range upstream {
		if err := enc.Add(r); err != nil {
			return nil, nil, 0, err
		}
	}
	for _, r := range downstream {
		if err := dec.AddLocal(r); err != nil {
			return nil, nil, 0, err
		}
	}
	for cells := 1; cells <= 100; cells++ {
		coded, err := enc.Next()
		if err != nil {
			return nil, nil, 0, err
		}
		if err := dec.AddCoded(coded); err != nil {
			return nil, nil, 0, err
		}
		if err := dec.TryDecode(); err != nil {
			return nil, nil, 0, err
		}
		if dec.Complete() {
			return records(dec.Remote()), records(dec.Local()), cells, nil
		}
	}
	return nil, nil, 0, fmt.Errorf("decode did not complete")
}

func records(in []riblt.HashedSymbol[Record]) []Record {
	out := make([]Record, len(in))
	for i := range in {
		out[i] = in[i].Symbol
	}
	return out
}
