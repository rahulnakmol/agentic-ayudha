//go:build !linux

package verify

import (
	"errors"
	"os/exec"
)

func isolate(*exec.Cmd)         {}
func namespacesAvailable() bool { return false }
func loopbackUp() error         { return errors.New("network namespaces are Linux-only") }
func execve(string, []string, []string) error {
	return errors.New("the floor sandbox re-exec is Linux-only")
}
