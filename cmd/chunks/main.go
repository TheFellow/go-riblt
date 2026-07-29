// Command chunks uses RIBLT to discover which content-addressed chunks a
// downstream needs, then rebuilds a document from its ordered manifest.
package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"log"
	"sort"

	"github.com/TheFellow/go-riblt"
)

const chunkSize = 32

type digest = [sha256.Size]byte

type chunkedDocument struct {
	manifest []digest
	chunks   map[digest][]byte
	digest   digest
}

func chunk(data []byte) chunkedDocument {
	doc := chunkedDocument{
		chunks: make(map[digest][]byte),
		digest: sha256.Sum256(data),
	}
	for start := 0; start < len(data); start += chunkSize {
		end := min(start+chunkSize, len(data))
		body := bytes.Clone(data[start:end])
		id := sha256.Sum256(body)
		doc.manifest = append(doc.manifest, id)
		doc.chunks[id] = body
	}
	return doc
}

func rebuild(manifest []digest, chunks map[digest][]byte, want digest) ([]byte, error) {
	var document []byte
	for _, id := range manifest {
		body, ok := chunks[id]
		if !ok {
			return nil, fmt.Errorf("missing chunk %x", id[:4])
		}
		if got := sha256.Sum256(body); got != id {
			return nil, fmt.Errorf("chunk %x failed content verification", id[:4])
		}
		document = append(document, body...)
	}
	if got := sha256.Sum256(document); got != want {
		return nil, fmt.Errorf("rebuilt document failed verification")
	}
	return document, nil
}

func main() {
	// Equal-sized edits keep later fixed-size chunk boundaries aligned. A
	// production system may use content-defined chunking to retain sharing when
	// insertions shift bytes; that choice is independent of RIBLT.
	sharedA := block("RIBLT reconciles chunk IDs.")
	sharedB := block("Only missing chunks cross wire.")
	oldBlock := block("The downstream has old content.")
	newBlock := block("The upstream has fresh content.")
	addedBlock := block("Manifests preserve chunk order.")

	upstreamBytes := bytes.Join([][]byte{sharedA, newBlock, sharedB, addedBlock}, nil)
	downstreamBytes := bytes.Join([][]byte{sharedA, oldBlock, sharedB}, nil)
	upstream := chunk(upstreamBytes)
	downstream := chunk(downstreamBytes)

	key := []byte("example-only-key-change-in-production")
	codec, err := riblt.NewBytesCodec(sha256.Size, key)
	if err != nil {
		log.Fatal(err)
	}
	missing, obsolete, cells, err := reconcile(codec, chunkIDs(upstream.chunks), chunkIDs(downstream.chunks))
	if err != nil {
		log.Fatal(err)
	}

	// The surrounding protocol sends the upstream manifest and requested chunk
	// bodies. Content hashes verify each body before it enters the local store.
	for _, rawID := range missing {
		id := digest(rawID)
		body := upstream.chunks[id]
		if sha256.Sum256(body) != id {
			log.Fatalf("upstream returned invalid chunk %x", id[:4])
		}
		downstream.chunks[id] = bytes.Clone(body)
	}
	rebuilt, err := rebuild(upstream.manifest, downstream.chunks, upstream.digest)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("upstream manifest: %d chunks, %d bytes\n", len(upstream.manifest), len(upstreamBytes))
	fmt.Printf("missing downstream: %d chunks (%s)\n", len(missing), shortIDs(missing))
	fmt.Printf("obsolete downstream: %d chunks (%s)\n", len(obsolete), shortIDs(obsolete))
	fmt.Printf("RIBLT decoded after %d coded cells\n", cells)
	fmt.Printf("transferred %d chunk bytes instead of %d document bytes\n", totalBytes(missing, upstream.chunks), len(upstreamBytes))
	fmt.Printf("rebuilt document verified: %v\n", bytes.Equal(rebuilt, upstreamBytes))
}

func block(text string) []byte {
	if len(text) > chunkSize {
		panic("example block exceeds chunk size")
	}
	b := make([]byte, chunkSize)
	copy(b, text)
	return b
}

func chunkIDs(chunks map[digest][]byte) [][]byte {
	ids := make([][]byte, 0, len(chunks))
	for id := range chunks {
		ids = append(ids, bytes.Clone(id[:]))
	}
	return ids
}

func reconcile(codec riblt.BytesCodec, upstream, downstream [][]byte) ([][]byte, [][]byte, int, error) {
	enc, err := riblt.NewEncoder[[]byte](codec)
	if err != nil {
		return nil, nil, 0, err
	}
	dec, err := riblt.NewDecoder[[]byte](codec)
	if err != nil {
		return nil, nil, 0, err
	}
	for _, id := range upstream {
		if err := enc.Add(id); err != nil {
			return nil, nil, 0, err
		}
	}
	for _, id := range downstream {
		if err := dec.AddLocal(id); err != nil {
			return nil, nil, 0, err
		}
	}
	for cells := 1; cells <= 100; cells++ {
		coded, err := enc.Next()
		if err != nil {
			return nil, nil, 0, err
		}
		if err := dec.AddCoded(coded); err != nil {
			return nil, nil, 0, err
		}
		if err := dec.TryDecode(); err != nil {
			return nil, nil, 0, err
		}
		if dec.Complete() {
			return symbols(dec.Remote()), symbols(dec.Local()), cells, nil
		}
	}
	return nil, nil, 0, fmt.Errorf("decode did not complete")
}

func symbols(in []riblt.HashedSymbol[[]byte]) [][]byte {
	out := make([][]byte, len(in))
	for i := range in {
		out[i] = in[i].Symbol
	}
	return out
}

func shortIDs(ids [][]byte) string {
	short := make([]string, len(ids))
	for i, id := range ids {
		short[i] = fmt.Sprintf("%x", id[:4])
	}
	sort.Strings(short)
	return fmt.Sprint(short)
}

func totalBytes(ids [][]byte, chunks map[digest][]byte) int {
	total := 0
	for _, rawID := range ids {
		total += len(chunks[digest(rawID)])
	}
	return total
}
