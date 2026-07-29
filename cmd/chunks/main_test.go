package main

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/TheFellow/go-riblt"
)

func TestChunkDeduplicatesStoreButPreservesManifest(t *testing.T) {
	body := bytes.Repeat([]byte("x"), chunkSize)
	doc := chunk(append(bytes.Clone(body), body...))
	if len(doc.manifest) != 2 {
		t.Fatalf("manifest has %d entries, want 2", len(doc.manifest))
	}
	if len(doc.chunks) != 1 {
		t.Fatalf("store has %d chunks, want 1", len(doc.chunks))
	}
	rebuilt, err := rebuild(doc.manifest, doc.chunks, doc.digest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rebuilt, append(body, body...)) {
		t.Fatal("rebuilt document differs")
	}
}

func TestContentAddressedSynchronization(t *testing.T) {
	shared := block("shared")
	old := block("old")
	current := block("current")
	added := block("added")
	upstreamBytes := bytes.Join([][]byte{shared, current, added}, nil)
	downstreamBytes := bytes.Join([][]byte{shared, old}, nil)
	upstream, downstream := chunk(upstreamBytes), chunk(downstreamBytes)

	codec, err := riblt.NewBytesCodec(sha256.Size, []byte("0123456789abcdef0123456789abcdef"))
	if err != nil {
		t.Fatal(err)
	}
	missing, obsolete, _, err := reconcile(codec, chunkIDs(upstream.chunks), chunkIDs(downstream.chunks))
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 2 || len(obsolete) != 1 {
		t.Fatalf("missing/obsolete = %d/%d, want 2/1", len(missing), len(obsolete))
	}
	for _, rawID := range missing {
		id := digest(rawID)
		downstream.chunks[id] = bytes.Clone(upstream.chunks[id])
	}
	rebuilt, err := rebuild(upstream.manifest, downstream.chunks, upstream.digest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rebuilt, upstreamBytes) {
		t.Fatal("rebuilt document differs")
	}
}

func TestRebuildRejectsCorruptChunk(t *testing.T) {
	doc := chunk([]byte("content"))
	doc.chunks[doc.manifest[0]][0] ^= 0xff
	if _, err := rebuild(doc.manifest, doc.chunks, doc.digest); err == nil {
		t.Fatal("rebuild accepted a corrupt chunk")
	}
}
