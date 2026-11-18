// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package retry

import (
	"crypto/x509"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	fastRetryWaitMin = 1 * time.Millisecond
	fastRetryWaitMax = 5 * time.Millisecond
)

func TestRetryHTTPStatus(t *testing.T) {
	cases := []struct {
		title            string
		okHandler        http.Handler
		koHandler        http.Handler
		successRate      int
		retryMax         int
		expectedStatus   int
		expectedAttempts int
	}{
		{
			// Fails 4 times (503) then succeeds on the 5th request.
			title:            "eventually succeeds",
			retryMax:         10,
			successRate:      5,
			expectedStatus:   http.StatusOK,
			expectedAttempts: 5,
		},
		{
			// 502 is retryable: fails 4 times then succeeds on the 5th request.
			title:            "retries 502 until success",
			koHandler:        newStatusHandler("bad gateway", http.StatusBadGateway),
			retryMax:         10,
			successRate:      5,
			expectedStatus:   http.StatusOK,
			expectedAttempts: 5,
		},
		{
			title:            "no retries",
			retryMax:         0,
			successRate:      5,
			expectedStatus:   http.StatusServiceUnavailable,
			expectedAttempts: 1,
		},
		{
			// Initial attempt plus 2 retries, all 503, then gives up.
			title:            "not enough retries",
			retryMax:         2,
			successRate:      5,
			expectedStatus:   http.StatusServiceUnavailable,
			expectedAttempts: 3,
		},
		{
			// 404 is not retried, so only the initial attempt is made.
			title:            "retries on non-fatal error",
			koHandler:        http.NotFoundHandler(),
			retryMax:         10,
			successRate:      5,
			expectedStatus:   http.StatusNotFound,
			expectedAttempts: 1,
		},
		{
			title:            "no retries non-fatal error",
			koHandler:        http.NotFoundHandler(),
			retryMax:         0,
			successRate:      5,
			expectedStatus:   http.StatusNotFound,
			expectedAttempts: 1,
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			opts := HTTPOptions{
				RetryMax:     c.retryMax,
				RetryWaitMin: fastRetryWaitMin,
				RetryWaitMax: fastRetryWaitMax,
			}
			client := WrapHTTPClient(&http.Client{}, opts)

			server, handler := newFlakyTestServer(c.okHandler, c.koHandler, c.successRate)
			defer server.Close()

			resp, err := client.Get(server.URL)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, c.expectedStatus, resp.StatusCode)
			assert.Equal(t, c.expectedAttempts, handler.requests())
		})
	}
}

func TestUnrecoverableErrors(t *testing.T) {
	opts := HTTPOptions{
		RetryMax:     5,
		RetryWaitMin: fastRetryWaitMin,
		RetryWaitMax: fastRetryWaitMax,
	}

	t.Run("invalid URL", func(t *testing.T) {
		client := WrapHTTPClient(&http.Client{}, opts)
		_, err := client.Get("http::\\localhost")
		var urlError *url.Error
		assert.ErrorAs(t, err, &urlError)
	})

	t.Run("infinite redirects", func(t *testing.T) {
		server := httptest.NewServer(infiniteRedirectsHandler())
		defer server.Close()
		client := WrapHTTPClient(&http.Client{}, opts)

		_, err := client.Get(server.URL)
		assert.ErrorIs(t, err, errTooManyRedirects)
	})

	t.Run("unknown CA", func(t *testing.T) {
		server := httptest.NewTLSServer(newStatusHandler("OK", http.StatusOK))
		defer server.Close()
		client := WrapHTTPClient(&http.Client{}, opts)

		_, err := client.Get(server.URL)
		var caError x509.UnknownAuthorityError
		assert.ErrorAs(t, err, &caError)
	})

	t.Run("invalid certificate", func(t *testing.T) {
		// TODO: Actually produce an invalid certificate error with an actual server.
		expectedErr := &x509.CertificateInvalidError{Reason: x509.Expired}
		brokenClient := http.Client{
			Transport: &brokenTransport{err: expectedErr},
		}
		client := WrapHTTPClient(&brokenClient, opts)

		server := httptest.NewServer(newStatusHandler("OK", http.StatusOK))
		defer server.Close()

		_, err := client.Get(server.URL)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("consistent network error", func(t *testing.T) {
		expectedErr := errors.New("network error")
		brokenClient := http.Client{
			Transport: &brokenTransport{err: expectedErr},
		}
		client := WrapHTTPClient(&brokenClient, opts)

		server := httptest.NewServer(newStatusHandler("OK", http.StatusOK))
		defer server.Close()

		_, err := client.Get(server.URL)
		assert.ErrorIs(t, err, expectedErr)
	})
}

// flakyTestHandler deterministically succeeds only once every rate requests.
type flakyTestHandler struct {
	okHandler http.Handler
	koHanlder http.Handler
	rate      int

	mutex sync.Mutex
	count int
}

func (s *flakyTestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.count++

	if s.rate == 0 || s.count%s.rate != 0 {
		s.koHanlder.ServeHTTP(w, r)
		return
	}

	s.okHandler.ServeHTTP(w, r)
}

// requests returns the number of requests the handler has served, i.e. the
// initial attempt plus every retry.
func (s *flakyTestHandler) requests() int {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.count
}

func newFlakyTestServer(ok, ko http.Handler, rate int) (*httptest.Server, *flakyTestHandler) {
	if ok == nil {
		ok = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	}
	if ko == nil {
		ko = newStatusHandler("not available", http.StatusServiceUnavailable)
	}
	handler := &flakyTestHandler{
		okHandler: ok,
		koHanlder: ko,
		rate:      rate,
	}
	return httptest.NewServer(handler), handler
}

func newStatusHandler(msg string, code int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, msg, code)
	})
}

func infiniteRedirectsHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.String(), http.StatusMovedPermanently)
	})
}

type brokenTransport struct {
	err error
}

func (t *brokenTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, t.err
}
