//go:build unix

package main

import (
	"net"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"
)

// Unix only: the test interrupts its own process with SIGINT, which Windows
// cannot deliver (os.Process.Signal(os.Interrupt) is unsupported there). On
// Windows, Ctrl+C reaches main through the same signal.NotifyContext(os.Interrupt).
// main serves until it is interrupted, then returns.
func TestMainServesUntilInterrupt(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	args := os.Args
	t.Cleanup(func() { os.Args = args })
	os.Args = []string{"benefits-qa", "-addr", addr, "-tokens", tokens}
	done := make(chan struct{})
	go func() { main(); close(done) }()
	for i := 0; ; i++ {
		resp, err := http.Get("http://" + addr + "/healthz")
		if err == nil {
			_ = resp.Body.Close()
			break
		}
		if i == 100 {
			t.Fatalf("main never served: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("main did not return after SIGINT")
	}
}
