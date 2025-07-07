package discovery

import (
	"context"
	"net"
)

const (
	DefaultServerType = "_file-sharing._tcp"
	DefaultDomain     = "local"
)

type ServiceInfo struct {
	Name   string // hostname or instance name
	Type   string // service name, e.g., "_file-sharing._tcp"
	Domain string // domain, e.g., "local"
	Addr   net.IP
	Port   int
}

type Adapter interface {
	Announce(ctx context.Context, service ServiceInfo) error
	Discover(ctx context.Context, service string) (chan []ServiceInfo, error)
}
