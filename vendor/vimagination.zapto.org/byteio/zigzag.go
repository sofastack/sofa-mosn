package byteio

func zigzag(d int64) uint64 {
	e := uint64(d)
	if d < 0 {
		e = ((^e) << 1) | 1
	} else {
		e <<= 1
	}
	return e
}

func unzigzag(i uint64) int64 {
	j := i >> 1
	if i&1 == 1 {
		j = ^j
	}
	return int64(j)
}
