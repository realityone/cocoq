package server

import (
	"net/http"
	"testing"

	"github.com/elazarl/goproxy"
)

func TestRequestLogFieldsIncludesClientTLSHandshakeFingerprint(t *testing.T) {
	req := newLoggingTestRequest(t)
	ctx := newLoggingTestProxyCtx(req, newLoggingTestTLSClientHello())

	fields := requestLogFields(req, ctx)

	assertClientTLSHandshakeFingerprintFields(t, fields)
}

func TestResponseLogFieldsIncludesClientTLSHandshakeFingerprint(t *testing.T) {
	req := newLoggingTestRequest(t)
	ctx := newLoggingTestProxyCtx(req, newLoggingTestTLSClientHello())
	resp := &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Proto:      "HTTP/1.1",
		Header:     make(http.Header),
	}

	fields := responseLogFields(resp, ctx)

	assertClientTLSHandshakeFingerprintFields(t, fields)
}

func newLoggingTestRequest(t *testing.T) *http.Request {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, "https://example.com/path", nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	return req
}

func newLoggingTestProxyCtx(req *http.Request, hello *goproxy.TLSClientHelloInfo) *goproxy.ProxyCtx {
	return &goproxy.ProxyCtx{
		Req:            req,
		TLSClientHello: hello,
	}
}

func newLoggingTestTLSClientHello() *goproxy.TLSClientHelloInfo {
	return &goproxy.TLSClientHelloInfo{
		Raw:     []byte{0x16, 0x03, 0x01, 0x00, 0x01, 0x01},
		JA3:     "771,4865-4866-4867,0-11-10,29-23-24,0",
		JA3Hash: "cd08e31494f9531f560d64c695473da9",
	}
}

func assertClientTLSHandshakeFingerprintFields(t *testing.T, fields map[string]any) {
	t.Helper()

	if got := fields["client_tls_fingerprint"]; got != "cd08e31494f9531f560d64c695473da9" {
		t.Fatalf("client_tls_fingerprint = %v, want cd08e31494f9531f560d64c695473da9", got)
	}
	if got := fields["client_tls_ja3"]; got != "771,4865-4866-4867,0-11-10,29-23-24,0" {
		t.Fatalf("client_tls_ja3 = %v, want JA3 string", got)
	}
	if got := fields["client_tls_ja3_hash"]; got != "cd08e31494f9531f560d64c695473da9" {
		t.Fatalf("client_tls_ja3_hash = %v, want cd08e31494f9531f560d64c695473da9", got)
	}
	if got := fields["client_tls_client_hello_parsed"]; got != false {
		t.Fatalf("client_tls_client_hello_parsed = %v, want false", got)
	}
	if got := fields["client_tls_client_hello_raw_len"]; got != 6 {
		t.Fatalf("client_tls_client_hello_raw_len = %v, want 6", got)
	}
}
