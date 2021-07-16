package byteio

// ReadUintX reads an unsinged integer that was encoded using a variable number
// of bytes
func (e *StickyBigEndianReader) ReadUintX() uint64 {
	c, _ := e.ReadByte()
	val := uint64(c) & 0x7f
	n := 1
	for ; c&0x80 != 0 && n < 9; n++ {
		c, _ = e.ReadByte()
		val++
		if n == 8 {
			val = (val << 8) | uint64(c)
		} else {
			val = (val << 7) | uint64(c&0x7f)
		}
	}
	return val
}

// ReadIntX reads an integer that was encoded using a variable number of bytes
func (e *StickyBigEndianReader) ReadIntX() int64 {
	return unzigzag(e.ReadUintX())
}
