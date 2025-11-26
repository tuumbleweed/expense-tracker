package util

import "cmp"

// Clamp clamps val to the range [min, max] for any ordered type.
func Clamp[T cmp.Ordered](val, min, max T) T {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}
