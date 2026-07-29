package riblt

import (
	"errors"
	"math/rand"
	"slices"
	"testing"
)

type uint64Codec struct{}

func (uint64Codec) Zero() uint64              { return 0 }
func (uint64Codec) IsZero(v uint64) bool      { return v == 0 }
func (uint64Codec) Clone(v uint64) uint64     { return v }
func (uint64Codec) Equal(a, b uint64) bool    { return a == b }
func (uint64Codec) Validate(uint64) error     { return nil }
func (uint64Codec) XOR(a, b uint64) uint64    { return a ^ b }
func (uint64Codec) CompatibilityID() [32]byte { return [32]byte{1} }
func mix(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}
func (uint64Codec) MappingHash(v uint64) uint64 { return mix(v ^ 0x6d617070696e6701) }
func (uint64Codec) Checksum(v uint64) uint64    { return mix(v ^ 0x636865636b737501) }

func reconcile(t testing.TB, a, b []uint64) (*Decoder[uint64], int) {
	t.Helper()
	enc, _ := NewEncoder[uint64](uint64Codec{})
	dec, _ := NewDecoder[uint64](uint64Codec{})
	for _, v := range a {
		if err := enc.Add(v); err != nil {
			t.Fatal(err)
		}
	}
	for _, v := range b {
		if err := dec.AddLocal(v); err != nil {
			t.Fatal(err)
		}
	}
	for n := 1; n < 100000; n++ {
		c, err := enc.Next()
		if err != nil {
			t.Fatal(err)
		}
		if err = dec.AddCoded(c); err != nil {
			t.Fatal(err)
		}
		if err = dec.TryDecode(); err != nil {
			t.Fatal(err)
		}
		if dec.Complete() {
			return dec, n
		}
	}
	t.Fatal("did not decode")
	return nil, 0
}
func values[T any](in []HashedSymbol[T]) []T {
	out := make([]T, len(in))
	for i := range in {
		out[i] = in[i].Symbol
	}
	return out
}

func TestReconcile(t *testing.T) {
	d, n := reconcile(t, []uint64{1, 2, 3, 4, 5}, []uint64{1, 3, 4, 5, 11})
	if n < 1 {
		t.Fatal(n)
	}
	if got := values(d.Remote()); !slices.Equal(got, []uint64{2}) {
		t.Fatalf("remote %v", got)
	}
	if got := values(d.Local()); !slices.Equal(got, []uint64{11}) {
		t.Fatalf("local %v", got)
	}
}
func TestEqualSetsRequireFirstCell(t *testing.T) {
	d, n := reconcile(t, []uint64{1, 2}, []uint64{1, 2})
	if n != 1 || !d.Complete() {
		t.Fatalf("n=%d complete=%v", n, d.Complete())
	}
}

