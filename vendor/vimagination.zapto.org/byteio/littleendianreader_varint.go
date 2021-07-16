package byteio

// ReadUintX reads an unsinged integer that was encoded using a variable number
// of bytes
func (e *LittleEndianReader) ReadUintX() (uint64, int, error) {
	var (
		n   int
		val uint64
	)
	for n < 9 {
		c, err := e.ReadByte()
		if err != nil {
			return 0, n, err
		}
		val += uint64(c&0xff) << uint(n*7)
		n++
		if c&0x80 == 0 {
			break
		}
	}
	return val, n, nil
}

// ReadIntX reads an integer that was encoded using a variable number of bytes
func (e *LittleEndianReader) ReadIntX() (int64, int, error) {
	i, n, err := e.ReadUintX()
	return unzigzag(i), n, err
}
