package riblt

import (
	"container/heap"
	"math"
)

type queuedMapping struct {
	source int
	coded  uint64
}
type mappingHeap []queuedMapping

func (h mappingHeap) Len() int           { return len(h) }
func (h mappingHeap) Less(i, j int) bool { return h[i].coded < h[j].coded }
func (h mappingHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *mappingHeap) Push(x any)        { *h = append(*h, x.(queuedMapping)) }
func (h *mappingHeap) Pop() any          { old := *h; x := old[len(old)-1]; *h = old[:len(old)-1]; return x }

type identity struct{ mapping, checksum uint64 }

type codingWindow[T any] struct {
	codec    Codec[T]
	symbols  []HashedSymbol[T]
	mappings []randomMapping
	queue    mappingHeap
	next     uint64
}

func (w *codingWindow[T]) add(s HashedSymbol[T], m randomMapping) {
	w.symbols = append(w.symbols, s)
	w.mappings = append(w.mappings, m)
	heap.Push(&w.queue, queuedMapping{len(w.symbols) - 1, m.last})
}

func (w *codingWindow[T]) apply(c CodedSymbol[T], direction int64) (CodedSymbol[T], error) {
	for len(w.queue) > 0 && w.queue[0].coded == w.next {
		q := w.queue[0]
		if direction > 0 && c.Count == math.MaxInt64 || direction < 0 && c.Count == math.MinInt64 {
			return c, ErrCountOverflow
		}
		c = apply(w.codec, c, w.symbols[q.source], direction)
		n, err := w.mappings[q.source].next()
		if err != nil {
			return c, err
		}
		w.queue[0].coded = n
		heap.Fix(&w.queue, 0)
	}
	if w.next == math.MaxUint64 {
		return c, ErrMappingOverflow
	}
	w.next++
	return c, nil
}

func (w *codingWindow[T]) reset() {
	w.symbols = w.symbols[:0]
	w.mappings = w.mappings[:0]
	w.queue = w.queue[:0]
	w.next = 0
}
