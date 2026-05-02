package server

import (
	"crypto/tls"
	"net/http"
	"strings"

	"github.com/elazarl/goproxy"
	"github.com/sirupsen/logrus"
)

func requestLogFields(req *http.Request, ctx *goproxy.ProxyCtx) logrus.Fields {
	fields := logrus.Fields{
		"session":          ctx.Session,
		"method":           req.Method,
		"url":              req.URL.String(),
		"url_host":         req.URL.Host,
		"host":             req.Host,
		"proto":            req.Proto,
		"scheme":           req.URL.Scheme,
		"content_length":   req.ContentLength,
		"content_type":     req.Header.Get("Content-Type"),
		"content_encoding": req.Header.Get("Content-Encoding"),
		"accept":           req.Header.Get("Accept"),
		"accept_encoding":  req.Header.Get("Accept-Encoding"),
		"user_agent":       req.UserAgent(),
		"referer":          req.Referer(),
		"range":            req.Header.Get("Range"),
		"client_ip":        clientIP(req),
		"remote_addr":      req.RemoteAddr,
		"x_forwarded_for":  req.Header.Get("X-Forwarded-For"),
	}

	if req.URL != nil {
		fields["path"] = req.URL.Path
		fields["query"] = req.URL.RawQuery
	}

	if len(req.TransferEncoding) > 0 {
		fields["transfer_encoding"] = strings.Join(req.TransferEncoding, ",")
	}

	addClientTLSHandshakeFingerprint(fields, ctx)
	return fields
}

func responseLogFields(resp *http.Response, ctx *goproxy.ProxyCtx) logrus.Fields {
	fields := logrus.Fields{
		"session":             ctx.Session,
		"method":              ctx.Req.Method,
		"url":                 ctx.Req.URL.String(),
		"host":                ctx.Req.Host,
		"status":              resp.StatusCode,
		"status_text":         resp.Status,
		"proto":               resp.Proto,
		"content_length":      resp.ContentLength,
		"content_type":        resp.Header.Get("Content-Type"),
		"content_encoding":    resp.Header.Get("Content-Encoding"),
		"cache_control":       resp.Header.Get("Cache-Control"),
		"location":            resp.Header.Get("Location"),
		"server":              resp.Header.Get("Server"),
		"etag":                resp.Header.Get("ETag"),
		"last_modified":       resp.Header.Get("Last-Modified"),
		"content_disposition": resp.Header.Get("Content-Disposition"),
		"vary":                resp.Header.Get("Vary"),
		"uncompressed":        resp.Uncompressed,
	}

	if len(resp.TransferEncoding) > 0 {
		fields["transfer_encoding"] = strings.Join(resp.TransferEncoding, ",")
	}

	if resp.TLS != nil {
		fields["upstream_tls_version"] = tlsVersion(resp.TLS.Version)
		fields["upstream_tls_server"] = resp.TLS.ServerName
		fields["upstream_tls_alpn"] = resp.TLS.NegotiatedProtocol
	}

	addClientTLSHandshakeFingerprint(fields, ctx)
	return fields
}

func clientIP(req *http.Request) string {
	if forwardedFor := req.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		if len(parts) > 0 {
			return strings.TrimSpace(parts[0])
		}
	}

	return req.RemoteAddr
}

func tlsVersion(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS1.0"
	case tls.VersionTLS11:
		return "TLS1.1"
	case tls.VersionTLS12:
		return "TLS1.2"
	case tls.VersionTLS13:
		return "TLS1.3"
	default:
		return "unknown"
	}
}

func addClientTLSHandshakeFingerprint(fields logrus.Fields, ctx *goproxy.ProxyCtx) {
	if ctx.TLSClientHello == nil {
		return
	}

	hello := ctx.TLSClientHello
	fields["client_tls_client_hello_parsed"] = hello.Parsed != nil
	fields["client_tls_client_hello_raw_len"] = len(hello.Raw)
	if hello.JA3 != "" {
		fields["client_tls_ja3"] = hello.JA3
	}
	if hello.JA3Hash != "" {
		fields["client_tls_fingerprint"] = hello.JA3Hash
		fields["client_tls_ja3_hash"] = hello.JA3Hash
	}
}
