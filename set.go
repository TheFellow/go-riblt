package riblt

type symbolSet[T any] struct {
	codec   Codec[T]
	byHash  map[identity][]T
	entries int
}

func newSymbolSet[T any](codec Codec[T]) symbolSet[T] {
	return symbolSet[T]{codec: codec, byHash: make(map[identity][]T)}
}

func (s *symbolSet[T]) add(v HashedSymbol[T]) bool {
	id := identity{v.MappingHash, v.Checksum}
	for _, existing := range s.byHash[id] {
		if s.codec.Equal(existing, v.Symbol) {
			return false
		}
	}
	s.byHash[id] = append(s.byHash[id], s.codec.Clone(v.Symbol))
	s.entries++
	return true
}

func (s *symbolSet[T]) remove(v HashedSymbol[T]) bool {
	id := identity{v.MappingHash, v.Checksum}
	values := s.byHash[id]
	for i, existing := range values {
		if s.codec.Equal(existing, v.Symbol) {
			values[i] = values[len(values)-1]
			values = values[:len(values)-1]
			if len(values) == 0 {
				delete(s.byHash, id)
			} else {
				s.byHash[id] = values
			}
			s.entries--
			return true
		}
	}
	return false
}

func (s *symbolSet[T]) reset() {
	clear(s.byHash)
	s.entries = 0
}
