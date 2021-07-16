package byteio

// ReadByte implements the io.ByteReader interface
func (e *BigEndianReader) ReadByte() (byte, error) {
	b, _, err := e.ReadUint8()
	return b, err
}

// WriteByte implements the io.ByteWriter interface
func (e *BigEndianWriter) WriteByte(c byte) error {
	_, err := e.WriteUint8(c)
	return err
}

// ReadByte implements the io.ByteReader interface
func (e *LittleEndianReader) ReadByte() (byte, error) {
	b, _, err := e.ReadUint8()
	return b, err
}

// WriteByte implements the io.ByteWriter interface
func (e *LittleEndianWriter) WriteByte(c byte) error {
	_, err := e.WriteUint8(c)
	return err
}

// ReadByte implements the io.ByteReader interface
func (e *StickyBigEndianReader) ReadByte() (byte, error) {
	return e.ReadUint8(), e.Err
}

// WriteByte implements the io.ByteWriter interface
func (e *StickyBigEndianWriter) WriteByte(c byte) error {
	e.WriteUint8(c)
	return e.Err
}

// ReadByte implements the io.ByteReader interface
func (e *StickyLittleEndianReader) ReadByte() (byte, error) {
	return e.ReadUint8(), e.Err
}

// WriteByte implements the io.ByteWriter interface
func (e *StickyLittleEndianWriter) WriteByte(c byte) error {
	e.WriteUint8(c)
	return e.Err
}
