// Copyright 2023, Pulumi Corporation.

package client

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/google/go-querystring/query"

	"github.com/pulumi/pulumi/pkg/v3/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/httputil"
)

type httpCallOptions struct {
	// RetryPolicy defines the policy for retrying requests by httpClient.Do.
	//
	// By default, only GET requests are retried.
	RetryPolicy retryPolicy

	// GzipCompress compresses the request using gzip before sending it.
	GzipCompress bool

	// Header is any additional headers to add to the request.
	Header http.Header

	// ErrorResponse is an optional response body for errors.
	ErrorResponse any
}

// retryPolicy defines the policy for retrying requests by httpClient.Do.
type retryPolicy int

const (
	// retryNone indicates that no retry should be attempted.
	retryNone retryPolicy = iota - 1

	// retryGetMethod indicates that only GET requests should be retried.
	//
	// This is the default retry policy.
	retryGetMethod // == 0

	// retryAllMethods indicates that all requests should be retried.
	retryAllMethods
)

func (p retryPolicy) String() string {
	switch p {
	case retryNone:
		return "none"
	case retryGetMethod:
		return "get"
	case retryAllMethods:
		return "all"
	default:
		return fmt.Sprintf("retryPolicy(%d)", p)
	}
}

func (p retryPolicy) shouldRetry(req *http.Request) bool {
	switch p {
	case retryNone:
		return false
	case retryGetMethod:
		return req.Method == http.MethodGet
	case retryAllMethods:
		return true
	default:
		contract.Failf("unknown retry policy: %v", p)
		return false // unreachable
	}
}

// httpClient is an HTTP client abstraction, used by defaultRESTClient.
type httpClient interface {
	Do(req *http.Request, policy retryPolicy) (*http.Response, error)
}

// defaultHTTPClient is an implementation of httpClient that provides a basic implementation of Do
// using the specified *http.Client, with retry support.
type defaultHTTPClient struct {
	client *http.Client
}

func some[T any](v T) *T {
	return &v
}

func (c *defaultHTTPClient) Do(req *http.Request, policy retryPolicy) (*http.Response, error) {
	if policy.shouldRetry(req) {
		// Wait 1s before retrying on failure. Then increase by 2x until the
		// maximum delay is reached. Stop after maxRetryCount requests have
		// been made.
		opts := httputil.RetryOpts{
			Delay:    some(time.Second),
			Backoff:  some(float64(2.0)),
			MaxDelay: some(30 * time.Second),

			MaxRetryCount: some(int(4)),
		}
		return httputil.DoWithRetryOpts(req, c.client, opts)
	}
	return c.client.Do(req)
}

// pulumiAPICall makes an HTTP request to the Pulumi API.
func pulumiAPICall(ctx context.Context,
	client httpClient, cloudAPI, method, path string, body []byte,
	tok string, opts httpCallOptions,
) (string, *http.Response, error) {
	// Normalize URL components
	cloudAPI = strings.TrimSuffix(cloudAPI, "/")
	path = cleanPath(path)

	url := fmt.Sprintf("%s%s", cloudAPI, path)
	var bodyReader io.Reader
	if opts.GzipCompress {
		// If we're being asked to compress the payload, go ahead and do it here to an intermediate buffer.
		//
		// If this becomes a performance bottleneck, we may want to consider marshaling json directly to this
		// gzip.Writer instead of marshaling to a byte array and compressing it to another buffer.
		var buf bytes.Buffer
		writer := gzip.NewWriter(&buf)
		defer contract.IgnoreClose(writer)
		if _, err := writer.Write(body); err != nil {
			return "", nil, fmt.Errorf("compressing payload: %w", err)
		}

		// gzip.Writer will not actually write anything unless it is flushed,
		//  and it will not actually write the GZip footer unless it is closed. (Close also flushes)
		// Without this, the compressed bytes do not decompress properly e.g. in python.
		if err := writer.Close(); err != nil {
			return "", nil, fmt.Errorf("closing compressed payload: %w", err)
		}

		bodyReader = &buf
	} else {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return "", nil, fmt.Errorf("creating new HTTP request: %w", err)
	}

	req = req.WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")

	// Set headers from the incoming options.
	for k, v := range opts.Header {
		req.Header[k] = v
	}

	// Add a User-Agent header to allow for the backend to make breaking API changes while preserving
	// backwards compatibility.
	userAgent := fmt.Sprintf("esc-cli/1 (%s; %s)", version.Version, runtime.GOOS)
	req.Header.Set("User-Agent", userAgent)
	// Specify the specific API version we accept.
	req.Header.Set("Accept", "application/vnd.pulumi+8")

	// Apply credentials if provided.
	if tok != "" {
		req.Header.Set("Authorization", fmt.Sprintf("token %s", tok))
	}

	if opts.GzipCompress {
		// If we're sending something that's gzipped, set that header too.
		req.Header.Set("Content-Encoding", "gzip")
	}

	resp, err := client.Do(req, opts.RetryPolicy)
	if err != nil {
		// Don't wrap *apitype.ErrorResponse.
		if _, ok := err.(*apitype.ErrorResponse); ok {
			return "", nil, err
		}
		return "", nil, fmt.Errorf("performing HTTP request: %w", err)
	}

	// Provide a better error if using an authenticated call without having logged in first.
	if resp.StatusCode == 401 && tok == "" {
		return "", nil, errors.New("this command requires logging in; try running `esc login` first")
	}

	// Provide a better error if rate-limit is exceeded(429: Too Many Requests)
	if resp.StatusCode == 429 {
		return "", nil, errors.New("esc: request rate-limit exceeded")
	}

	// For 4xx and 5xx failures, attempt to provide better diagnostics about what may have gone wrong.
	if resp.StatusCode >= 400 && resp.StatusCode <= 599 {
		// 4xx and 5xx responses should be of type ErrorResponse. See if we can unmarshal as that
		// type, and if not just return the raw response text.
		respBody, err := readBody(resp)
		if err != nil {
			return "", nil, fmt.Errorf("API call failed (%s), could not read response: %w", resp.Status, err)
		}
		return "", nil, decodeError(respBody, resp.StatusCode, opts)
	}

	return url, resp, nil
}

