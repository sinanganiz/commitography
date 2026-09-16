package core

import (
	"math"
	"sort"
)

// Round returns v rounded to the given number of decimal places. Every ratio
// and average in the report is rounded before serialization so two runs on the
// same repository produce byte-identical JSON.
func Round(v float64, places int) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	factor := math.Pow(10, float64(places))
	return math.Round(v*factor) / factor
}

// Mean is the arithmetic mean of values, or 0 for none.
func Mean(values []int) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0
	for _, v := range values {
		sum += v
	}
	return float64(sum) / float64(len(values))
}

// Median sorts a copy, so the caller's slice keeps whatever order it had.
func Median(values []int) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int(nil), values...)
	sort.Ints(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return float64(sorted[mid])
	}
	return float64(sorted[mid-1]+sorted[mid]) / 2
}
