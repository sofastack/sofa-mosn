package byteio

// File automatically generated with ./gen.sh

import (
	"io"
	"math"
)

// StickyLittleEndianReader wraps a io.Reader to provide methods
// to make it easier to Read fundamental types
type StickyLittleEndianReader struct {
	io.Reader
	buffer [9]byte
	Err    error
	Count  int64
}

// Read implements the io.Reader interface
func (e *StickyLittleEndianReader) Read(p []byte) (int, error) {
	if e.Err != nil {
		return 0, e.Err
	}
	var n int
	n, e.Err = e.Reader.Read(p)
	e.Count += int64(n)
	return n, e.Err
}

// ReadInt8 Reads a 8 bit int as a int8 using the underlying io.Reader
// Any errors and the running byte read count are stored instead or returned
func (e *StickyLittleEndianReader) ReadInt8() int8 {
	if e.Err != nil {
		return 0
	}
	n, err := io.ReadFull(e.Reader, e.buffer[:1])
	e.Count += int64(n)
	if err != nil {
		e.Err = err
		return 0
	}
	return int8(e.buffer[0])
}

// ReadInt16 Reads a 16 bit int as a int16 using the underlying io.Reader
// Any errors and the running byte read count are stored instead or returned
func (e *StickyLittleEndianReader) ReadInt16() int16 {
	if e.Err != nil {
		return 0
	}
	n, err := io.ReadFull(e.Reader, e.buffer[:2])
	e.Count += int64(n)
	if err != nil {
		e.Err = err
		return 0
	}
	return int16(uint16(e.buffer[0]) | uint16(e.buffer[1])<<8)
}

// ReadInt24 Reads a 24 bit int as a int32 using the underlying io.Reader
// Any errors and the running byte read count are stored instead or returned
func (e *StickyLittleEndianReader) ReadInt24() int32 {
	if e.Err != nil {
		return 0
	}
	n, err := io.ReadFull(e.Reader, e.buffer[:3])
	e.Count += int64(n)
	if err != nil {
		e.Err = err
		return 0
	}
	return int32(uint32(e.buffer[0]) | uint32(e.buffer[1])<<8 | uint32(e.buffer[2])<<16)
}

// ReadInt32 Reads a 32 bit int as a int32 using the underlying io.Reader
// Any errors and the running byte read count are stored instead or returned
func (e *StickyLittleEndianReader) ReadInt32() int32 {
	if e.Err != nil {
		return 0
	}
	n, err := io.ReadFull(e.Reader, e.buffer[:4])
	e.Count += int64(n)
	if err != nil {
		e.Err = err
		return 0
	}
	return int32(uint32(e.buffer[0]) | uint32(e.buffer[1])<<8 | uint32(e.buffer[2])<<16 | uint32(e.buffer[3])<<24)
}

// ReadInt40 Reads a 40 bit int as a int64 using the underlying io.Reader
// Any errors and the running byte read count are stored instead or returned
func (e *StickyLittleEndianReader) ReadInt40() int64 {
	if e.Err != nil {
		return 0
	}
	n, err := io.ReadFull(e.Reader, e.buffer[:5])
	e.Count += int64(n)
	if err != nil {
		e.Err = err
		return 0
	}
	return int64(uint64(e.buffer[0]) | uint64(e.buffer[1])<<8 | uint64(e.buffer[2])<<16 | uint64(e.buffer[3])<<24 | uint64(e.buffer[4])<<32)
}

// ReadInt48 Reads a 48 bit int as a int64 using the underlying io.Reader
// Any errors and the running byte read count are stored instead or returned
func (e *StickyLittleEndianReader) ReadInt48() int64 {
	if e.Err != nil {
		return 0
	}
	n, err := io.ReadFull(e.Reader, e.buffer[:6])
	e.Count += int64(n)
	if err != nil {
		e.Err = err
		return 0
	}
	return int64(uint64(e.buffer[0]) | uint64(e.buffer[1])<<8 | uint64(e.buffer[2])<<16 | uint64(e.buffer[3])<<24 | uint64(e.buffer[4])<<32 | uint64(e.buffer[5])<<40)
}

// ReadInt56 Reads a 56 bit int as a int64 using the underlying io.Reader
// Any errors and the running byte read count are stored instead or returned
func (e *StickyLittleEndianReader) ReadInt56() int64 {
	if e.Err != nil {
		return 0
	}
	n, err := io.ReadFull(e.Reader, e.buffer[:7])
	e.Count += int64(n)
	if err != nil {
		e.Err = err
		return 0
	}
	return int64(uint64(e.buffer[0]) | uint64(e.buffer[1])<<8 | uint64(e.buffer[2])<<16 | uint64(e.buffer[3])<<24 | uint64(e.buffer[4])<<32 | uint64(e.buffer[5])<<40 | uint64(e.buffer[6])<<48)
}

