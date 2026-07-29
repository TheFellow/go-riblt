package riblt

import (
	"errors"
	"slices"
	"testing"
)

var testKey = []byte("0123456789abcdef0123456789abcdef")

type pointerCodec struct{ uint64Codec }

func TestCodecValidation(t *testing.T) {
	var nilCodec *pointerCodec
	if _, err := NewEncoder[uint64](nilCodec); !errors.Is(err, ErrNilCodec) {
		t.Fatal(err)
	}
	bad := uint64CodecWithBadZero{uint64Codec{}}
	if _, err := NewDecoder[uint64](bad); !errors.Is(err, ErrInvalidCodec) {
		t.Fatal(err)
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
	cell, err := e.Next()
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
		cell, err := e.Next()
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

func TestSketchRejectsIncompatibleConfiguration(t *testing.T) {
	aCodec, _ := NewUint64Codec(testKey)
	bCodec, _ := NewUint64Codec([]byte("abcdef0123456789abcdef0123456789"))
	a, _ := NewSketch[uint64](aCodec, 8)
	b, _ := NewSketch[uint64](bCodec, 8)
	if err := a.Subtract(b); !errors.Is(err, ErrIncompatible) {
		t.Fatal(err)
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

func TestDecodedSymbolLimit(t *testing.T) {
	c, _ := NewUint64Codec(testKey)
	e, _ := NewEncoder[uint64](c)
	_ = e.Add(1)
	_ = e.Add(2)
	d, _ := NewDecoderWithLimits[uint64](c, DecoderLimits{MaxCells: 100, MaxDecodedSymbols: 1})
	for range 100 {
		cell, err := e.Next()
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
