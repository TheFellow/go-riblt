package riblt

// Sketch is a mutable fixed-length prefix, useful when its size is known.
type Sketch[T any] struct {
	codec Codec[T]
	cells []CodedSymbol[T]
	seen  map[identity]struct{}
}

func NewSketch[T any](codec Codec[T], length int) (*Sketch[T], error) {
	if codec == nil {
		return nil, ErrNilCodec
	}
	if length < 0 {
		return nil, ErrDifferentLength
	}
	s := &Sketch[T]{codec: codec, cells: make([]CodedSymbol[T], length), seen: make(map[identity]struct{})}
	for i := range s.cells {
		s.cells[i] = zeroCoded(codec)
	}
	return s, nil
}
func (s *Sketch[T]) Cells() []CodedSymbol[T] { return append([]CodedSymbol[T](nil), s.cells...) }
func (s *Sketch[T]) Add(v T) error           { return s.AddHashed(Hash(s.codec, v)) }
func (s *Sketch[T]) AddHashed(v HashedSymbol[T]) error {
	id := identity{v.MappingHash, v.Checksum}
	if _, ok := s.seen[id]; ok {
		return ErrDuplicate
	}
	s.seen[id] = struct{}{}
	return s.update(v, 1)
}
func (s *Sketch[T]) Remove(v T) error {
	h := Hash(s.codec, v)
	id := identity{h.MappingHash, h.Checksum}
	if _, ok := s.seen[id]; !ok {
		return ErrNotPresent
	}
	if err := s.update(h, -1); err != nil {
		return err
	}
	delete(s.seen, id)
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
	if len(s.cells) != len(other.cells) {
		return ErrDifferentLength
	}
	for i := range s.cells {
		if other.cells[i].Count > 0 && s.cells[i].Count < -int64(^uint64(0)>>1)-1+other.cells[i].Count || other.cells[i].Count < 0 && s.cells[i].Count > int64(^uint64(0)>>1)+other.cells[i].Count {
			return ErrCountOverflow
		}
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
