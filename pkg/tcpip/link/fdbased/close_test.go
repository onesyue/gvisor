//go:build linux

package fdbased

import (
	"errors"
	"os"
	"testing"

	"github.com/metacubex/gvisor/pkg/tcpip/stack"
	"golang.org/x/sys/unix"
)

type idleDispatcher struct{ stack.NetworkDispatcher }

func TestDetachReclaimsReadVStopDescriptor(t *testing.T) {
	for i := 0; i < 20; i++ {
		r, w, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		ep, err := New(&Options{FDs: []int{int(r.Fd())}, MTU: 1500, ProcessorsPerChannel: 1})
		if err != nil {
			t.Fatal(err)
		}
		e := ep.(*endpoint)
		stop := e.inboundDispatchers[0].(*readVDispatcher).EFD
		ep.Attach(&idleDispatcher{})
		ep.Attach(nil)
		ep.Wait()
		ep.Close()
		if _, err := unix.FcntlInt(uintptr(stop), unix.F_GETFD, 0); !errors.Is(err, unix.EBADF) {
			t.Fatal("detach retained its eventfd", err)
		}
		if _, err := r.Stat(); err != nil {
			t.Fatal("endpoint closed caller fd", err)
		}
		r.Close()
		w.Close()
	}
}

func TestDetachReclaimsRecvMMsgDescriptors(t *testing.T) {
	// Warm the Go runtime poller before comparing actual live descriptors.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	r.Close()
	w.Close()
	before, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		pair, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_DGRAM|unix.SOCK_NONBLOCK, 0)
		if err != nil {
			t.Fatal(err)
		}
		ep, err := New(&Options{FDs: []int{pair[0]}, MTU: 1500, ProcessorsPerChannel: 1, PacketDispatchMode: RecvMMsg})
		if err != nil {
			t.Fatal(err)
		}
		ep.Attach(&idleDispatcher{})
		ep.Attach(nil)
		ep.Wait()
		ep.Close()
		unix.Close(pair[0])
		unix.Close(pair[1])
	}
	after, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Fatalf("dispatcher construction/detach leaked descriptors: before=%d after=%d", len(before), len(after))
	}
}
