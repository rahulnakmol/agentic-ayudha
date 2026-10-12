//go:build linux

package verify

import (
	"os"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"
)

const capNetAdmin = 12

// isolate runs cmd in a new user and network namespace as the same uid, with
// CAP_NET_ADMIN kept only so the re-executed child can bring up loopback.
func isolate(cmd *exec.Cmd) {
	uid, gid := os.Getuid(), os.Getgid()
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:                 syscall.CLONE_NEWUSER | syscall.CLONE_NEWNET,
		UidMappings:                []syscall.SysProcIDMap{{ContainerID: uid, HostID: uid, Size: 1}},
		GidMappings:                []syscall.SysProcIDMap{{ContainerID: gid, HostID: gid, Size: 1}},
		GidMappingsEnableSetgroups: false,
		AmbientCaps:                []uintptr{capNetAdmin},
	}
}

var (
	nsOnce sync.Once
	nsOK   bool
)

// namespacesAvailable probes once whether unprivileged user and network
// namespaces work here (Ubuntu's AppArmor restriction can forbid them).
func namespacesAvailable() bool {
	nsOnce.Do(func() {
		self, err := os.Executable()
		if err != nil {
			return
		}
		cmd := exec.Command(self)
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), childMarker + `=["true"]`}
		isolate(cmd)
		nsOK = cmd.Run() == nil
	})
	return nsOK
}

// loopbackUp sets IFF_UP on lo in the current network namespace, then drops
// every ambient capability so the tool runs without CAP_NET_ADMIN.
func loopbackUp() error {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, 0)
	if err != nil {
		return err
	}
	defer func() { _ = syscall.Close(fd) }()
	var ifr [40]byte
	copy(ifr[:], "lo")
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.SIOCGIFFLAGS, uintptr(unsafe.Pointer(&ifr[0]))); e != 0 {
		return e
	}
	flags := *(*uint16)(unsafe.Pointer(&ifr[16]))
	*(*uint16)(unsafe.Pointer(&ifr[16])) = flags | syscall.IFF_UP | syscall.IFF_RUNNING
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.SIOCSIFFLAGS, uintptr(unsafe.Pointer(&ifr[0]))); e != 0 {
		return e
	}
	const prCapAmbient, prCapAmbientClearAll = 47, 4
	if _, _, e := syscall.RawSyscall(syscall.SYS_PRCTL, prCapAmbient, prCapAmbientClearAll, 0); e != 0 {
		return e
	}
	return nil
}

func execve(path string, argv, env []string) error { return syscall.Exec(path, argv, env) }
