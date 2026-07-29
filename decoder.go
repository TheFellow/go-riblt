package riblt

import (
	"fmt"
	"math"
)

// Decoder combines a remote coded stream with a local set and recovers both
// sides of the symmetric difference.
type Decoder[T any] struct {
	codec                  Codec[T]
	cells                  []CodedSymbol[T]
	local, initial, remote codingWindow[T]
	ready                  []uint64
	resolved               []bool
	decoded                uint64
	decodedSymbols         uint64
	started, complete      bool
	seen                   symbolSet[T]
	limits                 DecoderLimits
}

// DecoderLimits bounds memory and work controlled by a peer. Zero means no
// package-level limit; protocols exposed to untrusted peers should set every
// field to a finite value and also enforce transport byte/time limits.
type DecoderLimits struct {
	MaxCells          uint64
	MaxLocalSymbols   uint64
	MaxDecodedSymbols uint64
}

func NewDecoder[T any](codec Codec[T]) (*Decoder[T], error) {
	return NewDecoderWithLimits(codec, DecoderLimits{})
}

func NewDecoderWithLimits[T any](codec Codec[T], limits DecoderLimits) (*Decoder[T], error) {
	if err := validateCodec(codec); err != nil {
		return nil, err
	}
	d := &Decoder[T]{codec: codec, seen: newSymbolSet(codec), limits: limits}
	d.local.codec, d.initial.codec, d.remote.codec = codec, codec, codec
	return d, nil
}

func (d *Decoder[T]) AddLocal(v T) error {
	if d.started {
		return ErrDecoderStarted
	}
	if d.limits.MaxLocalSymbols > 0 && uint64(d.seen.entries) >= d.limits.MaxLocalSymbols {
		return fmt.Errorf("%w: local symbols", ErrResourceLimit)
	}
	if err := d.codec.Validate(v); err != nil {
		return err
	}
	s := hashSymbol(d.codec, d.codec.Clone(v))
	if !d.seen.add(s) {
		return ErrDuplicate
	}
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
	if d.limits.MaxCells > 0 && uint64(len(d.cells)) >= d.limits.MaxCells {
		return fmt.Errorf("%w: coded cells", ErrResourceLimit)
	}
	if err := d.codec.Validate(c.Symbol); err != nil {
		return err
	}
	c = cloneCoded(d.codec, c)
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
	d.resolved = append(d.resolved, false)
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
		if d.resolved[idx] {
			continue
		}
		c := d.cells[idx]
		switch c.Count {
		case 1, -1:
			if d.limits.MaxDecodedSymbols > 0 && d.decodedSymbols >= d.limits.MaxDecodedSymbols {
				d.ready = d.ready[i:]
				return fmt.Errorf("%w: decoded symbols", ErrResourceLimit)
			}
			s := hashSymbol(d.codec, d.codec.Clone(c.Symbol))
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
				d.ready = d.ready[i+1:]
				return err
			}
			d.decoded++
			d.decodedSymbols++
			d.resolved[idx] = true
		case 0:
			if c.Checksum == 0 && d.codec.IsZero(c.Symbol) {
				d.decoded++
				d.resolved[idx] = true
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
	return cloneResults(d.codec, d.remote.symbols)
}
func (d *Decoder[T]) Local() []HashedSymbol[T] {
	return cloneResults(d.codec, d.local.symbols)
}
func cloneResults[T any](codec Codec[T], in []HashedSymbol[T]) []HashedSymbol[T] {
	out := make([]HashedSymbol[T], len(in))
	for i := range in {
		out[i] = cloneHashed(codec, in[i])
	}
	return out
}
func (d *Decoder[T]) Reset() {
	clear(d.cells)
	clear(d.ready)
	clear(d.resolved)
	d.cells = d.cells[:0]
	d.ready = d.ready[:0]
	d.resolved = d.resolved[:0]
	d.local.reset()
	d.initial.reset()
	d.remote.reset()
	d.decoded = 0
	d.decodedSymbols = 0
	d.started = false
	d.complete = false
	d.seen.reset()
}
