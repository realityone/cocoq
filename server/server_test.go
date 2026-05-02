package server

import (
	"testing"

	"github.com/elazarl/goproxy"
)

func TestConfigureUpstreamTLSUsesUTLS(t *testing.T) {
	proxy := goproxy.NewProxyHttpServer()

	configureUpstreamTLS(proxy)

	if proxy.UpstreamTLSClientHelloID == nil {
		t.Fatal("UpstreamTLSClientHelloID was not configured")
	}
	if got := proxy.UpstreamTLSClientHelloID.Str(); got != defaultUpstreamTLSClientHelloID.Str() {
		t.Fatalf("UpstreamTLSClientHelloID = %q, want %q", got, defaultUpstreamTLSClientHelloID.Str())
	}
	if proxy.Tr == nil || proxy.Tr.DialTLSContext == nil {
		t.Fatal("proxy transport was not configured with a TLS dialer")
	}
}
