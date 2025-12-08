package conv

// SafeInt32 safely converts int64 to int32, clamping to int32 min/max on overflow.
func SafeInt32(val int64) int32 {
	const (
		maxInt32 = 1<<31 - 1
		minInt32 = -1 << 31
	)
	if val > maxInt32 {
		return maxInt32
	}
	if val < minInt32 {
		return minInt32
	}

	return int32(val)
}

// SafeUint32 safely converts uint64 to uint32, clamping to uint32 max on overflow.
func SafeUint32(val uint64) uint32 {
	const maxUint32 = 1<<32 - 1
	if val > maxUint32 {
		return maxUint32
	}

	return uint32(val)
}
