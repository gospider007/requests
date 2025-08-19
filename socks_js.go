//go:build js

package requests

import (
	"syscall"
)

func Control(network, address string, c syscall.RawConn) error {
	return c.Control(func(fd uintptr) {})
}
