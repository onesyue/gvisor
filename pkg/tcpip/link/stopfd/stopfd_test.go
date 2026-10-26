//go:build linux

package stopfd

import (
	"errors"
	"sync"
	"testing"

	"golang.org/x/sys/unix"
)

func TestCloseAndLaterStopCannotTouchReusedDescriptor(t *testing.T) {
	sf, err := New()
	if err != nil {
		t.Fatal(err)
	}
	sf.Stop()
	sf.Close()
	victim, err := unix.Eventfd(0, unix.EFD_NONBLOCK|unix.EFD_CLOEXEC)
	if err != nil {
		t.Fatal(err)
	}
	if victim != sf.EFD {
		if err := unix.Dup3(victim, sf.EFD, unix.O_CLOEXEC); err != nil {
			t.Fatal(err)
		}
		unix.Close(victim)
		victim = sf.EFD
	}
	defer unix.Close(victim)
	sf.Stop()
	sf.Close()
	var buf [8]byte
	if _, err := unix.Read(victim, buf[:]); !errors.Is(err, unix.EAGAIN) {
		t.Fatalf("old owner wrote to or closed reused descriptor: %v", err)
	}
}
func TestConcurrentStopAndClose(t *testing.T) {
	for i := 0; i < 100; i++ {
		sf, err := New()
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		wg.Add(2)
		go func() { defer wg.Done(); sf.Stop() }()
		go func() { defer wg.Done(); sf.Close() }()
		wg.Wait()
		sf.Close()
		if _, err := unix.FcntlInt(uintptr(sf.EFD), unix.F_GETFD, 0); !errors.Is(err, unix.EBADF) {
			t.Fatal("retained stop descriptor", err)
		}
	}
}