// ReadInt64 Reads a 64 bit int as a int64 using the underlying io.Reader
// Any errors and the running byte read count are stored instead or returned
func (e *StickyLittleEndianReader) ReadInt64() int64 {
	if e.Err != nil {
		return 0
	}
	n, err := io.ReadFull(e.Reader, e.buffer[:8])
	e.Count += int64(n)
	if err != nil {
		e.Err = err
		return 0
	}
	return int64(uint64(e.buffer[0]) | uint64(e.buffer[1])<<8 | uint64(e.buffer[2])<<16 | uint64(e.buffer[3])<<24 | uint64(e.buffer[4])<<32 | uint64(e.buffer[5])<<40 | uint64(e.buffer[6])<<48 | uint64(e.buffer[7])<<56)
}

// ReadUint8 Reads a 8 bit uint as a uint8 using the underlying io.Reader
// Any errors and the running byte read count are stored instead or returned
func (e *StickyLittleEndianReader) ReadUint8() uint8 {
	if e.Err != nil {
		return 0
	}
	n, err := io.ReadFull(e.Reader, e.buffer[:1])
	e.Count += int64(n)
	if err != nil {
		e.Err = err
		return 0
	}
	return e.buffer[0]
}

// ReadUint16 Reads a 16 bit uint as a uint16 using the underlying io.Reader
// Any errors and the running byte read count are stored instead or returned
func (e *StickyLittleEndianReader) ReadUint16() uint16 {
	if e.Err != nil {
		return 0
	}
	n, err := io.ReadFull(e.Reader, e.buffer[:2])
	e.Count += int64(n)
	if err != nil {
		e.Err = err
		return 0
	}
	return uint16(e.buffer[0]) | uint16(e.buffer[1])<<8
}

// ReadUint24 Reads a 24 bit uint as a uint32 using the underlying io.Reader
// Any errors and the running byte read count are stored instead or returned
func (e *StickyLittleEndianReader) ReadUint24() uint32 {
	if e.Err != nil {
		return 0
	}
	n, err := io.ReadFull(e.Reader, e.buffer[:3])
	e.Count += int64(n)
	if err != nil {
		e.Err = err
		return 0
	}
	return uint32(e.buffer[0]) | uint32(e.buffer[1])<<8 | uint32(e.buffer[2])<<16
}

// ReadUint32 Reads a 32 bit uint as a uint32 using the underlying io.Reader
// Any errors and the running byte read count are stored instead or returned
func (e *StickyLittleEndianReader) ReadUint32() uint32 {
	if e.Err != nil {
		return 0
	}
	n, err := io.ReadFull(e.Reader, e.buffer[:4])
	e.Count += int64(n)
	if err != nil {
		e.Err = err
		return 0
	}
	return uint32(e.buffer[0]) | uint32(e.buffer[1])<<8 | uint32(e.buffer[2])<<16 | uint32(e.buffer[3])<<24
}

// ReadUint40 Reads a 40 bit uint as a uint64 using the underlying io.Reader
// Any errors and the running byte read count are stored instead or returned
func (e *StickyLittleEndianReader) ReadUint40() uint64 {
	if e.Err != nil {
		return 0
	}
	n, err := io.ReadFull(e.Reader, e.buffer[:5])
	e.Count += int64(n)
	if err != nil {
		e.Err = err
		return 0
	}
	return uint64(e.buffer[0]) | uint64(e.buffer[1])<<8 | uint64(e.buffer[2])<<16 | uint64(e.buffer[3])<<24 | uint64(e.buffer[4])<<32
}

// ReadUint48 Reads a 48 bit uint as a uint64 using the underlying io.Reader
// Any errors and the running byte read count are stored instead or returned
func (e *StickyLittleEndianReader) ReadUint48() uint64 {
	if e.Err != nil {
		return 0
	}
	n, err := io.ReadFull(e.Reader, e.buffer[:6])
	e.Count += int64(n)
	if err != nil {
		e.Err = err
		return 0
	}
	return uint64(e.buffer[0]) | uint64(e.buffer[1])<<8 | uint64(e.buffer[2])<<16 | uint64(e.buffer[3])<<24 | uint64(e.buffer[4])<<32 | uint64(e.buffer[5])<<40
}

// ReadUint56 Reads a 56 bit uint as a uint64 using the underlying io.Reader
// Any errors and the running byte read count are stored instead or returned
func (e *StickyLittleEndianReader) ReadUint56() uint64 {
	if e.Err != nil {
		return 0
	}
	n, err := io.ReadFull(e.Reader, e.buffer[:7])
	e.Count += int64(n)
	if err != nil {
		e.Err = err
		return 0
	}
	return uint64(e.buffer[0]) | uint64(e.buffer[1])<<8 | uint64(e.buffer[2])<<16 | uint64(e.buffer[3])<<24 | uint64(e.buffer[4])<<32 | uint64(e.buffer[5])<<40 | uint64(e.buffer[6])<<48
}

