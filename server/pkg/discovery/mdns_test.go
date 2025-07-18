package discovery

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestServer_StartStop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mdnsAdapter := &MDNSAdapter{}
	serviceInfo := ServiceInfo{
		Name:   "test-instance",
		Type:   "_test-service._tcp",
		Domain: "local",
		Addr:   nil,
		Port:   8080,
	}

	done := make(chan struct{})

	errCh := make(chan error, 1)
	go func() {
		err := mdnsAdapter.Announce(ctx, serviceInfo)
		errCh <- err
		close(done)
	}()

	time.Sleep(50 * time.Millisecond) // Allow some time for the service to be announced

	cancel()

	select {
	case <-done:
		if err := <-errCh; err != nil {
			t.Fatalf("Failed to announce service: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("Service announcement did not complete in time")
	}
}

func TestMDNSAdapter_Discover(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	mdnsAdapter := &MDNSAdapter{}

	serviceInfo := ServiceInfo{
		Name:   "test-instance",
		Type:   "_test-service._tcp",
		Domain: "local",
		Addr:   nil,
		Port:   8080,
	}

	go func() {
		_ = mdnsAdapter.Announce(ctx, serviceInfo)
	}()
	time.Sleep(300 * time.Millisecond)
	// Allow some time for the service to be announced
	queryCtx, queryCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer queryCancel()

	service := fmt.Sprintf("%s.%s.", serviceInfo.Type, serviceInfo.Domain)
	outCh, err := mdnsAdapter.Discover(queryCtx, service)
	if err != nil {
		t.Fatalf("Failed to discover service: %v", err)
	}
	defer close(outCh)
	discoveredService := <-outCh
	assert.Equalf(t, serviceInfo.Name, discoveredService[0].Name,
		"Expected service instance %s, got %s", serviceInfo.Name, discoveredService[0].Name)

	assert.Equalf(t, serviceInfo.Type, discoveredService[0].Type,
		"Expected service name %s, got %s", serviceInfo.Type, discoveredService[0].Type)

	assert.Equalf(t, serviceInfo.Domain, discoveredService[0].Domain,
		"Expected service domain %s, got %s", serviceInfo.Domain, discoveredService[0].Domain)

	assert.Equalf(t, serviceInfo.Port, discoveredService[0].Port,
		"Expected service port %d, got %d", serviceInfo.Port, discoveredService[0].Port)
}
