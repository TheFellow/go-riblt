// Command documents reconciles small text documents as fixed-width byte slots.
package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"sort"

	"github.com/TheFellow/go-riblt"
)

const (
	documentWidth = 256
	headerWidth   = 2
	maxTextBytes  = documentWidth - headerWidth
)

func encodeDocument(text string) ([]byte, error) {
	if len(text) > maxTextBytes {
		return nil, fmt.Errorf("document is %d bytes; maximum is %d", len(text), maxTextBytes)
	}
	slot := make([]byte, documentWidth)
	binary.LittleEndian.PutUint16(slot, uint16(len(text)))
	copy(slot[headerWidth:], text)
	return slot, nil
}

func decodeDocument(slot []byte) (string, error) {
	if len(slot) != documentWidth {
		return "", fmt.Errorf("document slot is %d bytes; want %d", len(slot), documentWidth)
	}
	n := int(binary.LittleEndian.Uint16(slot))
	if n > maxTextBytes {
		return "", fmt.Errorf("encoded length %d exceeds maximum %d", n, maxTextBytes)
	}
	return string(slot[headerWidth : headerWidth+n]), nil
}

func main() {
	key := []byte("example-only-key-change-in-production")
	codec, err := riblt.NewBytesCodec(documentWidth, key)
	if err != nil {
		log.Fatal(err)
	}

	upstream := mustEncode(
		"RIBLT streams coded cells until the receiver can decode.",
		"Go generics let the algorithm remain independent of its symbols.",
		"This paragraph exists only upstream.",
	)
	downstream := mustEncode(
		"RIBLT streams coded cells until the receiver can decode.",
		"Go generics separate the algorithm from its symbols.",
	)

	add, remove, cells, err := reconcile(codec, upstream, downstream)
	if err != nil {
		log.Fatal(err)
	}
	addText := mustDecode(add)
	removeText := mustDecode(remove)
	sort.Strings(addText)
	sort.Strings(removeText)

	fmt.Println("documents to add:")
	for _, text := range addText {
		fmt.Printf("  %q\n", text)
	}
	fmt.Println("documents to remove:")
	for _, text := range removeText {
		fmt.Printf("  %q\n", text)
	}
	fmt.Printf("decoded after %d coded cells\n", cells)
}

func mustEncode(texts ...string) [][]byte {
	out := make([][]byte, len(texts))
	for i, text := range texts {
		var err error
		out[i], err = encodeDocument(text)
		if err != nil {
			panic(err)
		}
	}
	return out
}

func mustDecode(slots [][]byte) []string {
	out := make([]string, len(slots))
	for i, slot := range slots {
		var err error
		out[i], err = decodeDocument(slot)
		if err != nil {
			panic(err)
		}
	}
	return out
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
	for _, document := range upstream {
		if err := enc.Add(document); err != nil {
			return nil, nil, 0, err
		}
	}
	for _, document := range downstream {
		if err := dec.AddLocal(document); err != nil {
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
			return byteSymbols(dec.Remote()), byteSymbols(dec.Local()), cells, nil
		}
	}
	return nil, nil, 0, fmt.Errorf("decode did not complete")
}

func byteSymbols(in []riblt.HashedSymbol[[]byte]) [][]byte {
	out := make([][]byte, len(in))
	for i := range in {
		out[i] = in[i].Symbol
	}
	return out
}
