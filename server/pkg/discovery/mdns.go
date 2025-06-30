package discovery

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/brutella/dnssd"
)

type MDNSAdapter struct{}

func (m *MDNSAdapter) Announce(ctx context.Context, serviceInfo ServiceInfo) error {
	text := make(map[string]string)
	text["desc"] = "Local file sender"

	cfg := dnssd.Config{
		Name:   serviceInfo.Name,
		Type:   serviceInfo.Type,
		Domain: serviceInfo.Domain,
		// mdns will multicast to ip address, so we can leave it nil
		IPs:  nil,
		Text: text,
		Port: serviceInfo.Port,
	}

	service, err := dnssd.NewService(cfg)
	if err != nil {
		return fmt.Errorf("failed to create mDNS service: %w", err)
	}

	rp, err := dnssd.NewResponder()
	if err != nil {
		return fmt.Errorf("failed to create mDNS responder: %w", err)
	}

	rp.Add(service)
	rp.Respond(ctx)

	fmt.Println("Shutting down mDNS server")
	return nil
}

// windows does not support ipv6 multicast, so we need to use ipv4
func (m *MDNSAdapter) Discover(ctx context.Context, service string) (chan []ServiceInfo, error) {
	var (
		mu      sync.RWMutex
		entries = make(map[string]ServiceInfo)
		outCh   = make(chan []ServiceInfo, 10)
	)

	sendSnapshot := func() {
		mu.Lock()
		defer mu.Unlock()
		snapshot := make([]ServiceInfo, 0, len(entries))
		for _, entry := range entries {
			snapshot = append(snapshot, entry)
		}
		select {
		case outCh <- snapshot:
		default:
		}
	}

	addFn := func(e dnssd.BrowseEntry) {
		mu.Lock()
		log.Printf("Found service %s %s %d  %s in dns-sd", e.Name, e.Type, e.Port, e.IfaceName)
		entries[fmt.Sprintf("%s:%s:%s", e.Name, e.Type, e.Domain)] = ServiceInfo{
			Name:   e.Name,
			Type:   e.Type,
			Domain: e.Domain,
			Addr:   e.IPs[0],
			Port:   e.Port,
		}
		mu.Unlock()
		sendSnapshot()
	}

	rmvFn := func(e dnssd.BrowseEntry) {
		mu.Lock()
		delete(entries, fmt.Sprintf("%s:%s:%s", e.Name, e.Type, e.Domain))
		mu.Unlock()
		sendSnapshot()
	}

	go func() {
		dnssd.LookupType(ctx, service, addFn, rmvFn)
		close(outCh)
	}()

	return outCh, nil
}
