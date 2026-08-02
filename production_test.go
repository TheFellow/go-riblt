package riblt

import (
	"errors"
	"slices"
	"testing"
)

var testKey = []byte("0123456789abcdef0123456789abcdef")

type pointerCodec struct{ uint64Codec }

func assertInvalidCodecRejected[T any](t *testing.T, codec Codec[T], value T) {
	t.Helper()
	if _, err := NewEncoder[T](codec); !errors.Is(err, ErrInvalidCodec) {
		t.Errorf("NewEncoder error = %v, want ErrInvalidCodec", err)
	}
	if _, err := NewDecoder[T](codec); !errors.Is(err, ErrInvalidCodec) {
		t.Errorf("NewDecoder error = %v, want ErrInvalidCodec", err)
	}
	if _, err := NewSketch[T](codec, 1); !errors.Is(err, ErrInvalidCodec) {
		t.Errorf("NewSketch error = %v, want ErrInvalidCodec", err)
	}
	if _, err := Hash[T](codec, value); !errors.Is(err, ErrInvalidCodec) {
		t.Errorf("Hash error = %v, want ErrInvalidCodec", err)
	}
}

func TestCodecValidation(t *testing.T) {
	var nilCodec *pointerCodec
	if _, err := NewEncoder[uint64](nilCodec); !errors.Is(err, ErrNilCodec) {
		t.Fatal(err)
	}
	bad := uint64CodecWithBadZero{uint64Codec{}}
	if _, err := NewDecoder[uint64](bad); !errors.Is(err, ErrInvalidCodec) {
		t.Fatal(err)
	}
	weakUint64, err := NewUint64Codec([]byte("short"))
	if !errors.Is(err, ErrWeakKey) {
		t.Fatalf("NewUint64Codec error = %v, want ErrWeakKey", err)
	}
	for name, codec := range map[string]Codec[uint64]{
		"zero value":      Uint64Codec{},
		"weak-key result": weakUint64,
	} {
		t.Run("uint64 "+name, func(t *testing.T) {
			assertInvalidCodecRejected(t, codec, uint64(1))
		})
	}
	weakBytes, err := NewBytesCodec(4, []byte("short"))
	if !errors.Is(err, ErrWeakKey) {
		t.Fatalf("NewBytesCodec error = %v, want ErrWeakKey", err)
	}
	for name, codec := range map[string]Codec[[]byte]{
		"zero value":      BytesCodec{},
		"weak-key result": weakBytes,
	} {
		t.Run("bytes "+name, func(t *testing.T) {
			assertInvalidCodecRejected(t, codec, []byte{1, 2, 3, 4})
		})
	}
}

type uint64CodecWithBadZero struct{ uint64Codec }

func (uint64CodecWithBadZero) Zero() uint64 { return 1 }

func TestDefaultCodecs(t *testing.T) {
	u, err := NewUint64Codec(testKey)
	if err != nil {
		t.Fatal(err)
	}
	if u.MappingHash(7) == u.Checksum(7) {
		t.Fatal("hash domains overlap")
	}
	if got := u.MappingHash(7); got != 0xa64ca71c925e4fd0 {
		t.Fatalf("mapping golden: %#x", got)
	}
	if got := u.Checksum(7); got != 0x5cd470974dde9ef1 {
		t.Fatalf("checksum golden: %#x", got)
	}
	wantID := [32]byte{0x20, 0xce, 0x66, 0x11, 0x28, 0x1c, 0x78, 0x97, 0x45, 0xc7, 0x04, 0xf4, 0x07, 0x03, 0xa6, 0xd1, 0xfa, 0x97, 0x6f, 0x55, 0x22, 0x0e, 0x6e, 0x37, 0xbe, 0xee, 0xa2, 0x93, 0xa6, 0xb7, 0x9c, 0x0c}
	if got := u.CompatibilityID(); got != wantID {
		t.Fatalf("compatibility golden: %x", got)
	}
	if _, err = NewUint64Codec([]byte("short")); !errors.Is(err, ErrWeakKey) {
		t.Fatal(err)
	}
	b, err := NewBytesCodec(4, testKey)
	if err != nil {
		t.Fatal(err)
	}
	if err = b.Validate([]byte{1}); !errors.Is(err, ErrInvalidSymbol) {
		t.Fatal(err)
	}
	if got := b.XOR([]byte{1, 2, 3, 4}, []byte{4, 3, 2, 1}); !slices.Equal(got, []byte{5, 1, 1, 5}) {
		t.Fatal(got)
	}
}

