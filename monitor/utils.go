package monitor

func minBTCHeight(h1, h2 uint64) uint64 {
	if h1 > h2 {
		return h2
	}

	return h1
}
