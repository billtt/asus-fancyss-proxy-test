//go:build linux

package main

import (
	"context"
	"net"
	"syscall"
)

func listenLAN(ctx context.Context, address, iface string) (net.Listener, error) {
	lc := net.ListenConfig{Control: func(network, address string, c syscall.RawConn) error {
		var bindErr error
		e := c.Control(func(fd uintptr) {
			bindErr = syscall.SetsockoptString(int(fd), syscall.SOL_SOCKET, syscall.SO_BINDTODEVICE, iface)
		})
		if e != nil {
			return e
		}
		return bindErr
	}}
	return lc.Listen(ctx, "tcp4", address)
}
