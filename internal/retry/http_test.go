// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package retry

import (
	"errors"
	"net/http"
	"net/http/httptest"
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
		title          string
		okHandler      http.Handler
		koHandler      http.Handler
		successRate    int
		retryMax       int
		expectedStatus int
	}{
		{
			title:          "eventually succeeds",
			retryMax:       10,
			successRate:    5,
			expectedStatus: http.StatusOK,
		},
		{
			title:          "no retries",
			retryMax:       0,
			successRate:    5,
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			title:          "not enough retries",
			retryMax:       2,
			successRate:    5,
			expectedStatus: http.StatusServiceUnavailable,
		},
		{
			title:          "retries on non-fatal error",
			koHandler:      http.NotFoundHandler(),
			retryMax:       10,
			successRate:    5,
			expectedStatus: http.StatusNotFound,
		},
		{
			title:          "no retries non-fatal error",
			koHandler:      http.NotFoundHandler(),
			retryMax:       0,
			successRate:    5,
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, c := range cases {
		t.Run(c.title, func(t *testing.T) {
			opts := HTTPOptions{
				RetryMax:     c.retryMax,
				retryWaitMin: fastRetryWaitMin,
				retryWaitMax: fastRetryWaitMax,
			}
			client := WrapHTTPClient(&http.Client{}, opts)

			server := newFlakyTestServer(c.okHandler, c.koHandler, c.successRate)
			defer server.Close()

			resp, err := client.Get(server.URL)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, c.expectedStatus, resp.StatusCode)
		})
	}
}

func TestUnrecoverableErrors(t *testing.T) {
	opts := HTTPOptions{
		RetryMax:     5,
		retryWaitMin: fastRetryWaitMin,
		retryWaitMax: fastRetryWaitMax,
	}

	t.Run("invalid URL", func(t *testing.T) {
		client := WrapHTTPClient(&http.Client{}, opts)
		_, err := client.Get("http::\\localhost")
		assert.Error(t, err)
	})

	t.Run("infinite redirects", func(t *testing.T) {
		server := httptest.NewServer(infiniteRedirectsHandler())
		defer server.Close()
		client := WrapHTTPClient(&http.Client{}, opts)

		_, err := client.Get(server.URL)
		assert.Error(t, err)
	})

	t.Run("unknown CA", func(t *testing.T) {
		server := httptest.NewTLSServer(newStatusHandler("OK", http.StatusOK))
		defer server.Close()
		client := WrapHTTPClient(&http.Client{}, opts)

		_, err := client.Get(server.URL)
		assert.Error(t, err)
	})

	t.Run("invalid certificate", func(t *testing.T) {
		t.Skip("TODO")
	})

	t.Run("network error", func(t *testing.T) {
		brokenClient := http.Client{
			Transport: &brokenTransport{},
		}
		client := WrapHTTPClient(&brokenClient, opts)

		server := httptest.NewServer(newStatusHandler("OK", http.StatusOK))
		defer server.Close()

		_, err := client.Get(server.URL)
		assert.Error(t, err)
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
	}

	s.okHandler.ServeHTTP(w, r)
}

func newFlakyTestServer(ok, ko http.Handler, rate int) *httptest.Server {
	if ok == nil {
		ok = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	}
	if ko == nil {
		ko = newStatusHandler("not available", http.StatusServiceUnavailable)
	}
	handler := flakyTestHandler{
		okHandler: ok,
		koHanlder: ko,
		rate:      rate,
	}
	return httptest.NewServer(&handler)
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

type brokenTransport struct{}

func (*brokenTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("network error")
}
