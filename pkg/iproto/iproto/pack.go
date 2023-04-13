package iproto

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"reflect"
)

// PackMode defines encoding mode for integers and string/slice lengths. Mode is not recursive
// and applies only to the top-level element.
type PackMode int

const (
	ModeDefault PackMode = 0
	ModeBER     PackMode = 1
)

// PackUint8 packs a single 8-bit integer.
func PackUint8(w []byte, v uint8, mode PackMode) []byte {
	return append(w, v)
}

func packUint64BER(w []byte, v uint64) []byte {
	const maxLen = 10 // 10 bytes is sufficient to hold a 128-base 64-bit uint.

	if v == 0 {
		return append(w, 0)
	}

	buf := make([]byte, maxLen)
	n := maxLen - 1

	for ; n >= 0 && v > 0; n-- {
		buf[n] = byte(v & 0x7f)
		v >>= 7

		if n != (maxLen - 1) {
			buf[n] |= 0x80
		}
	}

	return append(w, buf[n+1:]...)
}

// PackUint16 packs a single 16-bit integer according to mode.
func PackUint16(w []byte, v uint16, mode PackMode) []byte {
	if mode == ModeBER {
		return packUint64BER(w, uint64(v))
	}

	return append(w,
		byte(v),
		byte(v>>8),
	)
}

// PackUint32 packs a single 32-bit integer according to mode.
func PackUint32(w []byte, v uint32, mode PackMode) []byte {
	if mode == ModeBER {
		return packUint64BER(w, uint64(v))
	}

	return append(w,
		byte(v),
		byte(v>>8),
		byte(v>>16),
		byte(v>>24),
	)
}

// PackUint64 packs a single 64-bit integer according to mode.
func PackUint64(w []byte, v uint64, mode PackMode) []byte {
	if mode == ModeBER {
		return packUint64BER(w, v)
	}

	return append(w,
		byte(v),
		byte(v>>8),
		byte(v>>16),
		byte(v>>24),
		byte(v>>32),
		byte(v>>40),
		byte(v>>48),
		byte(v>>56),
	)
}

// PackString packs string length (according to mode) followed by string data.
func PackString(w []byte, v string, mode PackMode) []byte {
	w = PackUint32(w, uint32(len(v)), mode)
	return append(w, v...)
}

// PackBytes packs data length (according to mode) followed by the data.
func PackBytes(w []byte, v []byte, mode PackMode) []byte {
	w = PackUint32(w, uint32(len(v)), mode)
	return append(w, v...)
}

const ber = "ber"

func packStruct(w []byte, v reflect.Value) ([]byte, error) {
	tp := v.Type()

	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)

		mode := ModeDefault

		tag := tp.Field(i).Tag.Get("iproto")
		if tag == ber {
			mode = ModeBER
		}

		var err error

		w, err = packOne(w, f.Interface(), mode)
		if err != nil {
			return nil, err
		}
	}

	return w, nil
}

func packSlice(w []byte, v reflect.Value, mode PackMode) ([]byte, error) {
	w = PackUint32(w, uint32(v.Len()), mode)

	for i := 0; i < v.Len(); i++ {
		f := v.Index(i)

		var err error

		w, err = packOne(w, f.Interface(), ModeDefault)
		if err != nil {
			return nil, err
		}
	}

	return w, nil
}

type Packer interface {
	IprotoPack(w []byte, mode PackMode) ([]byte, error)
}

func packOne(w []byte, v interface{}, mode PackMode) ([]byte, error) {
	if v == nil {
		return w, nil
	}

	switch v := v.(type) {
	case Packer:
		return v.IprotoPack(w, mode)
	case uint8:
		return PackUint8(w, v, mode), nil
	case int8:
		return PackUint8(w, uint8(v), mode), nil
	case uint16:
		return PackUint16(w, v, mode), nil
	case int16:
		return PackUint16(w, uint16(v), mode), nil
	case uint32:
		return PackUint32(w, v, mode), nil
	case int32:
		return PackUint32(w, uint32(v), mode), nil
	case int:
		return PackUint32(w, uint32(v), mode), nil
	case uint:
		return PackUint32(w, uint32(v), mode), nil
	case uint64:
		return PackUint64(w, v, mode), nil
	case int64:
		return PackUint64(w, uint64(v), mode), nil
	case []byte:
		return PackBytes(w, v, mode), nil
	case string:
		return PackString(w, v, mode), nil
	}

	val := reflect.Indirect(reflect.ValueOf(v))
	if val.Kind() == reflect.Struct {
		return packStruct(w, val)
	} else if val.Kind() == reflect.Slice {
		return packSlice(w, val, mode)
	}

	return nil, fmt.Errorf("unsupported iproto pack type: %T", v)
}

