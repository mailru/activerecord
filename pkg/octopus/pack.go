package octopus

import (
	"bytes"
	"fmt"

	"github.com/mailru/activerecord/pkg/iproto/iproto"
)

func ByteLen(length uint32) uint32 {
	switch {
	case length < (1 << 7):
		return 1 + length
	case length < (1 << 14):
		return 2 + length
	case length < (1 << 21):
		return 3 + length
	case length < (1 << 28):
		return 4 + length
	default:
		return 5 + length
	}
}

func PackedTupleLen(keys [][]byte) (length uint32) {
	length = 4

	for _, k := range keys {
		length += PackedFieldLen(k)
	}

	return length
}

func PackedKeysLen(keys [][]byte) (length uint32) {
	length = 4

	for _, k := range keys {
		length += PackedFieldLen(k)
	}

	return length
}

func PackedKeyLen(keys [][]byte) (length uint32) {
	return PackedKeysLen(keys)
}

func PackedUpdateOpsLen(updateOps []Ops) (length uint32) {
	length = OpsLen

	for _, op := range updateOps {
		length += OpFieldNumLen + OpOpLen + ByteLen(uint32(len(op.Value)))
	}

	return length
}

func PackedFieldLen(field []byte) uint32 {
	return ByteLen(uint32(len(field)))
}

func PackFieldNums(w []byte, cnt uint32) []byte {
	return iproto.PackUint32(w, cnt, iproto.ModeDefault)
}

func UnpackFieldNums(r *bytes.Reader) (uint32, error) {
	var fieldsNum uint32

	if err := iproto.UnpackUint32(r, &fieldsNum, iproto.ModeDefault); err != nil {
		return 0, fmt.Errorf("can't unpack fieldsNum: %w", err)
	}

	return fieldsNum, nil
}

func PackedTuplesLen(keys [][][]byte) (length uint32) {
	length = 4

	for _, kt := range keys {
		length += PackedTupleLen(kt)
	}

	return length
}

func PackBool(w []byte, v bool, mode iproto.PackMode) ([]byte, error) {
	if v {
		return iproto.PackUint8(w, 1, mode), nil
	}

	return iproto.PackUint8(w, 0, mode), nil
}

func PackField(w []byte, field []byte) []byte {
	return iproto.PackBytes(w, field, iproto.ModeBER)
}

func UnpackField(r *bytes.Reader) ([]byte, error) {
	field := []byte{}

	if err := iproto.UnpackBytes(r, &field, iproto.ModeBER); err != nil {
		return nil, fmt.Errorf("can't unpack field: %w", err)
	}

	return field, nil
}

func PackKey(w []byte, key [][]byte) []byte {
	w = iproto.PackUint32(w, uint32(len(key)), iproto.ModeDefault)
	for _, k := range key {
		w = PackField(w, k)
	}

	return w
}

func UnpackKey(r *bytes.Reader) ([][]byte, error) {
	ret := [][]byte{}
	fieldNum := uint32(0)

	err := iproto.UnpackUint32(r, &fieldNum, iproto.ModeDefault)
	if err != nil {
		return nil, fmt.Errorf("can't unpack fieldnum: %s", err)
	}

	for f := uint32(0); f < fieldNum; f++ {
		field, err := UnpackField(r)
		if err != nil {
			return nil, fmt.Errorf("can't unpack field %d: %s", f, err)
		}

		ret = append(ret, field)
	}

	return ret, nil
}

func PackTuples(w []byte, keys [][][]byte) []byte {
	w = iproto.PackUint32(w, uint32(len(keys)), iproto.ModeDefault)

	for _, kt := range keys {
		w = PackTuple(w, kt)
	}

	return w
}

func UnpackTuples(r *bytes.Reader) ([][][]byte, error) {
	ret := [][][]byte{}

	var tuples uint32

	if err := iproto.UnpackUint32(r, &tuples, iproto.ModeDefault); err != nil {
		return nil, fmt.Errorf("can't unpack tuple cnt: %w", err)
	}

	for t := uint32(0); t < tuples; t++ {
		tuple, err := UnpackTuple(r)
		if err != nil {
			return nil, fmt.Errorf("can't unpack tuple: %w", err)
		}

		ret = append(ret, tuple)
	}

	return ret, nil
}

func PackTuple(w []byte, keys [][]byte) []byte {
	w = PackFieldNums(w, uint32(len(keys)))

	for _, k := range keys {
		w = PackField(w, k)
	}

	return w
}

func UnpackTuple(r *bytes.Reader) ([][]byte, error) {
	ret := [][]byte{}

	fieldsNum, err := UnpackFieldNums(r)
	if err != nil {
		return nil, fmt.Errorf("can't unpack fieldnum: %w", err)
	}

	for f := uint32(0); f < fieldsNum; f++ {
		field, err := UnpackField(r)
		if err != nil {
			return nil, fmt.Errorf("can't unpack field: %w", err)
		}

		ret = append(ret, field)
	}

	return ret, nil
}

func PackSpace(w []byte, space uint32) []byte {
	return iproto.PackUint32(w, space, iproto.ModeDefault)
}

func UnpackSpace(r *bytes.Reader) (uint32, error) {
	var space uint32

	if err := iproto.UnpackUint32(r, &space, iproto.ModeDefault); err != nil {
		return 0, fmt.Errorf("can't unpack space: %w", err)
	}

	return space, nil
}

