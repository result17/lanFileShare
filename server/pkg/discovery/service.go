package discovery

import (
	"context"
	"net"
	"time"
)

const (
	DefaultServerName = "_file-sharing._tcp.local"
)

type ServiceInfo struct {
	Name string
	Addr net.IP
	Port int
}

type ServiceDiscovery interface {
	Announce(ctx context.Context, service ServiceInfo) error
	Discover(serviceType string, timeout time.Duration) (serviceInfo ServiceInfo, err error)
}
