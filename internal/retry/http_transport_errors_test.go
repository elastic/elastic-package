// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package retry

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync/atomic"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCheckRetryTransientConnectionErrors feeds checkRetry the error shapes
// http.Client.Do produces for transport-level failures, which are always
// wrapped in *url.Error. Transient connection errors must be retried.
// Regression test for https://github.com/elastic/elastic-package/issues/3887.
func TestCheckRetryTransientConnectionErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"EOF (stale keep-alive close)", &url.Error{Op: "Post", URL: "https://127.0.0.1:5601/api/fleet/agent_policies", Err: io.EOF}},
		{"unexpected EOF", &url.Error{Op: "Post", URL: "https://127.0.0.1:5601/api/fleet/package_policies", Err: io.ErrUnexpectedEOF}},
		{"connection reset by peer", &url.Error{Op: "Get", URL: "https://127.0.0.1:5601/api/status", Err: syscall.ECONNRESET}},
		{"connection refused", &url.Error{Op: "Get", URL: "https://127.0.0.1:5601/api/status", Err: syscall.ECONNREFUSED}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			shouldRetry, err := checkRetry(context.Background(), nil, c.err)
			assert.NoError(t, err)
			assert.True(t, shouldRetry, "%s should be retried", c.name)
		})
	}
}

// TestCheckRetryUnrecoverableTransportErrors ensures the unrecoverable causes
// are still not retried when wrapped in *url.Error, as http.Client.Do returns
// them.
func TestCheckRetryUnrecoverableTransportErrors(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"certificate verification", &url.Error{Op: "Get", URL: "https://example.com", Err: &tls.CertificateVerificationError{Err: errors.New("bad cert")}}},
		{"unknown authority", &url.Error{Op: "Get", URL: "https://example.com", Err: x509.UnknownAuthorityError{}}},
		{"invalid certificate", &url.Error{Op: "Get", URL: "https://example.com", Err: &x509.CertificateInvalidError{Reason: x509.Expired}}},
		{"unsupported protocol scheme", &url.Error{Op: "Get", URL: "ftp://example.com", Err: errors.New(`unsupported protocol scheme "ftp"`)}},
		{"too many redirects", &url.Error{Op: "Get", URL: "http://example.com", Err: errTooManyRedirects}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			shouldRetry, err := checkRetry(context.Background(), nil, c.err)
			assert.NoError(t, err)
			assert.False(t, shouldRetry, "%s should not be retried", c.name)
		})
	}
}

// flakyConnectionServer accepts TCP connections and closes the first
// failures of them without writing a response — the observable behavior of a
// server-side keep-alive close race — and serves a minimal HTTP 200 response
// afterwards. It returns the address and a counter of accepted connections.
func flakyConnectionServer(t *testing.T, failures int32) (addr string, attempts *atomic.Int32) {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() })

	attempts = &atomic.Int32{}
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			n := attempts.Add(1)
			buf := make([]byte, 1024)
			_, _ = conn.Read(buf)
			if n <= failures {
				// Close without responding: the client sees EOF.
				conn.Close()
				continue
			}
			_, _ = conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 2\r\nConnection: close\r\n\r\nOK"))
			conn.Close()
		}
	}()

	return ln.Addr().String(), attempts
}

// TestConnectionErrorsRetriedEndToEnd reproduces the failure mode of
// https://github.com/elastic/elastic-package/issues/3887 end to end: a server
// that closes the connection without responding must be retried, and the
// request must succeed once the server recovers.
func TestConnectionErrorsRetriedEndToEnd(t *testing.T) {
	addr, attempts := flakyConnectionServer(t, 2)

	client := WrapHTTPClient(&http.Client{}, HTTPOptions{
		RetryMax:     5,
		retryWaitMin: fastRetryWaitMin,
		retryWaitMax: fastRetryWaitMax,
	})

	resp, err := client.Post("http://"+addr+"/api/fleet/agent_policies", "application/json", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, int32(3), attempts.Load(), "expected 2 failed attempts plus 1 success")
}

// TestConnectionErrorsRetriesExhausted verifies that a persistently failing
// connection gives up after RetryMax retries and surfaces the underlying
// error.
func TestConnectionErrorsRetriesExhausted(t *testing.T) {
	addr, attempts := flakyConnectionServer(t, int32(^uint32(0)>>1)) // always fail

	client := WrapHTTPClient(&http.Client{}, HTTPOptions{
		RetryMax:     2,
		retryWaitMin: fastRetryWaitMin,
		retryWaitMax: fastRetryWaitMax,
	})

	resp, err := client.Post("http://"+addr+"/api/fleet/agent_policies", "application/json", nil)
	if resp != nil {
		resp.Body.Close()
	}

	require.Error(t, err)
	assert.True(t, errors.Is(err, io.EOF), "expected EOF, got: %v", err)
	assert.Equal(t, int32(3), attempts.Load(), "expected initial attempt plus RetryMax retries")
}
