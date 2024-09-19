//go:build unix

package apirouter

import (
	"fmt"
	"net"
	"os"

	"golang.org/x/sys/unix"
)

// MakeJsonSocketFD returns a file descriptor (integer) for a new json socket
func MakeJsonSocketFD(extraObjects map[string]any) (int, error) {
	fds, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		return -1, fmt.Errorf("failed to create socket pair: %w", err)
	}

	f := os.NewFile(uintptr(fds[1]), "pipe")
	defer f.Close()
	c, err := net.FileConn(f)
	if err != nil {
		return -1, fmt.Errorf("failed to handle socket: %w", err)
	}

	go handleJsonClient(c, extraObjects)

	return fds[0], nil
}
