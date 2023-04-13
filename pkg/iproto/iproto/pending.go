package iproto

import "sync"

type store struct {
	mu      sync.Mutex
	hash    map[uint64]func([]byte, error)
	sync    uint32
	emptied chan struct{}
}

func newStore() *store {
	return &store{
		hash: make(map[uint64]func([]byte, error)),
	}
}

// push saves given cb to be called in future and returns sync id of this.
func (c *store) push(method uint32, cb func([]byte, error)) (sync uint32) {
	var code uint64

	c.mu.Lock()

	for {
		c.sync++

		// Glue method and sync bits to prevent collisions on different method
		// but with same sync.
		sync = c.sync

		code = uint64(method)<<32 | uint64(sync)
		if _, exists := c.hash[code]; !exists {
			break
		}
	}

	c.hash[code] = cb
	c.mu.Unlock()

	return
}

// resolve removes callback at given sync and calls it with data and err.
// It returns true is callback was called.
func (c *store) resolve(method uint32, sync uint32, data []byte, err error) (removed bool) {
	code := uint64(method)<<32 | uint64(sync)

	c.mu.Lock()

	cb, ok := c.hash[code]
	if !ok {
		c.mu.Unlock()
		return
	}

	delete(c.hash, code)

	if len(c.hash) == 0 {
		c.onEmptied()
	}

	c.mu.Unlock()

	cb(data, err)

	return ok
}

// rejectAll drops all pending requests with err.
func (c *store) rejectAll(err error) {
	c.mu.Lock()
	hash := c.hash

	// Do not swap hash with empty one because current hash is already empty.
	if len(hash) == 0 {
		c.mu.Unlock()
		return
	}

	c.hash = make(map[uint64]func([]byte, error))
	c.onEmptied()

	c.mu.Unlock()

	for _, cb := range hash {
		cb(nil, err)
	}
}

func (c *store) size() (ret int) {
	c.mu.Lock()
	ret = len(c.hash)
	c.mu.Unlock()

	return
}

// mutex must be held.
func (c *store) onEmptied() {
	if c.emptied != nil {
		close(c.emptied)
		c.emptied = nil
	}
}

// empty returns channel which closure signals about store reached the empty
// (that is, zero pending callbacks) state.
func (c *store) empty() <-chan struct{} {
	c.mu.Lock()
	ret := c.emptied

	switch {
	case len(c.hash) == 0:
		ret = closed
	case ret == nil:
		c.emptied = make(chan struct{})
		ret = c.emptied
	}

	c.mu.Unlock()

	return ret
}

var closed = func() chan struct{} {
	ch := make(chan struct{})
	close(ch)
	return ch
}()
