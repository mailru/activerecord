package iproto

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"unsafe"

	"golang.org/x/net/context"
)

// Cluster is a wrapper around unique slice of Pools. It helps to atomically
// read and modify that slice. The uniqueness criteria is pool remote address.
type Cluster struct {
	mu sync.Mutex

	// Used as atomic storage for []*Pool.
	peers unsafe.Pointer
	// Mapping of temporarily disabled pools.
	disabled map[poolAddr]*Pool

	terminated bool

	onBeforeChange []func([]*Pool)
}

// OnBeforeChange registers given callback to be called right before peers list
// changes. Note that it will be called under Cluster inner mutex held.
func (c *Cluster) OnBeforeChange(cb func([]*Pool)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onBeforeChange = append(c.onBeforeChange, cb)
}

// Reset makes cluster peers list to be equal to the given slice of Pools.
// It tries to leave existing pools untouched if they are present in ps. It
// returns three slices of Pools: first one is those which were mixed in to the
// existing list and probably must to be initialized; the second one is those
// which were removed from the existing list and probably must to be closed;
// the third one is thoe which were ignored from the given slice due to
// deduplication.
//
// That is, it provides ability to merge two slices of Pools – currently
// existing and new one.
//
// So the most common way to use Reset() is to call it with slice of
// uninitialized pools, postponing their initialization to the moment when
// Reset() returns the slice of pools which are actually should be initialized.
//
// Note that it compares pools by their remote address, not by equality of
// values (pointer value).
func (c *Cluster) Reset(ps []*Pool) (inserted, removed, ignored []*Pool) {
	return c.reset(ps, false)
}

// ResetAndDisableInserted is the same as Reset but it does not enables
// inserted peers. Instead, it marks inserted peers as disabled and lets caller
// to Enable() them later.
func (c *Cluster) ResetAndDisableInserted(ps []*Pool) (inserted, removed, ignored []*Pool) {
	return c.reset(ps, true)
}

// Insert inserts given pool to the peer list. It returns a flag that become
// true if there was no pool with the same remote address in the list before.
func (c *Cluster) Insert(p *Pool) (inserted bool) {
	addr := p.remoteAddr()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.terminated {
		return false
	}

	if _, ok := c.disabled[addr]; ok {
		return false
	}

	return c.insert(p)
}

// Remove removes given pool from the peer list. It returns a flag that become
// true if there was pool with the same remote address in the list before.
func (c *Cluster) Remove(p *Pool) (removed bool) {
	addr := p.remoteAddr()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.terminated {
		return false
	}

	if _, ok := c.disabled[addr]; ok {
		delete(c.disabled, addr)
		return true
	}

	return c.remove(p)
}

func (c *Cluster) Disable(p *Pool) bool {
	addr := p.remoteAddr()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.terminated {
		return false
	}

	if !c.remove(p) {
		return false
	}

	c.initDisabled()
	c.disabled[addr] = p

	return true
}

func (c *Cluster) Enable(p *Pool) bool {
	addr := p.remoteAddr()

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.terminated {
		return false
	}

	if _, ok := c.disabled[addr]; !ok {
		return false
	}

	delete(c.disabled, addr)

	return c.insert(p)
}

// Peers returns current peer list. Note that it is for read only.
func (c *Cluster) Peers() []*Pool {
	return c.readPeers()
}

// Shutdown calls Shutdown() on every registered pool.
func (c *Cluster) Shutdown() {
	c.terminate((*Pool).Shutdown)
}

// Close calls Close() on every registered pool.
func (c *Cluster) Close() {
	c.terminate((*Pool).Close)
}

func (c *Cluster) initDisabled() {
	if c.disabled == nil {
		c.disabled = make(map[poolAddr]*Pool)
	}
}

