package common

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

func Wait(lock string) (err error) {
	var buf [syscall.SizeofInotifyEvent * 4096]byte

	fd, errno := syscall.InotifyInit()
	if fd == -1 {
		return os.NewSyscallError("inotify_init", errno)
	}
	_, err = syscall.InotifyAddWatch(fd, lock, syscall.IN_DELETE_SELF)
	if err != nil {
		return
	}

	n, err := syscall.Read(fd, buf[0:])
	if err != nil {
		return
	}
	if n < 0 {
		return os.NewSyscallError("read", err)
	}
	if n < syscall.SizeofInotifyEvent {
		return errors.New("inotify: short read in readEvents()")
	}
	offset := uint32(0)
	for offset <= uint32(n-syscall.SizeofInotifyEvent) {
		event := (*syscall.InotifyEvent)(unsafe.Pointer(&buf[offset]))
		if event.Mask&syscall.IN_DELETE_SELF != 0 {
			break
		} else {
			return errors.New(fmt.Sprintf("Error: unexpected event %v", event))
		}
	}

	return syscall.Close(fd)
}
