package riblt

import (
	"fmt"
	"testing"
)

func BenchmarkReconcile(b *testing.B) {
	for _, difference := range []int{1, 4, 32, 128, 1024} {
		b.Run(fmt.Sprintf("difference=%d", difference), func(b *testing.B) {
			var total int64
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				left, right := (difference+1)/2, difference/2
				a, local := make([]uint64, 10000+left), make([]uint64, 10000+right)
				for j := 0; j < 10000; j++ {
					a[j], local[j] = uint64(j), uint64(j)
				}
				for j := 0; j < left; j++ {
					a[10000+j] = uint64(20000 + j)
				}
				for j := 0; j < right; j++ {
					local[10000+j] = uint64(30000 + j)
				}
				_, sent := reconcile(b, a, local)
				total += int64(sent)
			}
			b.ReportMetric(float64(total)/(float64(b.N)*float64(difference)), "coded/difference")
		})
	}
}
func BenchmarkMapping(b *testing.B) {
	for i := 0; i < b.N; i++ {
		m := newMapping(uint64(i) + 123456789)
		if _, err := m.next(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUint64CodecHash(b *testing.B) {
	c, err := NewUint64Codec(testKey)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if c.MappingHash(uint64(i)) == c.Checksum(uint64(i)) {
			b.Fatal("domain collision")
		}
	}
}
