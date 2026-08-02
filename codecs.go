package riblt

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
)

const (
	mappingDomain  = "go-riblt/v1/mapping\x00"
	checksumDomain = "go-riblt/v1/checksum\x00"
)

type secureHasher struct {
	key []byte
	id  [32]byte
}

func (h secureHasher) validateConfiguration() error {
	if len(h.key) < 16 {
		return ErrWeakKey
	}
	return nil
}

func newSecureHasher(kind string, key []byte, parameter []byte) (secureHasher, error) {
	if len(key) < 16 {
		return secureHasher{}, ErrWeakKey
	}
	k := append([]byte(nil), key...)
	h := sha256.New()
	h.Write([]byte(ProtocolVersion))
	h.Write([]byte{0})
	h.Write([]byte(kind))
	h.Write([]byte{0})
	h.Write(parameter)
	h.Write([]byte{0})
	h.Write(k) // digest binds configuration without exposing the key
	var id [32]byte
	copy(id[:], h.Sum(nil))
	return secureHasher{key: k, id: id}, nil
}

func (h secureHasher) sum(domain string, value []byte) uint64 {
	m := hmac.New(sha256.New, h.key)
	m.Write([]byte(domain))
	m.Write(value)
	return binary.LittleEndian.Uint64(m.Sum(nil))
}

// Uint64Codec is a keyed, domain-separated codec for uint64 symbols.
type Uint64Codec struct{ h secureHasher }

// NewUint64Codec constructs a codec using key. Keys shorter than 16 bytes are
// rejected, and the returned zero codec must not be used when err is non-nil.
func NewUint64Codec(key []byte) (Uint64Codec, error) {
	h, err := newSecureHasher("uint64-le", key, nil)
	if err != nil {
		return Uint64Codec{}, err
	}
	return Uint64Codec{h}, nil
}
func (c Uint64Codec) validateConfiguration() error { return c.h.validateConfiguration() }
func (c Uint64Codec) Zero() uint64                 { return 0 }
func (c Uint64Codec) IsZero(v uint64) bool         { return v == 0 }
func (c Uint64Codec) Clone(v uint64) uint64        { return v }
func (c Uint64Codec) Equal(a, b uint64) bool       { return a == b }
func (c Uint64Codec) Validate(uint64) error        { return nil }
func (c Uint64Codec) XOR(a, b uint64) uint64       { return a ^ b }
func (c Uint64Codec) CompatibilityID() [32]byte    { return c.h.id }
func (c Uint64Codec) MappingHash(v uint64) uint64  { return c.hash(mappingDomain, v) }
func (c Uint64Codec) Checksum(v uint64) uint64     { return c.hash(checksumDomain, v) }
func (c Uint64Codec) hash(domain string, v uint64) uint64 {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	return c.h.sum(domain, b[:])
}

// BytesCodec is a keyed codec for fixed-width byte strings. Fixed width is
// required because zero padding would make variable-length XOR non-reversible.
type BytesCodec struct {
	h     secureHasher
	width int
}

// NewBytesCodec constructs a fixed-width codec using key. Invalid widths and
// keys shorter than 16 bytes return an unusable zero codec and a non-nil error.
func NewBytesCodec(width int, key []byte) (BytesCodec, error) {
	if width <= 0 {
		return BytesCodec{}, ErrInvalidWidth
	}
	var p [8]byte
	binary.LittleEndian.PutUint64(p[:], uint64(width))
	h, err := newSecureHasher("fixed-bytes", key, p[:])
	if err != nil {
		return BytesCodec{}, err
	}
	return BytesCodec{h: h, width: width}, nil
}
func (c BytesCodec) validateConfiguration() error {
	if c.width <= 0 {
		return ErrInvalidWidth
	}
	return c.h.validateConfiguration()
}
func (c BytesCodec) Zero() []byte { return make([]byte, c.width) }
func (c BytesCodec) IsZero(v []byte) bool {
	if len(v) != c.width {
		return false
	}
	for _, b := range v {
		if b != 0 {
			return false
		}
	}
	return true
}
func (c BytesCodec) Clone(v []byte) []byte  { return append([]byte(nil), v...) }
func (c BytesCodec) Equal(a, b []byte) bool { return hmac.Equal(a, b) }
func (c BytesCodec) Validate(v []byte) error {
	if len(v) != c.width {
		return fmt.Errorf("%w: got %d, want %d", ErrInvalidSymbol, len(v), c.width)
	}
	return nil
}
func (c BytesCodec) XOR(a, b []byte) []byte {
	out := make([]byte, c.width)
	for i := range out {
		out[i] = a[i] ^ b[i]
	}
	return out
}
func (c BytesCodec) MappingHash(v []byte) uint64 { return c.h.sum(mappingDomain, v) }
func (c BytesCodec) Checksum(v []byte) uint64    { return c.h.sum(checksumDomain, v) }
func (c BytesCodec) CompatibilityID() [32]byte   { return c.h.id }
func (c BytesCodec) Width() int                  { return c.width }
