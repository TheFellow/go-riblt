package riblt

import (
	"fmt"
	"reflect"
)

// ProtocolVersion identifies the coded-cell and mapping semantics implemented
// by this package. Put it in the surrounding wire protocol and reject peers
// that advertise a different value.
const ProtocolVersion = "go-riblt/v1"

// Codec supplies the value algebra, ownership, validation, hashing, and
// compatibility operations RIBLT needs.
//
// XOR must be a pure Boolean-group operation: x XOR zero == x and x XOR x ==
// zero. Clone must return storage independent of v. Equal compares values, not
// hashes. Validate rejects values outside the codec's algebra (for example a
// byte string of the wrong width). MappingHash and Checksum must be independent
// and deterministic. CompatibilityID must change whenever any of those
// semantics or their key changes.
type Codec[T any] interface {
	Zero() T
	IsZero(T) bool
	Clone(T) T
	Equal(a, b T) bool
	Validate(T) error
	XOR(a, b T) T
	MappingHash(T) uint64
	Checksum(T) uint64
	CompatibilityID() [32]byte
}

func validateCodec[T any](codec Codec[T]) error {
	if codec == nil || isNil(codec) {
		return ErrNilCodec
	}
	zero := codec.Zero()
	if err := codec.Validate(zero); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidCodec, err)
	}
	if !codec.IsZero(zero) {
		return ErrInvalidCodec
	}
	return nil
}

func isNil(v any) bool {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

// HashedSymbol is a decoded symbol together with its placement and validation
// hashes. Callers receive these values as results; insertion APIs deliberately
// recompute hashes so untrusted cached hashes cannot corrupt a sketch.
type HashedSymbol[T any] struct {
	Symbol      T
	MappingHash uint64
	Checksum    uint64
}

func hashSymbol[T any](codec Codec[T], value T) HashedSymbol[T] {
	return HashedSymbol[T]{value, codec.MappingHash(value), codec.Checksum(value)}
}

// Hash reports a symbol's hashes for diagnostics. Insertion APIs deliberately
// do not accept prehashed values: they validate, clone, and hash at their trust
// boundary.
func Hash[T any](codec Codec[T], value T) (HashedSymbol[T], error) {
	if err := validateCodec(codec); err != nil {
		return HashedSymbol[T]{}, err
	}
	if err := codec.Validate(value); err != nil {
		return HashedSymbol[T]{}, err
	}
	return hashSymbol(codec, codec.Clone(value)), nil
}

// CodedSymbol is one element of an encoder's rateless stream. Its fields are
// suitable for application-defined serialization; this package intentionally
// does not define a wire format.
type CodedSymbol[T any] struct {
	Symbol   T
	Checksum uint64
	Count    int64
}

func zeroCoded[T any](codec Codec[T]) CodedSymbol[T] {
	return CodedSymbol[T]{Symbol: codec.Clone(codec.Zero())}
}

func apply[T any](codec Codec[T], c CodedSymbol[T], s HashedSymbol[T], direction int64) CodedSymbol[T] {
	c.Symbol = codec.XOR(c.Symbol, s.Symbol)
	c.Checksum ^= s.Checksum
	c.Count += direction
	return c
}

func cloneHashed[T any](codec Codec[T], s HashedSymbol[T]) HashedSymbol[T] {
	s.Symbol = codec.Clone(s.Symbol)
	return s
}

func cloneCoded[T any](codec Codec[T], c CodedSymbol[T]) CodedSymbol[T] {
	c.Symbol = codec.Clone(c.Symbol)
	return c
}
