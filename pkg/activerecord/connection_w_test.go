package activerecord

import (
	"reflect"
	"sync"
	"testing"
)

type TestOptions struct {
	hash string
}

func (to *TestOptions) InstanceMode() ServerModeType {
	return ModeMaster
}

func (to *TestOptions) GetConnectionID() string {
	return to.hash
}

type TestConnection struct {
	ch chan struct{}
	id string
}

func (tc *TestConnection) Close() {
	tc.ch <- struct{}{}
}

func (tc *TestConnection) Done() <-chan struct{} {
	return tc.ch
}

var connectionCall = 0

func connectorFunc(options interface{}) (ConnectionInterface, error) {
	connectionCall++
	to, _ := options.(*TestOptions)
	return &TestConnection{id: to.hash}, nil
}

func Test_connectionPool_Add(t *testing.T) {
	to1 := &TestOptions{hash: "testopt1"}

	var clusterInfo = NewClusterInfo(
		WithShard([]OptionInterface{to1}, []OptionInterface{}),
	)

	type args struct {
		shard     ShardInstance
		connector func(interface{}) (ConnectionInterface, error)
	}

	tests := []struct {
		name    string
		args    args
		want    ConnectionInterface
		wantErr bool
		wantCnt int
	}{
		{
			name: "first connection",
			args: args{
				shard:     clusterInfo.NextMaster(0),
				connector: connectorFunc,
			},
			wantErr: false,
			want:    &TestConnection{id: "testopt1"},
			wantCnt: 1,
		},
		{
			name: "again first connection",
			args: args{
				shard:     clusterInfo.NextMaster(0),
				connector: connectorFunc,
			},
			wantErr: true,
			wantCnt: 1,
		},
	}

	cp := connectionPool{
		lock:      sync.Mutex{},
		container: map[string]ConnectionInterface{},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cp.Add(tt.args.shard, tt.args.connector)
			if (err != nil) != tt.wantErr {
				t.Errorf("connectionPool.Add() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("connectionPool.Add() = %+v, want %+v", got, tt.want)
			}
			if connectionCall != tt.wantCnt {
				t.Errorf("connectionPool.Add() connectionCnt = %v, want %v", connectionCall, tt.wantCnt)
			}
		})
	}
}
