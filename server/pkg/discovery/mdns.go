package discovery

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/mdns"
)

type MDNSAdapter struct{}

func (m *MDNSAdapter) Announce(ctx context.Context, serviceInfo ServiceInfo) error {
	host, _ := os.Hostname()
	info := []string{"Local file sender"}
	service, _ := mdns.NewMDNSService(host, serviceInfo.Name, "local", "", serviceInfo.Port, nil, info)

	server, _ := mdns.NewServer((&mdns.Config{Zone: service}))

	defer server.Shutdown()
	<-ctx.Done() // keep server running

	fmt.Println("Shutting down mDNS server")
	return nil
}

// windows does not support ipv6 multicast, so we need to use ipv4
func (m *MDNSAdapter) Discover(serviceType string, timeout time.Duration) (serviceInfo ServiceInfo, err error) {
	entriesCh := make(chan *mdns.ServiceEntry, 4)

	var service ServiceInfo

	go func() {
		for entry := range entriesCh {
			fmt.Printf("Got new entry: %v\n", entry)
			service = ServiceInfo{
				Name: entry.Name,
				Addr: entry.Addr,
				Port: entry.Port,
			}
		}
	}()
	// mdns.Lookup("file-share.local", entriesCh)
	mdns.Query(&mdns.QueryParam{
		Service:             "_file-share._tcp",
		Domain:              "local",
		Timeout:             timeout,
		Entries:             entriesCh,
		WantUnicastResponse: false,
		DisableIPv4:         false,
		DisableIPv6:         false,
	})
	defer close(entriesCh)

	return service, nil
}