func TestMalformedEmptyCellDoesNotComplete(t *testing.T) {
	d, _ := NewDecoder[uint64](uint64Codec{})
	if err := d.AddCoded(CodedSymbol[uint64]{Symbol: 1, Count: 0, Checksum: 0}); err != nil {
		t.Fatal(err)
	}
	if err := d.TryDecode(); err != nil {
		t.Fatal(err)
	}
	if d.Complete() {
		t.Fatal("non-identity symbol must not be accepted as an empty cell")
	}
}
func TestRandomSetProperties(t *testing.T) {
	for seed := range int64(100) {
		r := rand.New(rand.NewSource(seed))
		common := r.Intn(100)
		left, right := r.Intn(30), r.Intn(30)
		a := make([]uint64, 0, common+left)
		b := make([]uint64, 0, common+right)
		for i := range common {
			a = append(a, uint64(i))
			b = append(b, uint64(i))
		}
		wantA := make([]uint64, left)
		wantB := make([]uint64, right)
		for i := range wantA {
			wantA[i] = uint64(common + i + 1000)
			a = append(a, wantA[i])
		}
		for i := range wantB {
			wantB[i] = uint64(common + i + 2000)
			b = append(b, wantB[i])
		}
		d, _ := reconcile(t, a, b)
		gotA, gotB := values(d.Remote()), values(d.Local())
		slices.Sort(gotA)
		slices.Sort(gotB)
		if !slices.Equal(gotA, wantA) || !slices.Equal(gotB, wantB) {
			t.Fatalf("seed %d got %v/%v want %v/%v", seed, gotA, gotB, wantA, wantB)
		}
	}
}
func TestStateAndDuplicateErrors(t *testing.T) {
	e, _ := NewEncoder[uint64](uint64Codec{})
	if err := e.Add(1); err != nil {
		t.Fatal(err)
	}
	if err := e.Add(1); !errors.Is(err, ErrDuplicate) {
		t.Fatal(err)
	}
	if _, err := e.Next(); err != nil {
		t.Fatal(err)
	}
	if err := e.Add(2); !errors.Is(err, ErrEncoderStarted) {
		t.Fatal(err)
	}
	d, _ := NewDecoder[uint64](uint64Codec{})
	if err := d.AddLocal(1); err != nil {
		t.Fatal(err)
	}
	if err := d.AddLocal(1); !errors.Is(err, ErrDuplicate) {
		t.Fatal(err)
	}
	c, _ := e.Next()
	if err := d.AddCoded(c); err != nil {
		t.Fatal(err)
	}
	if err := d.AddLocal(2); !errors.Is(err, ErrDecoderStarted) {
		t.Fatal(err)
	}
}
func TestReset(t *testing.T) {
	e, _ := NewEncoder[uint64](uint64Codec{})
	_ = e.Add(1)
	_, _ = e.Next()
	e.Reset()
	if err := e.Add(1); err != nil {
		t.Fatal(err)
	}
	d, _ := NewDecoder[uint64](uint64Codec{})
	_ = d.AddLocal(1)
	d.Reset()
	if err := d.AddLocal(1); err != nil {
		t.Fatal(err)
	}
}
func TestSketch(t *testing.T) {
	a, _ := NewSketch[uint64](uint64Codec{}, 20)
	b, _ := NewSketch[uint64](uint64Codec{}, 20)
	for _, v := range []uint64{1, 2, 3} {
		_ = a.Add(v)
	}
	for _, v := range []uint64{1, 3, 4} {
		_ = b.Add(v)
	}
	if err := a.Subtract(b); err != nil {
		t.Fatal(err)
	}
	remote, local, ok, err := a.Decode()
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if !slices.Equal(values(remote), []uint64{2}) || !slices.Equal(values(local), []uint64{4}) {
		t.Fatalf("%v %v", values(remote), values(local))
	}
}

func TestMappingVectors(t *testing.T) {
	tests := []struct {
		seed uint64
		want []uint64
	}{{1, []uint64{1, 2, 3, 6, 11}}, {0, []uint64{1, 2, 6, 12, 24}}, {0x123456789abcdef0, []uint64{1, 2, 6, 10, 31}}}
	for _, tt := range tests {
		m := newMapping(tt.seed)
		got := make([]uint64, 5)
		for i := range got {
			n, err := m.next()
			if err != nil {
				t.Fatal(err)
			}
			got[i] = n
		}
		if !slices.Equal(got, tt.want) {
			t.Fatalf("seed %#x: got %v want %v", tt.seed, got, tt.want)
		}
	}
}

func FuzzReconcile(f *testing.F) {
	f.Add(uint64(1), uint8(10), uint8(3), uint8(4))
	f.Fuzz(func(t *testing.T, seed uint64, n, la, lb uint8) {
		if n > 80 || la > 20 || lb > 20 {
			t.Skip()
		}
		r := rand.New(rand.NewSource(int64(seed)))
		seen := map[uint64]bool{}
		next := func() uint64 {
			for {
				x := r.Uint64()
				if !seen[x] {
					seen[x] = true
					return x
				}
			}
		}
		a, b := []uint64{}, []uint64{}
		for range int(n) {
			x := next()
			a = append(a, x)
			b = append(b, x)
		}
		for range int(la) {
			a = append(a, next())
		}
		for range int(lb) {
			b = append(b, next())
		}
		d, _ := reconcile(t, a, b)
		if len(d.Remote()) != int(la) || len(d.Local()) != int(lb) {
			t.Fatalf("sizes %d/%d", len(d.Remote()), len(d.Local()))
		}
	})
}
