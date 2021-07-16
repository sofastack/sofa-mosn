// Package byteio helps with writing number types in both big and little endian formats
package byteio // import "vimagination.zapto.org/byteio"

import "io"

// EndianReader is an interface that reads various types with a particular
// endianness
type EndianReader interface {
	io.Reader
	io.ByteReader
	ReadUint8() (uint8, int, error)
	ReadInt8() (int8, int, error)
	ReadUint16() (uint16, int, error)
	ReadInt16() (int16, int, error)
	ReadUint32() (uint32, int, error)
	ReadInt32() (int32, int, error)
	ReadUint64() (uint64, int, error)
	ReadInt64() (int64, int, error)
	ReadFloat32() (float32, int, error)
	ReadFloat64() (float64, int, error)
	ReadUintX() (uint64, int, error)
	ReadIntX() (int64, int, error)
	ReadString(int) (string, int, error)
	ReadStringX() (string, int, error)
	ReadString8() (string, int, error)
	ReadString16() (string, int, error)
	ReadString32() (string, int, error)
	ReadString64() (string, int, error)
}

// EndianWriter is an interface that writes various types with a particular
// endianness
type EndianWriter interface {
	io.Writer
	io.ByteWriter
	WriteUint8(uint8) (int, error)
	WriteInt8(int8) (int, error)
	WriteUint16(uint16) (int, error)
	WriteInt16(int16) (int, error)
	WriteUint32(uint32) (int, error)
	WriteInt32(int32) (int, error)
	WriteUint64(uint64) (int, error)
	WriteInt64(int64) (int, error)
	WriteFloat32(float32) (int, error)
	WriteFloat64(float64) (int, error)
	WriteUintX(uint64) (int, error)
	WriteIntX(int64) (int, error)
	WriteString(string) (int, error)
	WriteStringX(string) (int, error)
	WriteString8(string) (int, error)
	WriteString16(string) (int, error)
	WriteString32(string) (int, error)
	WriteString64(string) (int, error)
}

// StickyEndianReader is an interface that reads various types with a
// particular endianness and stores the Read return values
type StickyEndianReader interface {
	io.Reader
	io.ByteReader
	ReadUint8() uint8
	ReadInt8() int8
	ReadUint16() uint16
	ReadInt16() int16
	ReadUint32() uint32
	ReadInt32() int32
	ReadUint64() uint64
	ReadInt64() int64
	ReadFloat32() float32
	ReadFloat64() float64
	ReadUintX() uint64
	ReadIntX() int64
	ReadString(int) string
	ReadStringX() string
	ReadString8() string
	ReadString16() string
	ReadString32() string
	ReadString64() string
}

// StickyEndianWriter is an interface that writes various types with a
// particular endianness and stores the Write return values
type StickyEndianWriter interface {
	io.Writer
	io.ByteWriter
	WriteUint8(uint8)
	WriteInt8(int8)
	WriteUint16(uint16)
	WriteInt16(int16)
	WriteUint32(uint32)
	WriteInt32(int32)
	WriteUint64(uint64)
	WriteInt64(int64)
	WriteFloat32(float32)
	WriteFloat64(float64)
	WriteUintX(uint64)
	WriteIntX(int64)
	WriteString(string) (int, error)
	WriteStringX(string)
	WriteString8(string)
	WriteString16(string)
	WriteString32(string)
	WriteString64(string)
}

var (
	_ EndianReader       = (*BigEndianReader)(nil)
	_ EndianReader       = (*LittleEndianReader)(nil)
	_ EndianWriter       = (*BigEndianWriter)(nil)
	_ EndianWriter       = (*LittleEndianWriter)(nil)
	_ StickyEndianReader = (*StickyBigEndianReader)(nil)
	_ StickyEndianReader = (*StickyLittleEndianReader)(nil)
	_ StickyEndianWriter = (*StickyBigEndianWriter)(nil)
	_ StickyEndianWriter = (*StickyLittleEndianWriter)(nil)
)
