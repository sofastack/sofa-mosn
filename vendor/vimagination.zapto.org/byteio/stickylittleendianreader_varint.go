package byteio

// ReadUintX reads an unsinged integer that was encoded using a variable number
// of bytes
func (e *StickyLittleEndianReader) ReadUintX() uint64 {
	var (
		n   int
		val uint64
	)
	for n < 9 {
		c, _ := e.ReadByte()
		val += uint64(c&0xff) << uint(n*7)
		n++
		if c&0x80 == 0 {
			break
		}
	}
	return val
}

// ReadIntX reads an integer that was encoded using a variable number of bytes
func (e *StickyLittleEndianReader) ReadIntX() int64 {
	return unzigzag(e.ReadUintX())
}
