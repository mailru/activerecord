package iproto

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"
	"reflect"
	"testing"
)

var testValues = []struct {
	Unpacked  interface{}
	PackedHEX string
}{
	{Unpacked: uint8(2), PackedHEX: "02"},
	{Unpacked: uint16(258), PackedHEX: "0201"},
	{Unpacked: uint32(258), PackedHEX: "02010000"},
	{Unpacked: uint(258), PackedHEX: "02010000"},
	{Unpacked: uint64(258), PackedHEX: "0201000000000000"},
	{Unpacked: int8(-1), PackedHEX: "ff"},
	{Unpacked: int16(-1234), PackedHEX: "2efb"},
	{Unpacked: int32(-1234567890), PackedHEX: "2efd69b6"},
	{Unpacked: -1234567890, PackedHEX: "2efd69b6"},
	{Unpacked: int64(-1234567890123456), PackedHEX: "404575c32a9dfbff"},
	{Unpacked: string("   "), PackedHEX: "03000000" + "202020"},
	{Unpacked: []uint32{2, 2}, PackedHEX: "020000000200000002000000"},

	{
		Unpacked: struct {
			F1 uint8
			F2 string
		}{
			F1: 15,
			F2: "  ",
		},
		PackedHEX: "0f" + "02000000" + "2020",
	}, {
		Unpacked: struct {
			F1 uint32 `iproto:"ber"`
			F2 string `iproto:"ber"`
		}{
			F1: 258,
			F2: "  ",
		},
		PackedHEX: "8202" + "02" + "2020",
	},
}

func TestPack(t *testing.T) {
	for i, v := range testValues {
		bytes, err := Pack(v.Unpacked)
		if err != nil {
			t.Errorf("[%v] Pack('%+v' %T) error: %v", i, v.Unpacked, v.Unpacked, err)
			continue
		}
		hex := hex.EncodeToString(bytes)
		if hex != v.PackedHEX {
			t.Errorf("[%v] Pack('%+v' %T) = %v; want %v", i, v.Unpacked, v.Unpacked, hex, v.PackedHEX)
		}
	}
}

func TestUnpack(t *testing.T) {
	for i, v := range testValues {
		bytes, err := hex.DecodeString(v.PackedHEX)
		if err != nil {
			t.Errorf("[%v] hex.DecodeString(%v) error: %v", i, v.PackedHEX, err)
			continue
		}

		tp := reflect.TypeOf(v.Unpacked)
		got := reflect.New(tp)

		_, err = Unpack(bytes, got.Interface())
		if err != nil {
			t.Errorf("[%v] Unpack('%x' %T) error: %v", i, bytes, v.Unpacked, err)
			continue
		}
		if !reflect.DeepEqual(reflect.Indirect(got).Interface(), v.Unpacked) {
			t.Errorf("[%v] Unpack('%x' %T) = %+v; want %+v", i, bytes, v.Unpacked, reflect.Indirect(got), v.Unpacked)
		}
	}

}

func TestBER(t *testing.T) {
	for _, test := range []struct {
		Unpacked  interface{}
		PackedHEX string
	}{
		{5, "05"},
		{0, "00"},
		{int32(-1), "8fffffff7f"},
		{int(-1), "8fffffff7f"},

		{128, "8100"},
		{uint32(math.MaxUint32), "8fffffff7f"},
		{uint64(math.MaxUint64), "81ffffffffffffffff7f"},
	} {
		t.Run(fmt.Sprintf("%T=%v", test.Unpacked, test.Unpacked), func(t *testing.T) {
			bytesWant, err := hex.DecodeString(test.PackedHEX)
			if err != nil {
				t.Fatalf("DecodeString(%s) error: %v", test.PackedHEX, err)
			}

			bytesGot, err := PackBER(test.Unpacked)
			if err != nil {
				t.Errorf("PackBER() error: %v", err)
			}

			if !reflect.DeepEqual(bytesGot, bytesWant) {
				t.Errorf("PackBER() = %x; want %x", bytesGot, bytesWant)
			}

			tp := reflect.TypeOf(test.Unpacked)
			got := reflect.New(tp)

			tail, err := UnpackBER(bytesWant, got.Interface())
			if err != nil {
				t.Errorf("UnpackBER() error: %v", err)
			}
			if len(tail) != 0 {
				t.Errorf("UnpackBER() data left: %x", tail)
			}

			if !reflect.DeepEqual(reflect.Indirect(got).Interface(), test.Unpacked) {
				t.Errorf("UnpackBER() = %v; want %v", reflect.Indirect(got), test.Unpacked)
			}
		})
	}

}

type testStrings []string

func (v testStrings) IprotoPack(w []byte, mode PackMode) ([]byte, error) {
	data := make([]byte, 0)
	for _, s := range v {
		data = PackString(data, s, mode)
	}
	return PackBytes(w, data, mode), nil
}

func (v *testStrings) IprotoUnpack(r *bytes.Reader, mode PackMode) error {
	var (
		strings []byte
		err     error
		str     string
	)
	err = UnpackBytes(r, &strings, mode)
	*v = make([]string, 0)
	for err == nil && len(strings) > 0 {
		switch mode {
		case ModeDefault:
			strings, err = Unpack(strings, &str)
		case ModeBER:
			strings, err = UnpackBER(strings, &str)
		}
		if err == nil {
			*v = append(*v, str)
		}
	}
	return err
}

func testPackUnpack(t *testing.T, values []interface{}, mode PackMode) {
	var (
		perr, uerr error
		data, tail []byte
	)
	for _, v := range values {
		switch mode {
		case ModeDefault:
			data, perr = Pack(v)
		case ModeBER:
			data, perr = PackBER(v)
		}
		t.Logf("Pack(%T): v=%v, data=%v, pack err=%v", v, v, data, perr)
		if perr != nil {
			t.Errorf("Pack error: %v", perr)
		}
		pres := reflect.New(reflect.TypeOf(v)).Elem().Addr().Interface()
		switch mode {
		case ModeDefault:
			tail, uerr = Unpack(data, pres)
		case ModeBER:
			tail, perr = UnpackBER(data, pres)
		}
		res := reflect.Indirect(reflect.ValueOf(pres).Elem()).Interface()
		t.Logf("Unpack(%T): v=%v, data left=%v, unpack err=%v", res, res, tail, uerr)
		switch {
		case uerr != nil:
			t.Errorf("Unpack error: %v", uerr)
		case len(tail) > 0:
			t.Errorf("Data left: %v", tail)
		case !reflect.DeepEqual(v, res):
			t.Errorf("Unpacked value %v does not match %v", res, v)
		}
	}
}

func TestPackUnpack(t *testing.T) {
	values := []interface{}{
		testStrings([]string{}),
		testStrings([]string{"abc", "cde"}),
		testStrings([]string{"0123", "", "!"}),
	}
	testPackUnpack(t, values, ModeDefault)
	testPackUnpack(t, values, ModeBER)
}
