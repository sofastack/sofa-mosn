package byteio

// WriteUintX writes the unsigned integer using a variable number of bytes
// This variable encoding uses the first seven bits of each byte to encode
// successive bits of the number, and the last bit to indicate that another
// byte is to be read. The exception to this is the ninth byte, which uses its
// highest bit to store the 64th bit of the number.
// To allow for unique encodings of all numbers, as well as having a smaller
// encoding for some numbers, the carry bit is also part of the number.
// If there is another byte of the encoding then, by definition, the number
// must be higher than what has currently been decoded. Thus, the next byte
// starts at the next number.
// For example, without this modification, the number 5 could be encoded as any
// of the following: -
// 0x05
// 0x85, 0x00
// 0x85, 0x80, 0x00
// 0x85, 0x80, 0x80, 0x00
// etc.
//
// By having the carry bit do double duty, adding 1 to the value of all bytes
// where the carry bit is set, the only valid encoding of 5 becomes 0x05.
func (e *LittleEndianWriter) WriteUintX(d uint64) (int, error) {
	var pos int
	for ; d > 127 && pos < 8; pos++ {
		e.buffer[pos] = byte(d&0x7f) | 0x80
		d >>= 7
		d--
	}
	e.buffer[pos] = byte(d)
	return e.Write(e.buffer[:pos+1])
}

// WriteIntX writes the integer using a variable number of bytes
func (e *LittleEndianWriter) WriteIntX(d int64) (int, error) {
	return e.WriteUintX(zigzag(d))
}
