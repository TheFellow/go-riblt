package riblt

// Encoder incrementally produces a set's infinite RIBLT coded-symbol stream.
type Encoder[T any] struct {
	codec   Codec[T]
	window  codingWindow[T]
	seen    map[identity]struct{}
	started bool
}

func NewEncoder[T any](codec Codec[T]) (*Encoder[T], error) {
	if codec == nil {
		return nil, ErrNilCodec
	}
	e := &Encoder[T]{codec: codec, seen: make(map[identity]struct{})}
	e.window.codec = codec
	return e, nil
}

func (e *Encoder[T]) Add(value T) error { return e.AddHashed(Hash(e.codec, value)) }
func (e *Encoder[T]) AddHashed(s HashedSymbol[T]) error {
	if e.started {
		return ErrEncoderStarted
	}
	id := identity{s.MappingHash, s.Checksum}
	if _, ok := e.seen[id]; ok {
		return ErrDuplicate
	}
	e.seen[id] = struct{}{}
	e.window.add(s, newMapping(s.MappingHash))
	return nil
}
func (e *Encoder[T]) Next() (CodedSymbol[T], error) {
	e.started = true
	return e.window.apply(zeroCoded(e.codec), 1)
}
func (e *Encoder[T]) Reset() { e.window.reset(); clear(e.seen); e.started = false }