func (c *Cluster) reset(ps []*Pool, disableInserted bool) (inserted, removed, ignored []*Pool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.terminated {
		return
	}

	peers := c.readPeers()
	// Append disabled peers to the merge source list.
	// If they must be deleted they can not be enabled later.
	for _, disabled := range c.disabled {
		peers = append(peers, disabled)
	}

	result, untouched, inserted, removed, ignored := reset(peers, ps)
	for _, p := range removed {
		addr := p.remoteAddr()
		// If peer p was disabled before and now become removed – it can
		// not be enabled later.
		delete(c.disabled, addr)
	}

	if disableInserted {
		// Must mark insreted peers as disabled to make possible further enabling
		// by a caller.
		c.initDisabled()

		for _, p := range inserted {
			addr := p.remoteAddr()
			c.disabled[addr] = p
		}
		// Must store only untouched part of previous peers.
		// That is, inserted peers will be enabled later.
		peers = untouched
	} else {
		// Store untouched and inserted peers.
		peers = result
	}

	// Must call under mutex preventing non-consistent behaviour.
	c.emitBeforeChange(peers)
	c.storePeers(peers)

	return inserted, removed, ignored
}

// mutex c.mu must be held.
func (c *Cluster) insert(p *Pool) (inserted bool) {
	peers := c.readPeers()

	x := indexPeer(peers, p)
	if x != -1 {
		return false
	}

	cp := make([]*Pool, len(peers)+1)
	x = copy(cp, peers)
	cp[x] = p

	c.storePeers(cp)

	return true
}

// mutex c.mu must be held.
func (c *Cluster) remove(p *Pool) (removed bool) {
	peers := c.readPeers()

	x := indexPeer(peers, p)
	if x == -1 {
		return false
	}

	cp := make([]*Pool, len(peers)-1)
	copy(cp[:x], peers[:x])
	copy(cp[x:], peers[x+1:])

	c.storePeers(cp)

	return true
}

func (c *Cluster) readPeers() []*Pool {
	p := atomic.LoadPointer(&c.peers)
	if p != nil {
		return *(*[]*Pool)(p)
	}

	return nil
}

func (c *Cluster) storePeers(p []*Pool) {
	//nolint:gosec
	atomic.StorePointer(&c.peers, unsafe.Pointer(&p))
}

func (c *Cluster) terminate(method func(p *Pool)) {
	c.mu.Lock()
	c.terminated = true
	old := c.readPeers()
	c.storePeers(nil)
	c.mu.Unlock()

	var wg sync.WaitGroup
	for _, p := range old {
		wg.Add(1)

		p := p // For closure.

		go func() {
			defer wg.Done()
			method(p)
		}()
	}

	wg.Wait()
}

func (c *Cluster) emitBeforeChange(peers []*Pool) {
	for _, cb := range c.onBeforeChange {
		cb(peers)
	}
}

func indexPeer(ps []*Pool, p *Pool) int {
	x := -1

	for i, exist := range ps {
		if cmpRemoteAddr(exist, p) == 0 {
			x = i
			break
		}
	}

	return x
}

func reset(dst, src []*Pool) (result, untouched, inserted, removed, ignored []*Pool) {
	// Prepare slice to have enough space for merging two lists.
	ps := make([]*Pool, len(dst)+len(src))
	// mid is an index in the resulting slice. Elements that are stored to the
	// left of it are created or already present in dst; to the right – removed
	// elements. Mid is shifting.
	mid := len(src)
	// Copy current elements to the right half of new slice.
	// This will help us to put removed items to the right, and created Pool to
	// left without extra moving.
	copy(ps[mid:], dst)
	copy(ps[:mid], src)
	// Sort halfs separately.
	sort.SliceStable(ps[:mid], func(i, j int) bool {
		return cmpRemoteAddr(ps[i], ps[j]) < 0
	})
	sort.SliceStable(ps[mid:], func(i, j int) bool {
		return cmpRemoteAddr(ps[mid+i], ps[mid+j]) < 0
	})

	var (
		// dp is the first deduped item index.
		dp = 0
		// rm is an last removed item index.
		rm = len(ps)
		// cr is a last created item index.
		cr = mid
		// i is a right bound of the source items.
		i = mid - 1
		// j is a right bound of the destination items.
		j = len(ps) - 1
	)

	// remove marks as removed an existing peer.
	remove := func() {
		rm--
		ps[rm], ps[j] = ps[j], ps[rm]
	}
	// create marks as created an upcoming peer.
	create := func() {
		cr--
		ps[cr], ps[i] = ps[i], ps[cr]
	}
	// dedup returns sign of equality of current processing source item with
	// previous processed source item.
	dedup := func() bool {
		dup := i < mid-1 && cmpRemoteAddr(ps[i+1], ps[i]) == 0
		if !dup {
			return false
		}

		tmp := ps[i]

		copy(ps[dp+1:i+1], ps[dp:i])
		ps[dp] = tmp
		dp++

		return true
	}

	for i >= dp && j >= mid {
		switch cmpRemoteAddr(ps[i], ps[j]) {
		case -1:
			// Must remove this peer.
			remove()
			j--
		case 1:
			// Must check that previous source addr is not the same as current.
			if dedup() {
				continue
			}
			// This addr will become a new peer.
			create()
			i--
		default:
			// This peer stays in the cluster untouched.
			// Do nothing.
			j--
			i--
		}
	}
	// Must remove remaining peers if any.
	for j >= mid {
		remove()
		j--
	}

	for i >= dp {
		if dedup() {
			continue
		}

		create()
		i--
	}

	return ps[cr:rm], ps[mid:rm], ps[cr:mid], ps[rm:], ps[:dp]
}

