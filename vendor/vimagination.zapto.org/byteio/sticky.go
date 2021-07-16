package byteio

// StickyReader will wrap an EndianReader and record all bytes read and errors
// received.
// Byte counts and errors will not be returned from any method (except Read so
// it still counts as an io.Reader), but can be retrieved from this type.
// All methods will be a no-op after an error has been returned, returning 0,
// unless that error is cleared on the type
type StickyReader struct {
	Reader EndianReader
	Err    error
	Count  int64
}

// Read will do a simple byte read from the underlying io.Reader.
func (s *StickyReader) Read(b []byte) (int, error) {
	if s.Err != nil {
		return 0, s.Err
	}
	n, err := s.Reader.Read(b)
	s.Err = err
	s.Count += int64(n)
	return n, err
}

// ReadUint8 will read a uint8 from the underlying reader
func (s *StickyReader) ReadUint8() uint8 {
	if s.Err != nil {
		return 0
	}
	i, n, err := s.Reader.ReadUint8()
	s.Err = err
	s.Count += int64(n)
	return i
}

// ReadInt8 will read a int8 from the underlying reader
func (s *StickyReader) ReadInt8() int8 {
	if s.Err != nil {
		return 0
	}
	i, n, err := s.Reader.ReadInt8()
	s.Err = err
	s.Count += int64(n)
	return i
}

// ReadUint16 will read a uint16 from the underlying reader
func (s *StickyReader) ReadUint16() uint16 {
	if s.Err != nil {
		return 0
	}
	i, n, err := s.Reader.ReadUint16()
	s.Err = err
	s.Count += int64(n)
	return i
}

// ReadInt16 will read a int16 from the underlying reader
func (s *StickyReader) ReadInt16() int16 {
	if s.Err != nil {
		return 0
	}
	i, n, err := s.Reader.ReadInt16()
	s.Err = err
	s.Count += int64(n)
	return i
}

// ReadUint32 will read a uint32 from the underlying reader
func (s *StickyReader) ReadUint32() uint32 {
	if s.Err != nil {
		return 0
	}
	i, n, err := s.Reader.ReadUint32()
	s.Err = err
	s.Count += int64(n)
	return i
}

// ReadInt32 will read a int32 from the underlying reader
func (s *StickyReader) ReadInt32() int32 {
	if s.Err != nil {
		return 0
	}
	i, n, err := s.Reader.ReadInt32()
	s.Err = err
	s.Count += int64(n)
	return i
}

// ReadUint64 will read a uint64 from the underlying reader
func (s *StickyReader) ReadUint64() uint64 {
	if s.Err != nil {
		return 0
	}
	i, n, err := s.Reader.ReadUint64()
	s.Err = err
	s.Count += int64(n)
	return i
}

// ReadInt64 will read a int64 from the underlying reader
func (s *StickyReader) ReadInt64() int64 {
	if s.Err != nil {
		return 0
	}
	i, n, err := s.Reader.ReadInt64()
	s.Err = err
	s.Count += int64(n)
	return i
}

// ReadFloat32 will read a float32 from the underlying reader
func (s *StickyReader) ReadFloat32() float32 {
	if s.Err != nil {
		return 0
	}
	i, n, err := s.Reader.ReadFloat32()
	s.Err = err
	s.Count += int64(n)
	return i
}

// ReadFloat64 will read a float64 from the underlying reader
func (s *StickyReader) ReadFloat64() float64 {
	if s.Err != nil {
		return 0
	}
	i, n, err := s.Reader.ReadFloat64()
	s.Err = err
	s.Count += int64(n)
	return i
}

// StickyWriter will wrap an EndianWriter and record all bytes written and
// errors received.
// Byte counts and errors will not be returned from any method (except Write,
// so it still counts as an io.Writer), but can be retrieved from this type.
// All methods will be a no-op after an error has been returned, unless that
// error is cleared on the type
type StickyWriter struct {
	Writer EndianWriter
	Err    error
	Count  int64
}

// Write will do a simple byte write to the underlying io.Writer.
func (s *StickyWriter) Write(p []byte) (int, error) {
	if s.Err != nil {
		return 0, s.Err
	}
	n, err := s.Writer.Write(p)
	s.Err = err
	s.Count += int64(n)
	return n, err
}

// WriteUint8 will write a uint8 to the underlying writer
func (s *StickyWriter) WriteUint8(i uint8) {
	if s.Err != nil {
		return
	}
	n, err := s.Writer.WriteUint8(i)
	s.Err = err
	s.Count += int64(n)
}

// WriteInt8 will write a int8 to the underlying writer
func (s *StickyWriter) WriteInt8(i int8) {
	if s.Err != nil {
		return
	}
	n, err := s.Writer.WriteInt8(i)
	s.Err = err
	s.Count += int64(n)
}

// WriteUint16 will write a uint16 to the underlying writer
func (s *StickyWriter) WriteUint16(i uint16) {
	if s.Err != nil {
		return
	}
	n, err := s.Writer.WriteUint16(i)
	s.Err = err
	s.Count += int64(n)
}

// WriteInt16 will write a int16 to the underlying writer
func (s *StickyWriter) WriteInt16(i int16) {
	if s.Err != nil {
		return
	}
	n, err := s.Writer.WriteInt16(i)
	s.Err = err
	s.Count += int64(n)
}

// WriteUint32 will write a uint32 to the underlying writer
func (s *StickyWriter) WriteUint32(i uint32) {
	if s.Err != nil {
		return
	}
	n, err := s.Writer.WriteUint32(i)
	s.Err = err
	s.Count += int64(n)
}

// WriteInt32 will write a int32 to the underlying writer
func (s *StickyWriter) WriteInt32(i int32) {
	if s.Err != nil {
		return
	}
	n, err := s.Writer.WriteInt32(i)
	s.Err = err
	s.Count += int64(n)
}

// WriteUint64 will write a uint64 to the underlying writer
func (s *StickyWriter) WriteUint64(i uint64) {
	if s.Err != nil {
		return
	}
	n, err := s.Writer.WriteUint64(i)
	s.Err = err
	s.Count += int64(n)
}

// WriteInt64 will write a int64 to the underlying writer
func (s *StickyWriter) WriteInt64(i int64) {
	if s.Err != nil {
		return
	}
	n, err := s.Writer.WriteInt64(i)
	s.Err = err
	s.Count += int64(n)
}

// WriteFloat32 will write a float32 to the underlying writer
func (s *StickyWriter) WriteFloat32(i float32) {
	if s.Err != nil {
		return
	}
	n, err := s.Writer.WriteFloat32(i)
	s.Err = err
	s.Count += int64(n)
}

// WriteFloat64 will write a float64 to the underlying writer
func (s *StickyWriter) WriteFloat64(i float64) {
	if s.Err != nil {
		return
	}
	n, err := s.Writer.WriteFloat64(i)
	s.Err = err
	s.Count += int64(n)
}
