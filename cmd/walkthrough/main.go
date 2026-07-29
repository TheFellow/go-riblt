// Command walkthrough narrates one small RIBLT reconciliation cell by cell.
package main

import (
	"fmt"
	"log"

	"github.com/TheFellow/go-riblt"
	"github.com/TheFellow/go-riblt/internal/demo"
)

func main() {
	codec := demo.Codec{}
	a, b := demo.Symbol{ID: 101}, demo.Symbol{ID: 103}
	fmt.Println("1. The codec supplies the algebra; RIBLT stays generic.")
	fmt.Printf("   a XOR zero = %s\n", codec.XOR(a, codec.Zero()))
	fmt.Printf("   a XOR a    = %s (the identity)\n", codec.XOR(a, a))
	fmt.Printf("   a XOR b    = %s; XOR that with b = %s\n\n", codec.XOR(a, b), codec.XOR(codec.XOR(a, b), b))

	fmt.Println("2. Placement and validation use independent deterministic hashes.")
	for _, s := range []demo.Symbol{a, b} {
		h, _ := riblt.Hash[demo.Symbol](codec, s)
		fmt.Printf("   %-8s mapping=%016x checksum=%016x\n", s, h.MappingHash, h.Checksum)
	}
	fmt.Println("   The mapping hash seeds a repeatable, increasingly sparse cell sequence.")
	fmt.Println("   The checksum decides whether a count of +1 or -1 is a real singleton.")
	fmt.Println()

	upstream := []uint64{100, 101, 102, 103}
	downstream := []uint64{100, 102, 104}
	encoder := demo.NewEncoder(upstream...)
	decoder := demo.NewDecoder(downstream...)

	fmt.Println("3. The upstream sends cells; the downstream subtracts its set as cells arrive.")
	fmt.Printf("   upstream:   %v\n", upstream)
	fmt.Printf("   downstream: %v\n", downstream)
	fmt.Println("   count +1 peels an upstream-only value; -1 peels a downstream-only value;")
	fmt.Println("   count 0 is resolved once its XOR and checksum are also zero.")

	const maxCells = 100
	used := 0
	for !decoder.Complete() && used < maxCells {
		cell, err := encoder.Next()
		if err != nil {
			log.Fatal(err)
		}
		used++
		fmt.Printf("   cell %2d on wire: symbol=%-8s count=%2d checksum=%016x\n", used-1, cell.Symbol, cell.Count, cell.Checksum)
		if err := decoder.AddCoded(cell); err != nil {
			log.Fatal(err)
		}
		if err := decoder.TryDecode(); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("            peeled: upstream-only=%v downstream-only=%v complete=%v\n",
			demo.IDs(decoder.Remote()), demo.IDs(decoder.Local()), decoder.Complete())
	}
	if !decoder.Complete() {
		log.Fatalf("did not decode after %d cells", used)
	}

	fmt.Println("\n4. Reconciliation is complete.")
	fmt.Printf("   add to downstream:      %v\n", demo.IDs(decoder.Remote()))
	fmt.Printf("   remove from downstream: %v\n", demo.IDs(decoder.Local()))
	fmt.Printf("   communication: %d coded cells, about %d payload bytes in this fixed-width demo\n", used, used*24)
}
