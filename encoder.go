package riblt

// Encoder incrementally produces a set's infinite RIBLT coded-symbol stream.
type Encoder[T any] struct {
	codec   Codec[T]
	window  codingWindow[T]
	seen    symbolSet[T]
	started bool
}

func NewEncoder[T any](codec Codec[T]) (*Encoder[T], error) {
	if err := validateCodec(codec); err != nil {
		return nil, err
	}
	e := &Encoder[T]{codec: codec, seen: newSymbolSet(codec)}
	e.window.codec = codec
	return e, nil
}

func (e *Encoder[T]) Add(value T) error {
	if e.started {
		return ErrEncoderStarted
	}
	if err := e.codec.Validate(value); err != nil {
		return err
	}
	s := hashSymbol(e.codec, e.codec.Clone(value))
	if !e.seen.add(s) {
		return ErrDuplicate
	}
	e.window.add(s, newMapping(s.MappingHash))
	return nil
}
func (e *Encoder[T]) Next() (CodedSymbol[T], error) {
	e.started = true
	return e.window.apply(zeroCoded(e.codec), 1)
}
func (e *Encoder[T]) Reset() { e.window.reset(); e.seen.reset(); e.started = false }
