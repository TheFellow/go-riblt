package riblt

import "math"

// Decoder combines a remote coded stream with a local set and recovers both
// sides of the symmetric difference.
type Decoder[T any] struct {
	codec                  Codec[T]
	cells                  []CodedSymbol[T]
	local, initial, remote codingWindow[T]
	ready                  []uint64
	decoded                uint64
	started, complete      bool
	seen                   map[identity]struct{}
}

func NewDecoder[T any](codec Codec[T]) (*Decoder[T], error) {
	if codec == nil {
		return nil, ErrNilCodec
	}
	d := &Decoder[T]{codec: codec, seen: make(map[identity]struct{})}
	d.local.codec, d.initial.codec, d.remote.codec = codec, codec, codec
	return d, nil
}

func (d *Decoder[T]) AddLocal(v T) error { return d.AddLocalHashed(Hash(d.codec, v)) }
func (d *Decoder[T]) AddLocalHashed(s HashedSymbol[T]) error {
	if d.started {
		return ErrDecoderStarted
	}
	id := identity{s.MappingHash, s.Checksum}
	if _, ok := d.seen[id]; ok {
		return ErrDuplicate
	}
	d.seen[id] = struct{}{}
	d.initial.add(s, newMapping(s.MappingHash))
	return nil
}

func (d *Decoder[T]) decodable(c CodedSymbol[T]) bool {
	return (c.Count == 1 || c.Count == -1) && c.Checksum == d.codec.Checksum(c.Symbol)
}

// AddCoded consumes the next coded symbol. Symbols must arrive in stream order.
func (d *Decoder[T]) AddCoded(c CodedSymbol[T]) error {
	if d.complete {
		return ErrAlreadyDecoded
	}
	d.started = true
	var err error
	if c, err = d.initial.apply(c, -1); err != nil {
		return err
	}
	if c, err = d.remote.apply(c, -1); err != nil {
		return err
	}
	if c, err = d.local.apply(c, 1); err != nil {
		return err
	}
	d.cells = append(d.cells, c)
	if d.decodable(c) || c.Count == 0 && c.Checksum == 0 && d.codec.IsZero(c.Symbol) {
		d.ready = append(d.ready, uint64(len(d.cells)-1))
	}
	return nil
}

func (d *Decoder[T]) applyNew(s HashedSymbol[T], direction int64) (randomMapping, error) {
	m := newMapping(s.MappingHash)
	for m.last < uint64(len(d.cells)) {
		i := m.last
		c := d.cells[i]
		if direction > 0 && c.Count == math.MaxInt64 || direction < 0 && c.Count == math.MinInt64 {
			return m, ErrCountOverflow
		}
		d.cells[i] = apply(d.codec, c, s, direction)
		if d.decodable(d.cells[i]) {
			d.ready = append(d.ready, i)
		}
		if _, err := m.next(); err != nil {
			return m, err
		}
	}
	return m, nil
}

// TryDecode peels every currently decodable cell. Complete reports whether the
// received prefix is sufficient; an empty prefix is deliberately incomplete.
func (d *Decoder[T]) TryDecode() error {
	for i := 0; i < len(d.ready); i++ {
		idx := d.ready[i]
		c := d.cells[idx]
		switch c.Count {
		case 1, -1:
			s := Hash(d.codec, d.codec.XOR(d.codec.Zero(), c.Symbol))
			// Preserve the transmitted checksum after validating it above.
			s.Checksum = c.Checksum
			var m randomMapping
			var err error
			if c.Count == 1 {
				m, err = d.applyNew(s, -1)
				if err == nil {
					d.remote.add(s, m)
				}
			} else {
				m, err = d.applyNew(s, 1)
				if err == nil {
					d.local.add(s, m)
				}
			}
			if err != nil {
				return err
			}
			d.decoded++
		case 0:
			if c.Checksum == 0 && d.codec.IsZero(c.Symbol) {
				d.decoded++
			}
		default:
			// A queued cell may already have been reduced to zero, but cannot
			// become a non-singleton under peeling.
			continue
		}
	}
	d.ready = d.ready[:0]
	d.complete = len(d.cells) > 0 && d.decoded == uint64(len(d.cells))
	return nil
}

func (d *Decoder[T]) Complete() bool { return d.complete }
func (d *Decoder[T]) Remote() []HashedSymbol[T] {
	return append([]HashedSymbol[T](nil), d.remote.symbols...)
}
func (d *Decoder[T]) Local() []HashedSymbol[T] {
	return append([]HashedSymbol[T](nil), d.local.symbols...)
}
func (d *Decoder[T]) Reset() {
	d.cells = d.cells[:0]
	d.ready = d.ready[:0]
	d.local.reset()
	d.initial.reset()
	d.remote.reset()
	d.decoded = 0
	d.started = false
	d.complete = false
	clear(d.seen)
}