func PackIndexNum(w []byte, indexnum uint32) []byte {
	return iproto.PackUint32(w, indexnum, iproto.ModeDefault)
}

func UnpackIndexNum(r *bytes.Reader) (uint32, error) {
	var indexnum uint32

	if err := iproto.UnpackUint32(r, &indexnum, iproto.ModeDefault); err != nil {
		return 0, fmt.Errorf("can't unpack indexnum: %w", err)
	}

	return indexnum, nil
}

func PackRequestFlagsVal(w []byte, ret bool, mode InsertMode) []byte {
	var flags uint32

	if ret {
		flags = 1
	}

	if mode != 0 {
		flags |= 1 << uint32(mode)
	}

	return iproto.PackUint32(w, flags, iproto.ModeDefault)
}

func UnpackRequestFlagsVal(r *bytes.Reader) (bool, InsertMode, error) {
	var flags uint32

	err := iproto.UnpackUint32(r, &flags, iproto.ModeDefault)
	if err != nil {
		return false, 0, fmt.Errorf("can't unpack flags: %w", err)
	}

	if flags&1 == 1 {
		return true, InsertMode(flags ^ 1), nil
	}

	return false, InsertMode(flags), nil
}

func PackDeleteFlagsVal(w []byte, ret bool) []byte {
	var flags uint32

	if ret {
		flags = 1
	}

	return iproto.PackUint32(w, flags, iproto.ModeDefault)
}

func PackLimit(w []byte, limit uint32) []byte {
	if limit == 0 {
		limit = 0xffffffff
	}

	return iproto.PackUint32(w, limit, iproto.ModeDefault)
}

func UnpackLimit(r *bytes.Reader) (uint32, error) {
	var limit uint32

	if err := iproto.UnpackUint32(r, &limit, iproto.ModeDefault); err != nil {
		return 0, fmt.Errorf("can't unpack limit: %w", err)
	}

	if limit == 0xffffffff {
		limit = 0
	}

	return limit, nil
}

func PackOffset(w []byte, offset uint32) []byte {
	return iproto.PackUint32(w, offset, iproto.ModeDefault)
}

func UnpackOffset(r *bytes.Reader) (uint32, error) {
	var limit uint32

	if err := iproto.UnpackUint32(r, &limit, iproto.ModeDefault); err != nil {
		return 0, fmt.Errorf("can't unpack offset: %w", err)
	}

	return limit, nil
}

func UnpackResopnseStatus(data []byte) (uint32, []byte, error) {
	rdr := bytes.NewReader(data)

	var retCode uint32

	if err := iproto.UnpackUint32(rdr, &retCode, iproto.ModeDefault); err != nil {
		return 0, []byte{}, fmt.Errorf("error unpack retCode: %w", err)
	}

	if RetCode(retCode) == RcOK {
		if rdr.Len() == 0 {
			return 0, []byte{}, nil
		}

		if rdr.Len() < 4 {
			return 0, nil, fmt.Errorf("error unpack tuple cnt data to small: '%d'", rdr.Len())
		}

		var cnt uint32

		err := iproto.UnpackUint32(rdr, &cnt, iproto.ModeDefault)
		if err != nil {
			return 0, nil, fmt.Errorf("error unpack tuple cnt in boxResp: '%w'", err)
		}

		return cnt, data[len(data)-rdr.Len():], nil
	}

	errStr := data[4:]
	if len(errStr) > 0 && errStr[len(errStr)-1] == 0x0 {
		errStr = errStr[:len(errStr)-1]
	}

	return 0, nil, fmt.Errorf("error request to octopus `%s`", errStr)
}

func PackResopnseStatus(statusCode RetCode, data [][][]byte) ([]byte, error) {
	resp := []byte{}
	resp = iproto.PackUint32(resp, uint32(statusCode), iproto.ModeDefault)

	if statusCode == RcOK {
		if len(data) == 0 {
			return resp, nil
		}

		resp = iproto.PackUint32(resp, uint32(len(data)), iproto.ModeDefault)

		for _, tuple := range data {
			payload := iproto.PackUint32([]byte{}, uint32(len(tuple)), iproto.ModeDefault)

			for _, field := range tuple {
				payload = PackField(payload, field)
			}

			resp = iproto.PackUint32(resp, uint32(len(payload)), iproto.ModeDefault)
			resp = append(resp, payload...)
		}

		return resp, nil
	}

	resp = iproto.PackUint32(resp, uint32(len(data[0])), iproto.ModeDefault)
	resp = append(resp, data[0][0]...)

	return resp, nil
}

func PackString(w []byte, field string, mode iproto.PackMode) []byte {
	return append(w, field...)
}

func UnpackString(r *bytes.Reader, res *string, mode iproto.PackMode) error {
	len := r.Len()
	if len == 0 {
		*res = ""
		return nil
	}

	bres := make([]byte, len)

	rlen, err := r.Read(bres)
	if err != nil {
		return fmt.Errorf("error unpack string: '%s'", err)
	}

	if rlen != len {
		return fmt.Errorf("error len while unpack string: %d, want %d", rlen, len)
	}

	*res = string(bres)

	return nil
}

func BoolToUint(v bool) uint8 {
	if v {
		return 1
	}

	return 0
}

func UintToBool(v uint8) bool {
	return v != 0
}
