package byteio

// maxBENineByte is the largest number that would be stored in 63 bits with
// this varint format without using a special case.
// This is needed to we can shift the bits to store the last bit within the
// carry bit of the ninth byte
// The byte structure would be as follows: -
// 	[0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f]
// With the high bit of each of the first 8 bytes being the carry bit, this
// would lead to a number of 0xffffffffffffff7f, except that the carry bit also
// adds one to the value of each byte.
// Thus, we end up with a value of 0x80 for the first eight bytes, and 0x7f for
// the last.
// Numbers greater than this value use the high bit of the ninth byte to store
// the eighth bit of the number, shifting the remaining bits down by one.
const maxBENineByte = 0x80<<56 | 0x80<<49 | 0x80<<42 | 0x80<<35 | 0x80<<28 | 0x80<<21 | 0x80<<14 | 0x80<<7 | 0x7f

// WriteUintX writes the unsigned integer using a variable number of bytes
func (e *BigEndianWriter) WriteUintX(d uint64) (int, error) {
	pos := 8
	if d > maxBENineByte {
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
	return e.Write(e.buffer[pos:])
}

// WriteIntX writes the integer using a variable number of bytes
func (e *BigEndianWriter) WriteIntX(d int64) (int, error) {
	return e.WriteUintX(zigzag(d))
}