// cmpRemoteAddr compares Pool by RemoteAddr().
func cmpRemoteAddr(ap, bp *Pool) int {
	a := ap.remoteAddr()
	b := bp.remoteAddr()
	an := a.network
	bn := b.network

	switch {
	case an < bn:
		return -1
	case an > bn:
		return 1
	}

	as := a.addr
	bs := b.addr

	switch {
	case as < bs:
		return -1
	case as > bs:
		return 1
	}

	return 0
}

// Unicast wraps Cluster for sending messages to any single cluster's peers.
type Unicast struct {
	*Cluster

	// Select contains logic of picking Pool from cluster's peers for further
	// Call/Notify call.
	// If Select is nil, then round-robin algorithm is used.
	Select func(context.Context, *Cluster) (*Pool, error)
	rr     roundRobin
}

// Call selects peer form cluster's peers and makes Call() on it.
func (u *Unicast) Call(ctx context.Context, method uint32, data []byte) ([]byte, error) {
	_, resp, err := u.CallNext(ctx, method, data)
	return resp, err
}

// CallNext selects peer form cluster's peers and makes Call() on it.
func (u *Unicast) CallNext(ctx context.Context, method uint32, data []byte) (peer *Pool, resp []byte, err error) {
	peer, err = u.selectPeer(ctx)
	if err == nil {
		resp, err = peer.Call(ctx, method, data)
	}

	return
}

// Notify selects peer form cluster's peers and makes Notify() on it.
func (u *Unicast) Notify(ctx context.Context, method uint32, data []byte) error {
	peer, err := u.selectPeer(ctx)
	if err != nil {
		return err
	}

	return peer.Notify(ctx, method, data)
}

// Send selects peer form cluster's peers and makes Send() on it.
func (u *Unicast) Send(ctx context.Context, pkt Packet) error {
	peer, err := u.selectPeer(ctx)
	if err != nil {
		return err
	}

	return peer.Send(ctx, pkt)
}

var (
	ErrEmptyPeers = fmt.Errorf("empty peers")
	ErrNoPeer     = fmt.Errorf("no peer")
)

func (u *Unicast) selectPeer(ctx context.Context) (p *Pool, err error) {
	if u.Select != nil {
		p, err = u.Select(ctx, u.Cluster)
	} else {
		p, err = u.rr.Select(ctx, u.Cluster)
	}

	if err != nil {
		return nil, err
	}

	if p == nil {
		return nil, ErrNoPeer
	}

	return p, nil
}

type roundRobin int32

func (r *roundRobin) Select(_ context.Context, cluster *Cluster) (*Pool, error) {
	ps := cluster.Peers()
	if len(ps) == 0 {
		return nil, ErrEmptyPeers
	}

	n := len(ps)
	i := atomic.AddInt32((*int32)(r), 1)
	i &= math.MaxInt32 // Avoid negative values.

	return ps[int(i)%n], nil
}
