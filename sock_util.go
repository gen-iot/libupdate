package libupdate

import (
	"os"
	"syscall"
)

// fd[0] for parent process
// fd[1] for child process
// nonblock : set socket nonblock
func sockPair(nonblock bool) (parent, child *os.File, err error) {
	syscall.ForkLock.Lock()
	defer syscall.ForkLock.Unlock()
	fds, err := syscall.Socketpair(syscall.AF_UNIX, syscall.SOCK_STREAM, 0)
	if err != nil {
		return
	}
	for _, fd := range fds {
		// cmd.Exec will dup fd , so we must set all paired_fd to closeOnExec
		// duped fd doesnt contain O_CLOEXEC
		syscall.CloseOnExec(fd)
		_ = syscall.SetNonblock(fd, nonblock)
	}
	parent = os.NewFile(uintptr(fds[0]), "")
	child = os.NewFile(uintptr(fds[1]), "")
	return
}