// UnpackUint8 unpacks a single 8-bit integer.
func UnpackUint8(r *bytes.Reader, v *uint8, mode PackMode) (err error) {
	*v, err = r.ReadByte()
	return
}

func unpackBER(r *bytes.Reader, valueBits int) (v uint64, err error) {
	v = 0

	for i := 0; i <= valueBits/7; i++ {
		var b byte

		b, err = r.ReadByte()
		if err != nil {
			break
		}

		v <<= 7
		v |= uint64(b & 0x7f)

		if b&0x80 == 0 {
			return
		}
	}

	return 0, fmt.Errorf("invalid ber-encoded integer")
}

// UnpackUint16 unpacks a single 16-bit integer according to mode.
func UnpackUint16(r *bytes.Reader, v *uint16, mode PackMode) (err error) {
	if mode == ModeBER {
		var v0 uint64
		v0, err = unpackBER(r, 16)
		*v = uint16(v0)

		return
	}

	data := make([]byte, 2)
	_, err = r.Read(data)

	*v = binary.LittleEndian.Uint16(data)

	return
}

// UnpackUint32 unpacks a single 32-bit integer according to mode.
func UnpackUint32(r *bytes.Reader, v *uint32, mode PackMode) (err error) {
	if mode == ModeBER {
		var v0 uint64
		v0, err = unpackBER(r, 32)
		*v = uint32(v0)

		return
	}

	data := make([]byte, 4)
	_, err = r.Read(data)

	*v = binary.LittleEndian.Uint32(data)

	return
}

// UnpackUint64 unpacks a single 64-bit integer according to mode.
func UnpackUint64(r *bytes.Reader, v *uint64, mode PackMode) (err error) {
	if mode == ModeBER {
		*v, err = unpackBER(r, 64)
		return
	}

	data := make([]byte, 8)
	_, err = r.Read(data)

	*v = uint64(data[0])
	*v += uint64(data[1]) << 8
	*v += uint64(data[2]) << 16
	*v += uint64(data[3]) << 24
	*v += uint64(data[4]) << 32
	*v += uint64(data[5]) << 40
	*v += uint64(data[6]) << 48
	*v += uint64(data[7]) << 56

	return
}

// UnpackString unpacks a string prefixed by length, length is packed according to mode.
func UnpackString(r *bytes.Reader, v *string, mode PackMode) (err error) {
	var l uint32

	if err = UnpackUint32(r, &l, mode); err != nil {
		return
	}

	if int64(l) > r.Size() {
		return fmt.Errorf("cant unpack string - invalid string length %d in packet of length %d", l, r.Size())
	}

	buf := make([]byte, l)

	if _, err = io.ReadFull(r, buf); err != nil {
		return err
	}

	*v = string(buf)

	return nil
}

// UnpackBytes unpacks raw bytes prefixed by length, length is packed according to mode.
func UnpackBytes(r *bytes.Reader, v *[]byte, mode PackMode) (err error) {
	var l uint32

	if err = UnpackUint32(r, &l, mode); err != nil {
		return
	}

	if int64(l) > r.Size() {
		return fmt.Errorf("cant unpack bytes - invalid bytes length %d in packet of length %d", l, r.Size())
	}

	*v = make([]byte, l)
	_, err = io.ReadFull(r, *v)

	return err
}

func unpackStruct(r *bytes.Reader, v reflect.Value) error {
	tp := v.Type()

	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)

		mode := ModeDefault

		tag := tp.Field(i).Tag.Get("iproto")
		if tag == ber {
			mode = ModeBER
		}

		err := unpackOne(r, f.Addr().Interface(), mode)
		if err != nil {
			return err
		}
	}

	return nil
}

func unpackSlice(r *bytes.Reader, v reflect.Value, mode PackMode) error {
	var l uint32

	if err := UnpackUint32(r, &l, mode); err != nil {
		return err
	}

	v.Set(reflect.MakeSlice(v.Type(), int(l), int(l)))

	for i := 0; i < v.Len(); i++ {
		f := v.Index(i)

		err := unpackOne(r, f.Addr().Interface(), ModeDefault)
		if err != nil {
			return err
		}
	}

	return nil
}

