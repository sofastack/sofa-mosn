package byteio

// File automatically generated with ./gen.sh

import (
	"io"
	"math"
)

// LittleEndianReader wraps a io.Reader to provide methods
// to make it easier to Read fundamental types
type LittleEndianReader struct {
	io.Reader
	buffer [9]byte
}

// ReadInt8 Reads a 8 bit int as a int8 using the underlying io.Reader
func (e *LittleEndianReader) ReadInt8() (int8, int, error) {
	n, err := io.ReadFull(e.Reader, e.buffer[:1])
	if err != nil {
		return 0, n, err
	}
	return int8(e.buffer[0]), 1, nil
}

// ReadInt16 Reads a 16 bit int as a int16 using the underlying io.Reader
func (e *LittleEndianReader) ReadInt16() (int16, int, error) {
	n, err := io.ReadFull(e.Reader, e.buffer[:2])
	if err != nil {
		return 0, n, err
	}
	return int16(uint16(e.buffer[0]) | uint16(e.buffer[1])<<8), 2, nil
}

// ReadInt24 Reads a 24 bit int as a int32 using the underlying io.Reader
func (e *LittleEndianReader) ReadInt24() (int32, int, error) {
	n, err := io.ReadFull(e.Reader, e.buffer[:3])
	if err != nil {
		return 0, n, err
	}
	return int32(uint32(e.buffer[0]) | uint32(e.buffer[1])<<8 | uint32(e.buffer[2])<<16), 3, nil
}

// ReadInt32 Reads a 32 bit int as a int32 using the underlying io.Reader
func (e *LittleEndianReader) ReadInt32() (int32, int, error) {
	n, err := io.ReadFull(e.Reader, e.buffer[:4])
	if err != nil {
		return 0, n, err
	}
	return int32(uint32(e.buffer[0]) | uint32(e.buffer[1])<<8 | uint32(e.buffer[2])<<16 | uint32(e.buffer[3])<<24), 4, nil
}

// ReadInt40 Reads a 40 bit int as a int64 using the underlying io.Reader
func (e *LittleEndianReader) ReadInt40() (int64, int, error) {
	n, err := io.ReadFull(e.Reader, e.buffer[:5])
	if err != nil {
		return 0, n, err
	}
	return int64(uint64(e.buffer[0]) | uint64(e.buffer[1])<<8 | uint64(e.buffer[2])<<16 | uint64(e.buffer[3])<<24 | uint64(e.buffer[4])<<32), 5, nil
}

// ReadInt48 Reads a 48 bit int as a int64 using the underlying io.Reader
func (e *LittleEndianReader) ReadInt48() (int64, int, error) {
	n, err := io.ReadFull(e.Reader, e.buffer[:6])
	if err != nil {
		return 0, n, err
	}
	return int64(uint64(e.buffer[0]) | uint64(e.buffer[1])<<8 | uint64(e.buffer[2])<<16 | uint64(e.buffer[3])<<24 | uint64(e.buffer[4])<<32 | uint64(e.buffer[5])<<40), 6, nil
}

// ReadInt56 Reads a 56 bit int as a int64 using the underlying io.Reader
func (e *LittleEndianReader) ReadInt56() (int64, int, error) {
	n, err := io.ReadFull(e.Reader, e.buffer[:7])
	if err != nil {
		return 0, n, err
	}
	return int64(uint64(e.buffer[0]) | uint64(e.buffer[1])<<8 | uint64(e.buffer[2])<<16 | uint64(e.buffer[3])<<24 | uint64(e.buffer[4])<<32 | uint64(e.buffer[5])<<40 | uint64(e.buffer[6])<<48), 7, nil
}

// ReadInt64 Reads a 64 bit int as a int64 using the underlying io.Reader
func (e *LittleEndianReader) ReadInt64() (int64, int, error) {
	n, err := io.ReadFull(e.Reader, e.buffer[:8])
	if err != nil {
		return 0, n, err
	}
	return int64(uint64(e.buffer[0]) | uint64(e.buffer[1])<<8 | uint64(e.buffer[2])<<16 | uint64(e.buffer[3])<<24 | uint64(e.buffer[4])<<32 | uint64(e.buffer[5])<<40 | uint64(e.buffer[6])<<48 | uint64(e.buffer[7])<<56), 8, nil
}

