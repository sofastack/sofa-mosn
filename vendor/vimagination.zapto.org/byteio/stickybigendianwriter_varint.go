package byteio

// WriteUintX writes the unsigned integer using a variable number of bytes
func (e *StickyBigEndianWriter) WriteUintX(d uint64) {
	pos := 8
	if d > 9295997013522923647 {
		e.buffer[8] = byte(d)
		d >>= 8
	} else {
		e.buffer[8] = byte(d & 0x7f)
		d >>= 7
	}
	for ; d > 0; d >>= 7 {
		pos--
		d--
		e.buffer[pos] = byte(d&0x7f) | 0x80
	}
	e.Write(e.buffer[pos:])
}

// WriteIntX writes the integer using a variable number of bytes
func (e *StickyBigEndianWriter) WriteIntX(d int64) {
	e.WriteUintX(zigzag(d))
}