func TestMutableValuesAreOwned(t *testing.T) {
	c, _ := NewBytesCodec(4, testKey)
	e, _ := NewEncoder[[]byte](c)
	v := []byte{1, 2, 3, 4}
	if err := e.Add(v); err != nil {
		t.Fatal(err)
	}
	v[0] = 99
	cell, err := e.next()
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cell.Symbol, []byte{1, 2, 3, 4}) {
		t.Fatalf("input aliased: %v", cell.Symbol)
	}

	d, _ := NewDecoder[[]byte](c)
	if err = d.AddCoded(cell); err != nil {
		t.Fatal(err)
	}
	cell.Symbol[0] = 88
	if err = d.TryDecode(); err != nil {
		t.Fatal(err)
	}
	got := d.Remote()
	if len(got) != 1 || !slices.Equal(got[0].Symbol, []byte{1, 2, 3, 4}) {
		t.Fatalf("coded input aliased: %v", got)
	}
	got[0].Symbol[0] = 77
	if d.Remote()[0].Symbol[0] != 1 {
		t.Fatal("result aliases decoder state")
	}
}

func TestUint64CodecReconciles(t *testing.T) {
	c, _ := NewUint64Codec(testKey)
	e, _ := NewEncoder[uint64](c)
	d, _ := NewDecoderWithLimits[uint64](c, DecoderLimits{MaxCells: 100, MaxLocalSymbols: 10, MaxDecodedSymbols: 10})
	for _, v := range []uint64{1, 2, 3} {
		if err := e.Add(v); err != nil {
			t.Fatal(err)
		}
	}
	for _, v := range []uint64{1, 3, 4} {
		if err := d.AddLocal(v); err != nil {
			t.Fatal(err)
		}
	}
	for !d.Complete() {
		cell, err := e.next()
		if err != nil {
			t.Fatal(err)
		}
		if err = d.AddCoded(cell); err != nil {
			t.Fatal(err)
		}
		if err = d.TryDecode(); err != nil {
			t.Fatal(err)
		}
	}
	if got := values(d.Remote()); !slices.Equal(got, []uint64{2}) {
		t.Fatal(got)
	}
	if got := values(d.Local()); !slices.Equal(got, []uint64{4}) {
		t.Fatal(got)
	}
}

type collidingCodec struct{ uint64Codec }

func (collidingCodec) MappingHash(uint64) uint64 { return 1 }
func (collidingCodec) Checksum(uint64) uint64    { return 2 }
func (collidingCodec) CompatibilityID() [32]byte { return [32]byte{2} }

func TestDuplicateDetectionUsesEquality(t *testing.T) {
	e, _ := NewEncoder[uint64](collidingCodec{})
	if err := e.Add(1); err != nil {
		t.Fatal(err)
	}
	if err := e.Add(2); err != nil {
		t.Fatalf("distinct colliding value rejected: %v", err)
	}
	if err := e.Add(1); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate accepted: %v", err)
	}
}

type placementCollisionCodec struct{ uint64Codec }

func (placementCollisionCodec) MappingHash(uint64) uint64 { return 1 }
func (placementCollisionCodec) CompatibilityID() [32]byte { return [32]byte{3} }

func TestMappingCollisionCannotBeRepairedByChecksum(t *testing.T) {
	codec := placementCollisionCodec{}
	e, err := NewEncoder[uint64](codec)
	if err != nil {
		t.Fatal(err)
	}
	d, err := NewDecoder[uint64](codec)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Add(1); err != nil {
		t.Fatal(err)
	}
	if err := d.AddLocal(2); err != nil {
		t.Fatal(err)
	}
	for range 32 {
		cell, err := e.next()
		if err != nil {
			t.Fatal(err)
		}
		if err := d.AddCoded(cell); err != nil {
			t.Fatal(err)
		}
		if err := d.TryDecode(); err != nil {
			t.Fatal(err)
		}
		if d.Complete() {
			t.Fatal("distinct symbols with identical placement unexpectedly converged")
		}
	}
}

