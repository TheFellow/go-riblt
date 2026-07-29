package main

import (
	"sort"
	"strings"
	"testing"

	"github.com/TheFellow/go-riblt"
)

func TestDocumentEncodingRoundTrip(t *testing.T) {
	for _, text := range []string{"", "plain text", "UTF-8: λ and 世界", strings.Repeat("x", maxTextBytes)} {
		slot, err := encodeDocument(text)
		if err != nil {
			t.Fatalf("encodeDocument(%q): %v", text, err)
		}
		got, err := decodeDocument(slot)
		if err != nil {
			t.Fatalf("decodeDocument(%q): %v", text, err)
		}
		if got != text {
			t.Fatalf("round trip = %q, want %q", got, text)
		}
	}
}

func TestDocumentEncodingRejectsOversizeInput(t *testing.T) {
	if _, err := encodeDocument(strings.Repeat("x", maxTextBytes+1)); err == nil {
		t.Fatal("encodeDocument accepted an oversized document")
	}
}

func TestDocumentReconciliation(t *testing.T) {
	codec, err := newTestCodec()
	if err != nil {
		t.Fatal(err)
	}
	documents := mustEncode("shared", "old", "current", "added")
	shared, old, current, added := documents[0], documents[1], documents[2], documents[3]
	add, remove, _, err := reconcile(codec, [][]byte{shared, current, added}, [][]byte{shared, old})
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(sorted(mustDecode(add)), ","); got != "added,current" {
		t.Fatalf("add = %q", got)
	}
	if got := strings.Join(sorted(mustDecode(remove)), ","); got != "old" {
		t.Fatalf("remove = %q", got)
	}
}

func newTestCodec() (riblt.BytesCodec, error) {
	return riblt.NewBytesCodec(documentWidth, []byte("0123456789abcdef0123456789abcdef"))
}

func sorted(values []string) []string {
	sort.Strings(values)
	return values
}
