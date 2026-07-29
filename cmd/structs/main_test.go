package main

import "testing"

func TestRecordCodecBooleanGroup(t *testing.T) {
	codec, err := newRecordCodec([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	a := newRecord(42, 7, "Ada", "body")
	if got := codec.XOR(a, codec.Zero()); got != a {
		t.Fatalf("a XOR zero = %#v, want %#v", got, a)
	}
	if got := codec.XOR(a, a); !codec.IsZero(got) {
		t.Fatalf("a XOR a = %#v, want zero", got)
	}
}

func TestRecordReconciliation(t *testing.T) {
	codec, err := newRecordCodec([]byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	shared := newRecord(1, 1, "Ada", "shared")
	old := newRecord(2, 1, "Grace", "old")
	current := newRecord(2, 2, "Grace", "current")
	added := newRecord(3, 1, "Linus", "added")
	add, remove, _, err := reconcile(codec, []Record{shared, current, added}, []Record{shared, old})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(add, current) || !contains(add, added) || len(add) != 2 {
		t.Fatalf("add = %#v", add)
	}
	if !contains(remove, old) || len(remove) != 1 {
		t.Fatalf("remove = %#v", remove)
	}
}

func contains(records []Record, want Record) bool {
	for _, record := range records {
		if record == want {
			return true
		}
	}
	return false
}
