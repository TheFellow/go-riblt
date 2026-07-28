package riblt

// Codec supplies the Boolean-group and hashing operations RIBLT needs.
//
// Zero returns the identity. XOR must be pure: it must not mutate or retain
// either argument and must return storage that does not alias them. IsZero
// reports whether a value is that identity; it prevents a malformed count-zero
// cell from masquerading as empty. MappingHash determines cell placement.
// Checksum validates decoded singleton cells; it must be independently
// domain-separated from MappingHash and should be keyed when inputs are not
// trusted.
type Codec[T any] interface {
	Zero() T
	IsZero(T) bool
	XOR(a, b T) T
	MappingHash(T) uint64
	Checksum(T) uint64
}

// HashedSymbol caches both hashes of a source symbol.
type HashedSymbol[T any] struct {
	Symbol      T
	MappingHash uint64
	Checksum    uint64
}

// Hash computes a value's placement and validation hashes.
func Hash[T any](codec Codec[T], value T) HashedSymbol[T] {
	return HashedSymbol[T]{value, codec.MappingHash(value), codec.Checksum(value)}
}

// CodedSymbol is one element of an encoder's rateless stream.
type CodedSymbol[T any] struct {
	Symbol   T
	Checksum uint64
	Count    int64
}

func zeroCoded[T any](codec Codec[T]) CodedSymbol[T] {
	return CodedSymbol[T]{Symbol: codec.Zero()}
}

func apply[T any](codec Codec[T], c CodedSymbol[T], s HashedSymbol[T], direction int64) CodedSymbol[T] {
	c.Symbol = codec.XOR(c.Symbol, s.Symbol)
	c.Checksum ^= s.Checksum
	c.Count += direction
	return c
}
