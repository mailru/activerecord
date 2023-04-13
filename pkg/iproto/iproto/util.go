package iproto

import (
	"io"
	"sync"
)

type result struct {
	data []byte
	err  error
}

var fnPool sync.Pool

type fn struct {
	ch chan result
	cb func([]byte, error)
}

func acquireResultFunc() *fn {
	if r, _ := fnPool.Get().(*fn); r != nil {
		return r
	}

	ch := make(chan result, 1)

	return &fn{ch, func(data []byte, err error) {
		ch <- result{data, err}
	}}
}

func releaseResultFunc(r *fn) {
	if len(r.ch) == 0 {
		fnPool.Put(r)
	}
}

// BytePoolFunc returns BytePool that uses given get and put functions as its
// methods.
func BytePoolFunc(get func(int) []byte, put func([]byte)) BytePool {
	return &bytePool{get, put}
}

type bytePool struct {
	DoGet func(int) []byte
	DoPut func([]byte)
}

func (p *bytePool) Get(n int) []byte {
	if p.DoGet != nil {
		return p.DoGet(n)
	}

	return make([]byte, n)
}

func (p *bytePool) Put(bts []byte) {
	if p.DoPut != nil {
		p.DoPut(bts)
	}
}

// CopyPoolConfig return deep copy of c.
// If c is nil, it returns new PoolConfig.
// Returned config always contains non-nil ChannelConfig.
func CopyPoolConfig(c *PoolConfig) (pc *PoolConfig) {
	if c == nil {
		pc = &PoolConfig{}
	} else {
		cp := *c
		pc = &cp
	}

	pc.ChannelConfig = CopyChannelConfig(pc.ChannelConfig)

	return pc
}

// CopyChannelConfig returns deep copy of c.
// If c is nil, it returns new ChannelConfig.
func CopyChannelConfig(c *ChannelConfig) *ChannelConfig {
	if c == nil {
		return &ChannelConfig{}
	}

	cp := *c

	return &cp
}

// CopyPacketServerConfig returns deep copy of c.
// If c is nil, it returns new PacketServerConfig.
func CopyPacketServerConfig(c *PacketServerConfig) *PacketServerConfig {
	if c == nil {
		return &PacketServerConfig{}
	}

	cp := *c

	return &cp
}

func isNoConnError(err error) bool {
	return err == ErrStopped || err == io.EOF
}
