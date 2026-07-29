package riblt

// Sketch is a mutable fixed-length prefix, useful when its size is known.
type Sketch[T any] struct {
	codec         Codec[T]
	cells         []CodedSymbol[T]
	seen          symbolSet[T]
	compatibility [32]byte
}

func NewSketch[T any](codec Codec[T], length int) (*Sketch[T], error) {
	if err := validateCodec(codec); err != nil {
		return nil, err
	}
	if length < 0 {
		return nil, ErrDifferentLength
	}
	s := &Sketch[T]{codec: codec, cells: make([]CodedSymbol[T], length), seen: newSymbolSet(codec), compatibility: codec.CompatibilityID()}
	for i := range s.cells {
		s.cells[i] = zeroCoded(codec)
	}
	return s, nil
}
func (s *Sketch[T]) Cells() []CodedSymbol[T] {
	out := make([]CodedSymbol[T], len(s.cells))
	for i := range s.cells {
		out[i] = cloneCoded(s.codec, s.cells[i])
	}
	return out
}
func (s *Sketch[T]) Add(value T) error {
	if err := s.codec.Validate(value); err != nil {
		return err
	}
	v := hashSymbol(s.codec, s.codec.Clone(value))
	if !s.seen.add(v) {
		return ErrDuplicate
	}
	if err := s.update(v, 1); err != nil {
		s.seen.remove(v)
		return err
	}
	return nil
}
func (s *Sketch[T]) Remove(v T) error {
	if err := s.codec.Validate(v); err != nil {
		return err
	}
	h := hashSymbol(s.codec, v)
	if !s.seen.remove(h) {
		return ErrNotPresent
	}
	if err := s.update(h, -1); err != nil {
		s.seen.add(h)
		return err
	}
	return nil
}
func (s *Sketch[T]) update(v HashedSymbol[T], direction int64) error {
	m := newMapping(v.MappingHash)
	for m.last < uint64(len(s.cells)) {
		if direction > 0 && s.cells[m.last].Count == int64(^uint64(0)>>1) || direction < 0 && s.cells[m.last].Count == -int64(^uint64(0)>>1)-1 {
			return ErrCountOverflow
		}
		s.cells[m.last] = apply(s.codec, s.cells[m.last], v, direction)
		if _, err := m.next(); err != nil {
			return err
		}
	}
	return nil
}
func (s *Sketch[T]) Subtract(other *Sketch[T]) error {
	if other == nil || s.compatibility != other.compatibility || s.codec.CompatibilityID() != s.compatibility || other.codec.CompatibilityID() != other.compatibility {
		return ErrIncompatible
	}
	if len(s.cells) != len(other.cells) {
		return ErrDifferentLength
	}
	for i := range s.cells {
		if other.cells[i].Count > 0 && s.cells[i].Count < -int64(^uint64(0)>>1)-1+other.cells[i].Count || other.cells[i].Count < 0 && s.cells[i].Count > int64(^uint64(0)>>1)+other.cells[i].Count {
			return ErrCountOverflow
		}
		if err := s.codec.Validate(s.cells[i].Symbol); err != nil {
			return err
		}
		if err := other.codec.Validate(other.cells[i].Symbol); err != nil {
			return err
		}
	}
	for i := range s.cells {
		s.cells[i].Symbol = s.codec.XOR(s.cells[i].Symbol, other.cells[i].Symbol)
		s.cells[i].Checksum ^= other.cells[i].Checksum
		s.cells[i].Count -= other.cells[i].Count
	}
	return nil
}
func (s *Sketch[T]) Decode() (remote, local []HashedSymbol[T], complete bool, err error) {
	d, e := NewDecoder(s.codec)
	if e != nil {
		return nil, nil, false, e
	}
	for _, c := range s.cells {
		if e = d.AddCoded(c); e != nil {
			return nil, nil, false, e
		}
	}
	if e = d.TryDecode(); e != nil {
		return nil, nil, false, e
	}
	return d.Remote(), d.Local(), d.Complete(), nil
}