// ReadUint8 Reads a 8 bit uint as a uint8 using the underlying io.Reader
func (e *LittleEndianReader) ReadUint8() (uint8, int, error) {
	n, err := io.ReadFull(e.Reader, e.buffer[:1])
	if err != nil {
		return 0, n, err
	}
	return e.buffer[0], 1, nil
}

// ReadUint16 Reads a 16 bit uint as a uint16 using the underlying io.Reader
func (e *LittleEndianReader) ReadUint16() (uint16, int, error) {
	n, err := io.ReadFull(e.Reader, e.buffer[:2])
	if err != nil {
		return 0, n, err
	}
	return uint16(e.buffer[0]) | uint16(e.buffer[1])<<8, 2, nil
}

// ReadUint24 Reads a 24 bit uint as a uint32 using the underlying io.Reader
func (e *LittleEndianReader) ReadUint24() (uint32, int, error) {
	n, err := io.ReadFull(e.Reader, e.buffer[:3])
	if err != nil {
		return 0, n, err
	}
	return uint32(e.buffer[0]) | uint32(e.buffer[1])<<8 | uint32(e.buffer[2])<<16, 3, nil
}

// ReadUint32 Reads a 32 bit uint as a uint32 using the underlying io.Reader
func (e *LittleEndianReader) ReadUint32() (uint32, int, error) {
	n, err := io.ReadFull(e.Reader, e.buffer[:4])
	if err != nil {
		return 0, n, err
	}
	return uint32(e.buffer[0]) | uint32(e.buffer[1])<<8 | uint32(e.buffer[2])<<16 | uint32(e.buffer[3])<<24, 4, nil
}

// ReadUint40 Reads a 40 bit uint as a uint64 using the underlying io.Reader
func (e *LittleEndianReader) ReadUint40() (uint64, int, error) {
	n, err := io.ReadFull(e.Reader, e.buffer[:5])
	if err != nil {
		return 0, n, err
	}
	return uint64(e.buffer[0]) | uint64(e.buffer[1])<<8 | uint64(e.buffer[2])<<16 | uint64(e.buffer[3])<<24 | uint64(e.buffer[4])<<32, 5, nil
}

// ReadUint48 Reads a 48 bit uint as a uint64 using the underlying io.Reader
func (e *LittleEndianReader) ReadUint48() (uint64, int, error) {
	n, err := io.ReadFull(e.Reader, e.buffer[:6])
	if err != nil {
		return 0, n, err
	}
	return uint64(e.buffer[0]) | uint64(e.buffer[1])<<8 | uint64(e.buffer[2])<<16 | uint64(e.buffer[3])<<24 | uint64(e.buffer[4])<<32 | uint64(e.buffer[5])<<40, 6, nil
}

// ReadUint56 Reads a 56 bit uint as a uint64 using the underlying io.Reader
func (e *LittleEndianReader) ReadUint56() (uint64, int, error) {
	n, err := io.ReadFull(e.Reader, e.buffer[:7])
	if err != nil {
		return 0, n, err
	}
	return uint64(e.buffer[0]) | uint64(e.buffer[1])<<8 | uint64(e.buffer[2])<<16 | uint64(e.buffer[3])<<24 | uint64(e.buffer[4])<<32 | uint64(e.buffer[5])<<40 | uint64(e.buffer[6])<<48, 7, nil
}

// ReadUint64 Reads a 64 bit uint as a uint64 using the underlying io.Reader
func (e *LittleEndianReader) ReadUint64() (uint64, int, error) {
	n, err := io.ReadFull(e.Reader, e.buffer[:8])
	if err != nil {
		return 0, n, err
	}
	return uint64(e.buffer[0]) | uint64(e.buffer[1])<<8 | uint64(e.buffer[2])<<16 | uint64(e.buffer[3])<<24 | uint64(e.buffer[4])<<32 | uint64(e.buffer[5])<<40 | uint64(e.buffer[6])<<48 | uint64(e.buffer[7])<<56, 8, nil
}