func TestSketchRejectsIncompatibleConfiguration(t *testing.T) {
	aCodec, _ := NewUint64Codec(testKey)
	bCodec, _ := NewUint64Codec([]byte("abcdef0123456789abcdef0123456789"))
	a, _ := NewSketch[uint64](aCodec, 8)
	b, _ := NewSketch[uint64](bCodec, 8)
	if err := a.Subtract(b); !errors.Is(err, ErrIncompatible) {
		t.Fatal(err)
	}
	if err := a.Add(1); err != nil {
		t.Fatalf("failed subtraction sealed sketch: %v", err)
	}
}

func TestSketchIsReadOnlyAfterSubtract(t *testing.T) {
	codec, err := NewUint64Codec(testKey)
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewSketch[uint64](codec, 20)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewSketch[uint64](codec, 20)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Add(1); err != nil {
		t.Fatal(err)
	}
	if err := b.Add(2); err != nil {
		t.Fatal(err)
	}
	if err := a.Subtract(b); err != nil {
		t.Fatal(err)
	}
	want := a.Cells()
	if _, _, complete, err := a.Decode(); err != nil || !complete {
		t.Fatalf("Decode after Subtract: complete=%v err=%v", complete, err)
	}
	for name, mutate := range map[string]func() error{
		"Add":            func() error { return a.Add(3) },
		"Remove":         func() error { return a.Remove(1) },
		"Subtract again": func() error { return a.Subtract(b) },
	} {
		t.Run(name, func(t *testing.T) {
			if err := mutate(); !errors.Is(err, ErrSketchSubtracted) {
				t.Fatalf("error = %v, want ErrSketchSubtracted", err)
			}
			if got := a.Cells(); !slices.Equal(got, want) {
				t.Fatalf("sealed sketch mutated: got %#v want %#v", got, want)
			}
		})
	}
}

func TestSketchRejectsSubtractedRightHandSide(t *testing.T) {
	codec, err := NewUint64Codec(testKey)
	if err != nil {
		t.Fatal(err)
	}
	receiver, err := NewSketch[uint64](codec, 20)
	if err != nil {
		t.Fatal(err)
	}
	right, err := NewSketch[uint64](codec, 20)
	if err != nil {
		t.Fatal(err)
	}
	base, err := NewSketch[uint64](codec, 20)
	if err != nil {
		t.Fatal(err)
	}
	if err := receiver.Add(1); err != nil {
		t.Fatal(err)
	}
	if err := right.Add(2); err != nil {
		t.Fatal(err)
	}
	if err := base.Add(3); err != nil {
		t.Fatal(err)
	}
	if err := right.Subtract(base); err != nil {
		t.Fatal(err)
	}
	want := receiver.Cells()
	if err := receiver.Subtract(right); !errors.Is(err, ErrSketchSubtracted) {
		t.Fatalf("Subtract error = %v, want ErrSketchSubtracted", err)
	}
	if got := receiver.Cells(); !slices.Equal(got, want) {
		t.Fatalf("rejected RHS mutated receiver: got %#v want %#v", got, want)
	}
	if err := receiver.Add(4); err != nil {
		t.Fatalf("rejected RHS sealed receiver: %v", err)
	}
}

func TestDecoderLimitsAndMalformedSymbols(t *testing.T) {
	c, _ := NewBytesCodec(4, testKey)
	d, _ := NewDecoderWithLimits[[]byte](c, DecoderLimits{MaxCells: 1, MaxLocalSymbols: 1})
	if err := d.AddLocal([]byte{1, 2, 3, 4}); err != nil {
		t.Fatal(err)
	}
	if err := d.AddLocal([]byte{4, 3, 2, 1}); !errors.Is(err, ErrResourceLimit) {
		t.Fatal(err)
	}
	if err := d.AddCoded(CodedSymbol[[]byte]{Symbol: []byte{1}}); !errors.Is(err, ErrInvalidSymbol) {
		t.Fatal(err)
	}
	if err := d.AddCoded(CodedSymbol[[]byte]{Symbol: make([]byte, 4)}); err != nil {
		t.Fatal(err)
	}
	if err := d.AddCoded(CodedSymbol[[]byte]{Symbol: make([]byte, 4)}); !errors.Is(err, ErrResourceLimit) {
		t.Fatal(err)
	}
}