// ReadUint64 Reads a 64 bit uint as a uint64 using the underlying io.Reader
// Any errors and the running byte read count are stored instead or returned
func (e *StickyLittleEndianReader) ReadUint64() uint64 {
	if e.Err != nil {
		return 0
	}
	n, err := io.ReadFull(e.Reader, e.buffer[:8])
	e.Count += int64(n)
	if err != nil {
		e.Err = err
		return 0
	}
	return uint64(e.buffer[0]) | uint64(e.buffer[1])<<8 | uint64(e.buffer[2])<<16 | uint64(e.buffer[3])<<24 | uint64(e.buffer[4])<<32 | uint64(e.buffer[5])<<40 | uint64(e.buffer[6])<<48 | uint64(e.buffer[7])<<56
}

// ReadFloat32 Reads a 32 bit float as a float32 using the underlying io.Reader
// Any errors and the running byte read count are stored instead or returned
func (e *StickyLittleEndianReader) ReadFloat32() float32 {
	if e.Err != nil {
		return 0
	}
	n, err := io.ReadFull(e.Reader, e.buffer[:4])
	e.Count += int64(n)
	if err != nil {
		e.Err = err
		return 0
	}
	return math.Float32frombits(uint32(e.buffer[0]) | uint32(e.buffer[1])<<8 | uint32(e.buffer[2])<<16 | uint32(e.buffer[3])<<24)
}

// ReadFloat64 Reads a 64 bit float as a float64 using the underlying io.Reader
// Any errors and the running byte read count are stored instead or returned
func (e *StickyLittleEndianReader) ReadFloat64() float64 {
	if e.Err != nil {
		return 0
	}
	n, err := io.ReadFull(e.Reader, e.buffer[:8])
	e.Count += int64(n)
	if err != nil {
		e.Err = err
		return 0
	}
	return math.Float64frombits(uint64(e.buffer[0]) | uint64(e.buffer[1])<<8 | uint64(e.buffer[2])<<16 | uint64(e.buffer[3])<<24 | uint64(e.buffer[4])<<32 | uint64(e.buffer[5])<<40 | uint64(e.buffer[6])<<48 | uint64(e.buffer[7])<<56)
}

// ReadString Reads a string
func (e *StickyLittleEndianReader) ReadString(size int) string {
	if e.Err != nil {
		return ""
	}
	buf := make([]byte, size)
	var n int
	n, e.Err = io.ReadFull(e.Reader, buf)
	e.Count += int64(n)
	return string(buf[:n])
}

// ReadStringX Reads the length of the string, using ReadUintX and then Reads the bytes of the string
func (e *StickyLittleEndianReader) ReadStringX() string {
	return e.ReadString(int(e.ReadUintX()))
}

// ReadString8 Reads the length of the string, using ReadUint8 and then Reads the bytes of the string
func (e *StickyLittleEndianReader) ReadString8() string {
	return e.ReadString(int(e.ReadUint8()))
}

// ReadString16 Reads the length of the string, using ReadUint16 and then Reads the bytes of the string
func (e *StickyLittleEndianReader) ReadString16() string {
	return e.ReadString(int(e.ReadUint16()))
}

// ReadString24 Reads the length of the string, using ReadUint24 and then Reads the bytes of the string
func (e *StickyLittleEndianReader) ReadString24() string {
	return e.ReadString(int(e.ReadUint24()))
}

// ReadString32 Reads the length of the string, using ReadUint32 and then Reads the bytes of the string
func (e *StickyLittleEndianReader) ReadString32() string {
	return e.ReadString(int(e.ReadUint32()))
}

// ReadString40 Reads the length of the string, using ReadUint40 and then Reads the bytes of the string
func (e *StickyLittleEndianReader) ReadString40() string {
	return e.ReadString(int(e.ReadUint40()))
}

// ReadString48 Reads the length of the string, using ReadUint48 and then Reads the bytes of the string
func (e *StickyLittleEndianReader) ReadString48() string {
	return e.ReadString(int(e.ReadUint48()))
}

// ReadString56 Reads the length of the string, using ReadUint56 and then Reads the bytes of the string
func (e *StickyLittleEndianReader) ReadString56() string {
	return e.ReadString(int(e.ReadUint56()))
}

// ReadString64 Reads the length of the string, using ReadUint64 and then Reads the bytes of the string
func (e *StickyLittleEndianReader) ReadString64() string {
	return e.ReadString(int(e.ReadUint64()))
}
