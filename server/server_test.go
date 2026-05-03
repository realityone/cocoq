package server

import (
	"crypto/tls"
	"encoding/json"
	"strings"
	"testing"

	appconfig "github.com/realityone/cocoq/config"
	"github.com/realityone/cocoq/server/database/dbrt"
)

func TestResolveAPIServiceFactoryMapsConfiguredServices(t *testing.T) {
	tests := []struct {
		name       string
		wantDomain string
	}{
		{
			name:       appconfig.APIServiceAnthropic,
			wantDomain: "api.anthropic.com",
		},
		{
			name:       appconfig.APIServiceOpenRouter,
			wantDomain: "openrouter.ai",
		},
		{
			name:       appconfig.APIServicePoe,
			wantDomain: "api.poe.com",
		},
		{
			name:       "",
			wantDomain: "openrouter.ai",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factory, err := resolveAPIServiceFactory(tt.name)
			if err != nil {
				t.Fatalf("resolveAPIServiceFactory() error = %v", err)
			}

			service, err := factory(tls.Certificate{}, nil, nil)
			if err != nil {
				t.Fatalf("factory() error = %v", err)
			}
			if !service.Domains().Has(tt.wantDomain) {
				t.Fatalf("service domains = %v, want %q", service.Domains().UnsortedList(), tt.wantDomain)
			}
		})
	}
}

func TestNewAPIServicesInstallsMultipleConfiguredServices(t *testing.T) {
	services, err := newAPIServices(tls.Certificate{}, nil, []appconfig.APIServiceConfig{
		{Name: appconfig.APIServiceAnthropic},
		{Name: appconfig.APIServicePoe},
		{
			Name:    appconfig.APIServiceOpenRouter,
			Options: json.RawMessage(`{"provider":"openai"}`),
		},
	})
	if err != nil {
		t.Fatalf("newAPIServices() error = %v", err)
	}
	if len(services) != 3 {
		t.Fatalf("len(services) = %d, want 3", len(services))
	}
	if services[0].Domains().Has("openrouter.ai") {
		t.Fatalf("anthropic service domains = %v, unexpectedly included openrouter.ai", services[0].Domains().UnsortedList())
	}
	if !services[0].Domains().Has("api.anthropic.com") {
		t.Fatalf("anthropic service domains = %v, want api.anthropic.com", services[0].Domains().UnsortedList())
	}
	if !services[1].Domains().Has("api.poe.com") {
		t.Fatalf("poe service domains = %v, want api.poe.com", services[1].Domains().UnsortedList())
	}
	if !services[2].Domains().Has("openrouter.ai") {
		t.Fatalf("openrouter service domains = %v, want openrouter.ai", services[2].Domains().UnsortedList())
	}
}

func TestNewAPIServicesRejectsInvalidServiceOptions(t *testing.T) {
	_, err := newAPIServices(tls.Certificate{}, nil, []appconfig.APIServiceConfig{
		{
			Name:    appconfig.APIServiceOpenRouter,
			Options: json.RawMessage(`{"provider":`),
		},
	})
	if err == nil {
		t.Fatal("newAPIServices() error = nil, want invalid options error")
	}
	if !strings.Contains(err.Error(), "decode openrouter API service options") {
		t.Fatalf("newAPIServices() error = %v, want openrouter options error", err)
	}
}

func TestNewAPIServicesRejectsDuplicateService(t *testing.T) {
	_, err := newAPIServices(tls.Certificate{}, nil, []appconfig.APIServiceConfig{
		{Name: appconfig.APIServiceOpenRouter},
		{Name: " OpenRouter "},
	})
	if err == nil {
		t.Fatal("newAPIServices() error = nil, want duplicate API service error")
	}
	if !strings.Contains(err.Error(), `duplicate API service "openrouter"`) {
		t.Fatalf("newAPIServices() error = %v, want duplicate openrouter error", err)
	}
}

func TestNewAPIServicesDefaultsToOpenRouter(t *testing.T) {
	services, err := newAPIServices(tls.Certificate{}, nil, nil)
	if err != nil {
		t.Fatalf("newAPIServices() error = %v", err)
	}
	if len(services) != 1 {
		t.Fatalf("len(services) = %d, want 1", len(services))
	}
	if !services[0].Domains().Has("openrouter.ai") {
		t.Fatalf("service domains = %v, want openrouter.ai", services[0].Domains().UnsortedList())
	}
}

func TestNewRejectsUnsupportedAPIService(t *testing.T) {
	_, err := New(appconfig.ServerConfig{
		RootDir: t.TempDir(),
		APIServices: []appconfig.APIServiceConfig{
			{Name: "unknown"},
		},
	}, &dbrt.Client{})
	if err == nil {
		t.Fatal("New() error = nil, want unsupported API service error")
	}
	if !strings.Contains(err.Error(), `unsupported API service "unknown"`) {
		t.Fatalf("New() error = %v, want unsupported API service error", err)
	}
}
