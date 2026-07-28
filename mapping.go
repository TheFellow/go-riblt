package riblt

import (
	"math"
)

const mappingMultiplier uint64 = 0xda942042e4dd58b5

type randomMapping struct {
	state uint64
	last  uint64
}

func newMapping(seed uint64) randomMapping {
	// Zero is the absorbing state of the multiplicative generator.
	if seed == 0 {
		seed = 0x9e3779b97f4a7c15
	}
	return randomMapping{state: seed}
}

// next implements the gap distribution from the RIBLT paper. Index zero is
// always selected; subsequent selections become progressively sparser.
func (m *randomMapping) next() (uint64, error) {
	m.state *= mappingMultiplier
	factor := (float64(m.last) + 1.5) * (math.Ldexp(1, 32)/math.Sqrt(float64(m.state)+1) - 1)
	gap := math.Ceil(factor)
	if gap < 1 {
		gap = 1
	}
	if gap > float64(^uint64(0)-m.last) {
		return 0, ErrMappingOverflow
	}
	m.last += uint64(gap)
	return m.last, nil
}
