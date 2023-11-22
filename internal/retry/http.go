// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License;
// you may not use this file except in compliance with the Elastic License.

package retry

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

const (
	defaultRetryWaitMin = 1 * time.Second
	defaultRetryWaitMax = 5 * time.Second
)

type HTTPOptions struct {
	RetryMax int

	retryWaitMin time.Duration
	retryWaitMax time.Duration
}

func WrapHTTPClient(client *http.Client, opts HTTPOptions) *http.Client {
	if opts.RetryMax <= 0 {
		return client
	}
	retryWaitMin := opts.retryWaitMin
	if retryWaitMin == 0 {
		retryWaitMin = defaultRetryWaitMin
	}
	retryWaitMax := opts.retryWaitMax
	if retryWaitMax == 0 {
		retryWaitMax = defaultRetryWaitMax
	}

	if client == nil {
		client = &http.Client{}
	}
	if client.CheckRedirect == nil {
		client.CheckRedirect = checkRedirect
	}
	retryClient := retryablehttp.NewClient()
	retryClient.HTTPClient = client
	retryClient.CheckRetry = checkRetry
	retryClient.ErrorHandler = retryablehttp.PassthroughErrorHandler
	retryClient.RetryMax = opts.RetryMax
	retryClient.RetryWaitMin = retryWaitMin
	retryClient.RetryWaitMax = retryWaitMax
	return retryClient.StandardClient()
}

var (
	maxRedirects   = 10
	redirectsError = fmt.Errorf("stopped after %d redirects", maxRedirects)
)

// checkRedirect reimplements default http redirect policy but returning a typed error.
func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return redirectsError
	}
	return nil
}

// checkRetry reimplements retryablehttp.DefaultRetryPolicy with better error checking.
func checkRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}

	if err != nil {
		if errors.Is(err, redirectsError) {
			// Too many redirects, let's stop here.
			return false, nil
		}

		var urlError *url.Error
		if errors.As(err, &urlError) {
			// URL is invalid, not recoverable.
			return false, nil
		}

		var certError *x509.CertificateInvalidError
		if errors.As(err, &certError) {
			// Invalid certificate, not recoverable.
			return false, nil
		}

		var caError *x509.UnknownAuthorityError
		if errors.As(err, &caError) {
			// Unknown CA, not recoverable.
			return false, nil
		}

		// Consider other errors as recoverable.
		return true, nil
	}

	// 429 Too Many Requests is recoverable. Sometimes the server puts
	// a Retry-After response header to indicate when the server is
	// available to start processing request from client.
	if resp.StatusCode == http.StatusTooManyRequests {
		return true, nil
	}

	// Check the response code. We retry on 500-range responses to allow
	// the server time to recover, as 500's are typically not permanent
	// errors and may relate to outages on the server side. This will catch
	// invalid response codes as well, like 0 and 999.
	if resp.StatusCode == 0 || (resp.StatusCode >= 500 && resp.StatusCode != http.StatusNotImplemented) {
		// Return the underlying error, that will probably be nil.
		// retryablehttp.DefaultRetryPolicy did generate an error for these cases,
		// but this is not what the default HTTP client does.
		return true, err
	}

	return false, nil
}
