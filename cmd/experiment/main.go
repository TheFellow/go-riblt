// Command experiment measures how the streamed prefix follows set difference,
// rather than total set size. It is illustrative, not a Go benchmark harness.
package main

import (
	"fmt"
	"log"

	"github.com/TheFellow/go-riblt/internal/demo"
)

func main() {
	fmt.Println("set size  difference  cells sent  payload bytes  cells/difference")
	for _, tc := range []struct {
		size, difference int
	}{
		{1_000, 2}, {1_000, 10}, {1_000, 50},
		{10_000, 2}, {10_000, 10}, {10_000, 50},
	} {
		cells, err := reconcile(tc.size, tc.difference)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%8d  %10d  %10d  %13d  %16.2f\n",
			tc.size, tc.difference, cells, cells*24, float64(cells)/float64(tc.difference))
	}
	fmt.Println("\nEach coded cell has three fixed-width fields here (24 payload bytes).")
	fmt.Println("Real wire size also depends on framing and serialization.")
}

// The sets share size-difference/2 values. The remainder is split equally, so
// difference is the full symmetric difference and both sides have equal size.
func reconcile(size, difference int) (int, error) {
	shared := size - difference/2
	upstream := make([]uint64, 0, size)
	downstream := make([]uint64, 0, size)
	for id := 1; id <= shared; id++ {
		upstream = append(upstream, uint64(id))
		downstream = append(downstream, uint64(id))
	}
	for i := 0; i < difference/2; i++ {
		upstream = append(upstream, uint64(size+i+1))
		downstream = append(downstream, uint64(2*size+i+1))
	}

	encoder := demo.NewEncoder(upstream...)
	decoder := demo.NewDecoder(downstream...)
	cells := 0
	for cell, err := range encoder.Cells() {
		if err != nil {
			return 0, err
		}
		cells++
		if err := decoder.AddCoded(cell); err != nil {
			return 0, err
		}
		if err := decoder.TryDecode(); err != nil {
			return 0, err
		}
		if decoder.Complete() {
			return cells, nil
		}
	}
	return 0, fmt.Errorf("encoder stream ended before decoding completed")
}