func decodeError(respBody []byte, statusCode int, opts httpCallOptions) error {
	if opts.ErrorResponse != nil {
		if err := json.Unmarshal(respBody, opts.ErrorResponse); err == nil {
			return opts.ErrorResponse.(error)
		}
	}

	var errResp apitype.ErrorResponse
	if err := json.Unmarshal(respBody, &errResp); err != nil {
		errResp.Code = statusCode
		errResp.Message = strings.TrimSpace(string(respBody))
	}
	return &errResp
}

// restClient is an abstraction for calling the Pulumi REST API.
type restClient interface {
	Call(ctx context.Context, cloudAPI, method, path string, queryObj, reqObj,
		respObj interface{}, tok string, opts httpCallOptions) error
}

// defaultRESTClient is the default implementation for calling the Pulumi REST API.
type defaultRESTClient struct {
	client httpClient
}

// Call calls the Pulumi REST API marshalling reqObj to JSON and using that as
// the request body (use nil for GETs), and if successful, marshalling the responseObj
// as JSON and storing it in respObj (use nil for NoContent). The error return type might
// be an instance of apitype.ErrorResponse, in which case will have the response code.
func (c *defaultRESTClient) Call(ctx context.Context, cloudAPI, method, path string, queryObj, reqObj,
	respObj interface{}, tok string, opts httpCallOptions,
) error {
	// Compute query string from query object
	querystring := ""
	if queryObj != nil {
		queryValues, err := query.Values(queryObj)
		if err != nil {
			return fmt.Errorf("marshalling query object as JSON: %w", err)
		}
		query := queryValues.Encode()
		if len(query) > 0 {
			querystring = "?" + query
		}
	}

	// Compute request body from request object
	var reqBody []byte
	var err error
	if reqObj != nil {
		// Send verbatim if already marshalled. This is
		// important when sending indented JSON is needed.
		if raw, ok := reqObj.(json.RawMessage); ok {
			reqBody = []byte(raw)
		} else {
			reqBody, err = json.Marshal(reqObj)
			if err != nil {
				return fmt.Errorf("marshalling request object as JSON: %w", err)
			}
		}
	}

	// Make API call
	_, resp, err := pulumiAPICall(
		ctx, c.client, cloudAPI, method, path+querystring, reqBody, tok, opts)
	if err != nil {
		return err
	}
	if respPtr, ok := respObj.(**http.Response); ok {
		*respPtr = resp
		return nil
	}

	// Read API response
	respBody, err := readBody(resp)
	if err != nil {
		return fmt.Errorf("reading response from API: %w", err)
	}

	if respObj != nil {
		switch respObj := respObj.(type) {
		case *[]byte:
			// Return the raw bytes of the response body.
			*respObj = respBody
		case []byte:
			return fmt.Errorf("Can't unmarshal response body to []byte. Try *[]byte")
		default:
			// Else, unmarshal as JSON.
			if err = json.Unmarshal(respBody, respObj); err != nil {
				return fmt.Errorf("unmarshalling response object: %w", err)
			}
		}
	}

	return nil
}

func readBody(resp *http.Response) ([]byte, error) {
	return io.ReadAll(resp.Body)
}

// cleanPath returns the canonical path for p, eliminating . and .. elements.
// Borrowed from gorilla/mux.
func cleanPath(p string) string {
	if p == "" {
		return "/"
	}

	if p[0] != '/' {
		p = "/" + p
	}
	np := path.Clean(p)

	// path.Clean removes trailing slash except for root;
	// put the trailing slash back if necessary.
	if p[len(p)-1] == '/' && np != "/" {
		np += "/"
	}

	return np
}