func TestAddCodedRejectedInputDoesNotStartDecoder(t *testing.T) {
	c, err := NewBytesCodec(4, testKey)
	if err != nil {
		t.Fatal(err)
	}
	d, err := NewDecoderWithLimits[[]byte](c, DecoderLimits{MaxCells: 2, MaxLocalSymbols: 2})
	if err != nil {
		t.Fatal(err)
	}
	if err := d.AddCoded(CodedSymbol[[]byte]{Symbol: []byte{1}}); !errors.Is(err, ErrInvalidSymbol) {
		t.Fatalf("malformed cell error = %v, want ErrInvalidSymbol", err)
	}
	if d.started || len(d.cells) != 0 || len(d.ready) != 0 || len(d.resolved) != 0 {
		t.Fatalf("malformed cell changed decoder state: started=%v cells=%d ready=%d resolved=%d", d.started, len(d.cells), len(d.ready), len(d.resolved))
	}
	if err := d.AddLocal([]byte{1, 2, 3, 4}); err != nil {
		t.Fatalf("AddLocal after rejected first cell: %v", err)
	}
	if err := d.AddCoded(CodedSymbol[[]byte]{Symbol: make([]byte, 4)}); err != nil {
		t.Fatal(err)
	}
	type decoderState struct {
		cells, ready, resolved int
		decoded                uint64
		initial, local, remote uint64
		started, complete      bool
	}
	state := func() decoderState {
		return decoderState{
			cells: len(d.cells), ready: len(d.ready), resolved: len(d.resolved),
			decoded: d.decoded, initial: d.initial.next, local: d.local.next,
			remote: d.remote.next, started: d.started, complete: d.complete,
		}
	}
	want := state()
	if err := d.AddCoded(CodedSymbol[[]byte]{Symbol: []byte{1}}); !errors.Is(err, ErrInvalidSymbol) {
		t.Fatalf("malformed later cell error = %v, want ErrInvalidSymbol", err)
	}
	if got := state(); got != want {
		t.Fatalf("malformed later cell changed decoder state: got %+v want %+v", got, want)
	}
	if err := d.AddCoded(CodedSymbol[[]byte]{Symbol: make([]byte, 4)}); err != nil {
		t.Fatal(err)
	}
	want = state()
	if err := d.AddCoded(CodedSymbol[[]byte]{Symbol: make([]byte, 4)}); !errors.Is(err, ErrResourceLimit) {
		t.Fatalf("over-limit cell error = %v, want ErrResourceLimit", err)
	}
	if got := state(); got != want {
		t.Fatalf("over-limit cell changed decoder state: got %+v want %+v", got, want)
	}
}

func TestDecodedSymbolLimit(t *testing.T) {
	c, _ := NewUint64Codec(testKey)
	e, _ := NewEncoder[uint64](c)
	_ = e.Add(1)
	_ = e.Add(2)
	d, _ := NewDecoderWithLimits[uint64](c, DecoderLimits{MaxCells: 100, MaxDecodedSymbols: 1})
	for range 100 {
		cell, err := e.next()
		if err != nil {
			t.Fatal(err)
		}
		if err = d.AddCoded(cell); err != nil {
			t.Fatal(err)
		}
		err = d.TryDecode()
		if errors.Is(err, ErrResourceLimit) {
			return
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	t.Fatal("decoded symbol limit was not enforced")
}

func FuzzMalformedCodedBytes(f *testing.F) {
	f.Add([]byte{1, 2, 3, 4}, uint64(3), int64(1))
	f.Fuzz(func(t *testing.T, symbol []byte, checksum uint64, count int64) {
		c, _ := NewBytesCodec(4, testKey)
		d, _ := NewDecoderWithLimits[[]byte](c, DecoderLimits{MaxCells: 2, MaxDecodedSymbols: 2})
		err := d.AddCoded(CodedSymbol[[]byte]{Symbol: symbol, Checksum: checksum, Count: count})
		if len(symbol) != 4 {
			if !errors.Is(err, ErrInvalidSymbol) {
				t.Fatalf("wrong-width symbol: %v", err)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		if err := d.TryDecode(); err != nil && !errors.Is(err, ErrResourceLimit) {
			t.Fatal(err)
		}
	})
}
