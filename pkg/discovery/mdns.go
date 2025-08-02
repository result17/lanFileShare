package discovery

import (
	"context"
	"fmt"
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

	if _, err = rp.Add(service); err != nil {
		return fmt.Errorf("failed to add mDNS service: %w", err)
	}

	if err = rp.Respond(ctx); err != nil {
		// Context cancellation is not an error in normal operation
		if err == context.Canceled {
			return nil
		}
		return fmt.Errorf("failed to respond to mDNS service: %w", err)
	}

	fmt.Println("Shutting down mDNS server")
	return nil
}

// DiscoveryResult contains either service info or an error
// DiscoverWithErrors returns a channel that can contain both services and errors
func (m *MDNSAdapter) Discover(ctx context.Context, service string) <-chan DiscoveryResult {
	var (
		mu      sync.RWMutex
		entries = make(map[string]ServiceInfo)
		outCh   = make(chan DiscoveryResult, 10)
	)

	sendSnapshot := func() {
		mu.Lock()
		defer mu.Unlock()
		snapshot := make([]ServiceInfo, 0, len(entries))
		for _, entry := range entries {
			snapshot = append(snapshot, entry)
		}
		select {
		case outCh <- DiscoveryResult{Services: snapshot, Error: nil}:
		default:
		}
	}

	sendError := func(err error) {
		select {
		case outCh <- DiscoveryResult{Services: nil, Error: err}:
		default:
		}
	}

	addFn := func(e dnssd.BrowseEntry) {
		mu.Lock()
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
		defer close(outCh)
		if err := dnssd.LookupType(ctx, service, addFn, rmvFn); err != nil {
			sendError(fmt.Errorf("mDNS lookup failed: %w", err))
		}
	}()

	return outCh
}
