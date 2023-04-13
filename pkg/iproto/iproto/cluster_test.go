package iproto

import (
	"net"
	"sort"
	"testing"

	"golang.org/x/net/context"
)

type LoggerFunc func(context.Context, string, ...interface{})

func (fn LoggerFunc) Printf(ctx context.Context, format string, args ...interface{}) {
	fn(ctx, format, args...)
}

func (fn LoggerFunc) Debugf(ctx context.Context, format string, args ...interface{}) {
	fn(ctx, format, args...)
}

func addrPool(addr string) *Pool {
	return &Pool{
		addr: addr,
	}
}

func addrsPools(addrs ...string) []*Pool {
	ret := make([]*Pool, len(addrs))
	for i, addr := range addrs {
		ret[i] = addrPool(addr)
	}
	return ret
}
func poolsAddrs(ps []*Pool) []string {
	ret := make([]string, len(ps))
	for i := range ret {
		ret[i] = ps[i].RemoteAddr().String()
	}
	return ret
}
func stringsEqual(a, b []string) bool {
	sort.Strings(a)
	sort.Strings(b)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestClusterReset(t *testing.T) {
	mp := poolsAddrs
	eq := stringsEqual
	for _, test := range []struct {
		name         string
		exist        []string
		reset        []string
		expResult    []string
		expUntouched []string
		expCreated   []string
		expRemoved   []string
		expIgnored   []string
	}{
		{
			reset:      []string{"a", "b", "c"},
			expResult:  []string{"a", "b", "c"},
			expCreated: []string{"a", "b", "c"},
		},
		{
			name:       "duplicates",
			reset:      []string{"a", "a", "b", "a", "c", "a"},
			expResult:  []string{"a", "b", "c"},
			expCreated: []string{"a", "b", "c"},
			expIgnored: []string{"a", "a", "a"},
		},
		{
			name:       "duplicates",
			exist:      []string{"x", "y", "z"},
			reset:      []string{"a", "a", "b", "a", "c", "a"},
			expResult:  []string{"a", "b", "c"},
			expCreated: []string{"a", "b", "c"},
			expRemoved: []string{"x", "y", "z"},
			expIgnored: []string{"a", "a", "a"},
		},
		{
			exist:      []string{"a", "b", "c"},
			expRemoved: []string{"a", "b", "c"},
		},
		{
			exist:        []string{"a", "b", "c"},
			reset:        []string{"a", "b", "c"},
			expResult:    []string{"a", "b", "c"},
			expUntouched: []string{"a", "b", "c"},
		},
		{
			exist:        []string{"a", "b", "c", "d"},
			reset:        []string{"b", "c"},
			expResult:    []string{"b", "c"},
			expUntouched: []string{"b", "c"},
			expRemoved:   []string{"a", "d"},
		},
		{
			exist:        []string{"b", "c"},
			reset:        []string{"a", "b", "c", "d"},
			expResult:    []string{"a", "b", "c", "d"},
			expUntouched: []string{"b", "c"},
			expCreated:   []string{"a", "d"},
		},
		{
			exist:        []string{"b", "c"},
			reset:        []string{"a", "c"},
			expResult:    []string{"a", "c"},
			expUntouched: []string{"c"},
			expRemoved:   []string{"b"},
			expCreated:   []string{"a"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			exist := addrsPools(test.exist...)
			rst := addrsPools(test.reset...)

			result, untouched, created, removed, ignored := reset(exist, rst)

			if act, exp := mp(result), test.expResult; !eq(act, exp) {
				t.Errorf("unexpected result: %v; want %v", act, exp)
			}
			if act, exp := mp(untouched), test.expUntouched; !eq(act, exp) {
				t.Errorf("unexpected untouched: %v; want %v", act, exp)
			}
			if act, exp := mp(created), test.expCreated; !eq(act, exp) {
				t.Errorf("unexpected created: %v; want %v", act, exp)
			}
			if act, exp := mp(removed), test.expRemoved; !eq(act, exp) {
				t.Errorf("unexpected removed: %v; want %v", act, exp)
			}
			if act, exp := mp(ignored), test.expIgnored; !eq(act, exp) {
				t.Errorf("unexpected ignored: %v; want %v", act, exp)
			}
		})
	}
}

func TestClusterDisableEnable(t *testing.T) {
	reset := func() *Cluster {
		c := new(Cluster)
		for _, p := range addrsPools("a", "b", "c") {
			if !c.Insert(p) {
				t.Fatalf("can not insert pool")
			}
		}
		return c
	}
	{
		c := reset()
		c.Disable(addrPool("b"))
		if act, exp := poolsAddrs(c.Peers()), []string{"a", "c"}; !stringsEqual(act, exp) {
			t.Fatalf("unexpected peers list after Disable(): %v; want %v", act, exp)
		}
		c.Enable(addrPool("b"))
		if act, exp := poolsAddrs(c.Peers()), []string{"a", "b", "c"}; !stringsEqual(act, exp) {
			t.Fatalf("unexpected peers list after Enable(): %v; want %v", act, exp)
		}
	}
	{
		c := reset()
		c.Disable(addrPool("b"))
		if !c.Remove(addrPool("b")) {
			t.Fatalf("Remove() disabled address is not okay; want okay")
		}
		c.Enable(addrPool("b"))
		if act, exp := poolsAddrs(c.Peers()), []string{"a", "c"}; !stringsEqual(act, exp) {
			t.Fatalf("unexpected peers list after Enable(removed item): %v; want %v", act, exp)
		}
	}
	{
		c := reset()
		c.Disable(addrPool("b"))
		if c.Insert(addrPool("b")) {
			t.Fatalf("Insert() disabled address is okay; want not")
		}
		_, removed, _ := c.Reset(addrsPools("a", "c"))
		if act, exp := poolsAddrs(removed), []string{"b"}; !stringsEqual(act, exp) {
			t.Fatalf("removed %v; want %v", act, exp)
		}
		c.Enable(addrPool("b"))
		if act, exp := poolsAddrs(c.Peers()), []string{"a", "c"}; !stringsEqual(act, exp) {
			t.Fatalf("unexpected peers list after Enable(removed item): %v; want %v", act, exp)
		}
	}
}

func getPeers(n int, handler func(int, Conn, Packet), onAccept func(int, *Channel)) ([]string, error) {
	addr := make([]string, n)
	for i := 0; i < n; i++ {
		ln, err := net.Listen("tcp", "127.0.0.1:")
		if err != nil {
			return nil, err
		}

		i := i // For closure.
		srv := &Server{
			ChannelConfig: &ChannelConfig{
				Handler: HandlerFunc(func(_ context.Context, conn Conn, pkt Packet) {
					if handler != nil {
						handler(i, conn, pkt)
					}
				}),
				Logger: LoggerFunc(func(_ context.Context, format string, args ...interface{}) {
					//log.Printf("[rebus test server] "+format, args...)
				}),
			},
			Accept: func(conn net.Conn, cfg *ChannelConfig) (*Channel, error) {
				ch := NewChannel(conn, cfg)
				if onAccept != nil {
					onAccept(i, ch)
				}
				return ch, nil
			},
		}

		//nolint:errcheck
		go srv.Serve(bg, ln)

		addr[i] = ln.Addr().String()
	}

	return addr, nil
}
