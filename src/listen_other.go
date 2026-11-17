//go:build !linux

package main

import (
	"context"
	"errors"
	"net"
)

func listenLAN(ctx context.Context, address, iface string) (net.Listener, error) {
	return nil, errors.New("production listener requires Linux SO_BINDTODEVICE")
}
