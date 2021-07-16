package byteio

// ReadUintX reads an unsinged integer that was encoded using a variable number
// of bytes
func (e *BigEndianReader) ReadUintX() (uint64, int, error) {
	c, err := e.ReadByte()
	if err != nil {
		return 0, 0, err
	}
	val := uint64(c) & 0x7f
	n := 1
	for ; c&0x80 != 0 && n < 9; n++ {
		c, err = e.ReadByte()
		if err != nil {
			return 0, n, err
		}
		val++
		if n == 8 && c&0x80 > 0 {
			val = (val << 8) | uint64(c)
		} else {
			val = (val << 7) | uint64(c&0x7f)
		}
	}
	return val, n, nil
}

// ReadIntX reads an integer that was encoded using a variable number of bytes
func (e *BigEndianReader) ReadIntX() (int64, int, error) {
	i, n, err := e.ReadUintX()
	return unzigzag(i), n, err
}
