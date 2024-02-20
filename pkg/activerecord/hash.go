package activerecord

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash"
)

type GroupHash struct {
	hash       hash.Hash32
	calculated bool
}

func NewGroupHash(hash hash.Hash32) *GroupHash {
	return &GroupHash{hash: hash}
}

func (o *GroupHash) UpdateHash(data ...interface{}) error {
	if o.calculated {
		return fmt.Errorf("can't update hash after calculate")
	}

	for _, v := range data {
		var err error

		switch v := v.(type) {
		case string:
			err = binary.Write(o.hash, binary.LittleEndian, []byte(v))
		case int:
			err = binary.Write(o.hash, binary.LittleEndian, int64(v))
		default:
			err = binary.Write(o.hash, binary.LittleEndian, v)
		}

		if err != nil {
			return fmt.Errorf("can't calculate connectionID: %w", err)
		}
	}

	return nil
}

func (o *GroupHash) GetHash() string {
	o.calculated = true
	hashInBytes := o.hash.Sum(nil)[:]

	return hex.EncodeToString(hashInBytes)
}