type Unpacker interface {
	IprotoUnpack(r *bytes.Reader, mode PackMode) error
}

func unpackOne(r *bytes.Reader, v interface{}, mode PackMode) error {
	if v == nil {
		return nil
	}

	switch v := v.(type) {
	case Unpacker:
		return v.IprotoUnpack(r, mode)
	case *uint8:
		return UnpackUint8(r, v, mode)
	case *int8:
		var v0 uint8
		err := UnpackUint8(r, &v0, mode)
		*v = int8(v0)

		return err
	case *uint16:
		return UnpackUint16(r, v, mode)
	case *int16:
		var v0 uint16
		err := UnpackUint16(r, &v0, mode)
		*v = int16(v0)

		return err
	case *uint32:
		return UnpackUint32(r, v, mode)
	case *int32:
		var v0 uint32
		err := UnpackUint32(r, &v0, mode)
		*v = int32(v0)

		return err
	case *int:
		var v0 uint32
		err := UnpackUint32(r, &v0, mode)
		*v = int(int32(v0))

		return err
	case *uint:
		var v0 uint32
		err := UnpackUint32(r, &v0, mode)
		*v = uint(v0)

		return err
	case *uint64:
		return UnpackUint64(r, v, mode)
	case *int64:
		var v0 uint64
		err := UnpackUint64(r, &v0, mode)
		*v = int64(v0)

		return err
	case *string:
		return UnpackString(r, v, mode)
	case *[]byte:
		return UnpackBytes(r, v, mode)
	}

	val := reflect.Indirect(reflect.ValueOf(v))
	if val.Kind() == reflect.Struct {
		return unpackStruct(r, val)
	} else if val.Kind() == reflect.Slice {
		return unpackSlice(r, val, mode)
	}

	return fmt.Errorf("unsupported iproto unpack type: %T", v)
}

func doAppend(mode PackMode, data []byte, value ...interface{}) (out []byte, err error) {
	out = data
	for _, v := range value {
		if out, err = packOne(out, v, mode); err != nil {
			return
		}
	}

	return
}

// Pack encodes values into a byte slice using default mode (little endian fixed length ints).
//
// Strings and slices are prefixed by item count. Structs are encoded field-by-field in the order
// of definition.
//
// For struct fields, `iproto:"ber"` tag can be used to use BER encoding for a particular field.
// BER-encoding used is similar to one used by perl 'pack()' function and is different from varint
// encoding used in go standard library. BER-encoding is not recursive: for slices only the length
// of the slice will be BER-encoded, not the items.
func Pack(value ...interface{}) (out []byte, err error) {
	return doAppend(ModeDefault, nil, value...)
}

// Append is similar to Pack function, but it appends to the slice instead of unconditionally creating
// a new slice.
func Append(data []byte, value ...interface{}) (out []byte, err error) {
	return doAppend(ModeDefault, data, value...)
}

// PackBER is similar to Pack function, except that BER-encoding is used by default for every item.
func PackBER(value ...interface{}) (out []byte, err error) {
	return doAppend(ModeBER, nil, value...)
}

// AppendBER is similar to Pack function, but it appends to the slice instead of unconditionally creating
// a new slice. BER-encoding is used by default for every item.
func AppendBER(data []byte, value ...interface{}) (out []byte, err error) {
	return doAppend(ModeBER, data, value...)
}

func unpack(mode PackMode, data []byte, value ...interface{}) ([]byte, error) {
	rdr := bytes.NewReader(data)
	for _, v := range value {
		if err := unpackOne(rdr, v, mode); err != nil {
			return nil, err
		}
	}

	l := rdr.Len()

	if l == 0 {
		return nil, nil
	}

	return data[len(data)-l:], nil
}

// Unpack decodes the data as encoded by Pack function. Remaining bytes are returned on success.
func Unpack(data []byte, value ...interface{}) ([]byte, error) {
	return unpack(ModeDefault, data, value...)
}

// UnpackBER is similar to Unpack function, except that BER-encoding is used by default for every item.
func UnpackBER(data []byte, value ...interface{}) ([]byte, error) {
	return unpack(ModeBER, data, value...)
}
