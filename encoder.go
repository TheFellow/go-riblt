package riblt

import "iter"

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
func (e *Encoder[T]) next() (CodedSymbol[T], error) {
	e.started = true
	return e.window.apply(zeroCoded(e.codec), 1)
}

// Cells returns a lazy, unbounded sequence of coded symbols and their errors.
// Ranging begins at the encoder's current position and produces a cell only
// when the consumer requests one. Breaking from the range stops production; a
// later range over Cells resumes with the next cell.
//
// The sequence is a stateful view of e, not a replayable collection. Do not
// range it concurrently or mix it with concurrent calls to other Encoder
// methods. A non-nil error is yielded once, then that range stops.
func (e *Encoder[T]) Cells() iter.Seq2[CodedSymbol[T], error] {
	return func(yield func(CodedSymbol[T], error) bool) {
		for {
			cell, err := e.next()
			if !yield(cell, err) || err != nil {
				return
			}
		}
	}
}

func (e *Encoder[T]) Reset() { e.window.reset(); e.seen.reset(); e.started = false }
