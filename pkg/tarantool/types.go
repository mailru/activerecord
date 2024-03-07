package tarantool

import (
	"context"

	"github.com/tarantool/go-tarantool"
)

type ModelStruct interface {
	Insert(ctx context.Context) error
	Replace(ctx context.Context) error
	InsertOrReplace(ctx context.Context) error
	Update(ctx context.Context) error
	Delete(ctx context.Context) error
}

type BaseField struct {
	Collection []ModelStruct
	UpdateOps  []tarantool.Op
	Exists     bool
	IsReplica  bool
	ReadOnly   bool
	Objects    map[string][]ModelStruct
}