// ReadFloat32 Reads a 32 bit float as a float32 using the underlying io.Reader
func (e *LittleEndianReader) ReadFloat32() (float32, int, error) {
	n, err := io.ReadFull(e.Reader, e.buffer[:4])
	if err != nil {
		return 0, n, err
	}
	return math.Float32frombits(uint32(e.buffer[0]) | uint32(e.buffer[1])<<8 | uint32(e.buffer[2])<<16 | uint32(e.buffer[3])<<24), 4, nil
}

// ReadFloat64 Reads a 64 bit float as a float64 using the underlying io.Reader
func (e *LittleEndianReader) ReadFloat64() (float64, int, error) {
	n, err := io.ReadFull(e.Reader, e.buffer[:8])
	if err != nil {
		return 0, n, err
	}
	return math.Float64frombits(uint64(e.buffer[0]) | uint64(e.buffer[1])<<8 | uint64(e.buffer[2])<<16 | uint64(e.buffer[3])<<24 | uint64(e.buffer[4])<<32 | uint64(e.buffer[5])<<40 | uint64(e.buffer[6])<<48 | uint64(e.buffer[7])<<56), 8, nil
}

// ReadString Reads a string
func (e *LittleEndianReader) ReadString(size int) (string, int, error) {
	buf := make([]byte, size)
	n, err := io.ReadFull(e, buf)
	return string(buf[:n]), n, err
}

// ReadStringX Reads the length of the string, using ReadUintX and then Reads the bytes of the string
func (e *LittleEndianReader) ReadStringX() (string, int, error) {
	size, n, err := e.ReadUintX()
	if err != nil {
		return "", n, err
	}
	str, m, err := e.ReadString(int(size))
	return str, n + m, err
}

// ReadString8 Reads the length of the string, using ReadUint8 and then Reads the bytes of the string
func (e *LittleEndianReader) ReadString8() (string, int, error) {
	size, n, err := e.ReadUint8()
	if err != nil {
		return "", n, err
	}
	str, m, err := e.ReadString(int(size))
	return str, n + m, err
}

// ReadString16 Reads the length of the string, using ReadUint16 and then Reads the bytes of the string
func (e *LittleEndianReader) ReadString16() (string, int, error) {
	size, n, err := e.ReadUint16()
	if err != nil {
		return "", n, err
	}
	str, m, err := e.ReadString(int(size))
	return str, n + m, err
}

// ReadString24 Reads the length of the string, using ReadUint24 and then Reads the bytes of the string
func (e *LittleEndianReader) ReadString24() (string, int, error) {
	size, n, err := e.ReadUint24()
	if err != nil {
		return "", n, err
	}
	str, m, err := e.ReadString(int(size))
	return str, n + m, err
}

// ReadString32 Reads the length of the string, using ReadUint32 and then Reads the bytes of the string
func (e *LittleEndianReader) ReadString32() (string, int, error) {
	size, n, err := e.ReadUint32()
	if err != nil {
		return "", n, err
	}
	str, m, err := e.ReadString(int(size))
	return str, n + m, err
}

// ReadString40 Reads the length of the string, using ReadUint40 and then Reads the bytes of the string
func (e *LittleEndianReader) ReadString40() (string, int, error) {
	size, n, err := e.ReadUint40()
	if err != nil {
		return "", n, err
	}
	str, m, err := e.ReadString(int(size))
	return str, n + m, err
}

// ReadString48 Reads the length of the string, using ReadUint48 and then Reads the bytes of the string
func (e *LittleEndianReader) ReadString48() (string, int, error) {
	size, n, err := e.ReadUint48()
	if err != nil {
		return "", n, err
	}
	str, m, err := e.ReadString(int(size))
	return str, n + m, err
}

// ReadString56 Reads the length of the string, using ReadUint56 and then Reads the bytes of the string
func (e *LittleEndianReader) ReadString56() (string, int, error) {
	size, n, err := e.ReadUint56()
	if err != nil {
		return "", n, err
	}
	str, m, err := e.ReadString(int(size))
	return str, n + m, err
}

// ReadString64 Reads the length of the string, using ReadUint64 and then Reads the bytes of the string
func (e *LittleEndianReader) ReadString64() (string, int, error) {
	size, n, err := e.ReadUint64()
	if err != nil {
		return "", n, err
	}
	str, m, err := e.ReadString(int(size))
	return str, n + m, err
}
