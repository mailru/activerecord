package iproto

import (
	"math/rand"
	"testing"
)

func TestPending(t *testing.T) {
	for _, test := range []struct {
		name    string
		push    int
		resolve int
	}{
		{"base", 10, 10},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := newStore()

			pending := make([][2]uint32, test.push)
			for i, v := range rand.Perm(test.push) {
				method := uint32(v)
				pending[i] = [2]uint32{method, s.push(method, func(data []byte, _ error) {})}
			}
			if sz := s.size(); sz != test.push {
				t.Errorf("after push %d items size() is %v; want %v", test.push, sz, test.push)
			}

			diff := test.push - test.resolve
			emptied := s.empty()

			for _, i := range rand.Perm(test.resolve) {
				method, sync := pending[i][0], pending[i][1]
				s.resolve(method, sync, nil, nil)
			}
			if sz := s.size(); sz != diff {
				t.Errorf(
					"after push %d items and resolve %d, size() is %v; want %v",
					test.push, test.resolve, sz, diff,
				)
			}
			if diff == 0 {
				select {
				case <-emptied:
				default:
					t.Errorf("emptied was not noticed after resolving all requests")
				}
				select {
				case <-s.empty():
				default:
					t.Errorf("empty store empty() returned non closed channel")
				}
			}
		})
	}
}
