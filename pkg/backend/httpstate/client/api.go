// Copyright 2016-2018, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package client

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"runtime"
	"strings"
	"time"

	"github.com/google/go-querystring/query"
	"github.com/opentracing/opentracing-go"

	"github.com/pulumi/pulumi/pkg/v3/util/tracing"
	"github.com/pulumi/pulumi/pkg/v3/version"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/httputil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
)

const (
	apiRequestLogLevel       = 10 // log level for logging API requests and responses
	apiRequestDetailLogLevel = 11 // log level for logging extra details about API requests and responses
)

// StackIdentifier is the set of data needed to identify a Pulumi Cloud stack.
type StackIdentifier struct {
	Owner   string
	Project string
	Stack   string
}

func (s StackIdentifier) String() string {
	return fmt.Sprintf("%s/%s/%s", s.Owner, s.Project, s.Stack)
}

// UpdateIdentifier is the set of data needed to identify an update to a Pulumi Cloud stack.
type UpdateIdentifier struct {
	StackIdentifier

	UpdateKind apitype.UpdateKind
	UpdateID   string
}

// accessTokenKind is enumerates the various types of access token used with the Pulumi API. These kinds correspond
// directly to the "method" piece of an HTTP `Authorization` header.
type accessTokenKind string

const (
	// accessTokenKindAPIToken denotes a standard Pulumi API token.
	accessTokenKindAPIToken accessTokenKind = "token"
	// accessTokenKindUpdateToken denotes an update lease token.
	accessTokenKindUpdateToken accessTokenKind = "update-token"
)

// accessToken is an abstraction over the two different kinds of access tokens used by the Pulumi API.
type accessToken interface {
	Kind() accessTokenKind
	String() string
}

type httpCallOptions struct {
	// RetryAllMethods allows non-GET calls to be retried if the server fails to return a response.
	RetryAllMethods bool

	// GzipCompress compresses the request using gzip before sending it.
	GzipCompress bool
}

// apiAccessToken is an implementation of accessToken for Pulumi API tokens (i.e. tokens of kind
// accessTokenKindAPIToken)
type apiAccessToken string

func (apiAccessToken) Kind() accessTokenKind {
	return accessTokenKindAPIToken
}

func (t apiAccessToken) String() string {
	return string(t)
}

// updateAccessToken is an implementation of accessToken for update lease tokens (i.e. tokens of kind
// accessTokenKindUpdateToken)
type updateAccessToken string

func (updateAccessToken) Kind() accessTokenKind {
	return accessTokenKindUpdateToken
}

func (t updateAccessToken) String() string {
	return string(t)
}

func float64Ptr(f float64) *float64 {
	return &f
}

func durationPtr(d time.Duration) *time.Duration {
	return &d
}

func intPtr(i int) *int {
	return &i
}

// httpClient is an HTTP client abstraction, used by defaultRESTClient.
type httpClient interface {
	Do(req *http.Request, retryAllMethods bool) (*http.Response, error)
}

// defaultHTTPClient is an implementation of httpClient that provides a basic implementation of Do
// using the specified *http.Client, with retry support.
type defaultHTTPClient struct {
	client *http.Client
}

func (c *defaultHTTPClient) Do(req *http.Request, retryAllMethods bool) (*http.Response, error) {
	if req.Method == "GET" || retryAllMethods {
		// Wait 1s before retrying on failure. Then increase by 2x until the
		// maximum delay is reached. Stop after maxRetryCount requests have
		// been made.
		opts := httputil.RetryOpts{
			Delay:    durationPtr(time.Second),
			Backoff:  float64Ptr(2.0),
			MaxDelay: durationPtr(30 * time.Second),

			MaxRetryCount: intPtr(4),
		}
		return httputil.DoWithRetryOpts(req, c.client, opts)
	}
	return c.client.Do(req)
}

// pulumiAPICall makes an HTTP request to the Pulumi API.
func pulumiAPICall(ctx context.Context,
	requestSpan opentracing.Span,
	d diag.Sink, client httpClient, cloudAPI, method, path string,
	body io.WriterTo,
	tok accessToken, opts httpCallOptions) (string, *http.Response, error) {

	// Normalize URL components
	cloudAPI = strings.TrimSuffix(cloudAPI, "/")
	path = cleanPath(path)

	url := fmt.Sprintf("%s%s", cloudAPI, path)

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return "", nil, fmt.Errorf("creating new HTTP request: %w", err)
	}
	if body != nil {
		if opts.GzipCompress {
			body = &gzipEncodingWriterTo{body}
		}
		if err := setupBody(req, body); err != nil {
			return "", nil, fmt.Errorf("setting up body for the new HTTP request: %w", err)
		}
	}

	req.Header.Set("Content-Type", "application/json")

	// Add a User-Agent header to allow for the backend to make breaking API changes while preserving
	// backwards compatibility.
	userAgent := fmt.Sprintf("pulumi-cli/1 (%s; %s)", version.Version, runtime.GOOS)
	req.Header.Set("User-Agent", userAgent)
	// Specify the specific API version we accept.
	req.Header.Set("Accept", "application/vnd.pulumi+8")

	// Apply credentials if provided.
	if tok.String() != "" {
		req.Header.Set("Authorization", fmt.Sprintf("%s %s", tok.Kind(), tok.String()))
	}

	tracingOptions := tracing.OptionsFromContext(ctx)
	if tracingOptions.PropagateSpans {
		carrier := opentracing.HTTPHeadersCarrier(req.Header)
		if err = requestSpan.Tracer().Inject(requestSpan.Context(), opentracing.HTTPHeaders, carrier); err != nil {
			logging.Errorf("injecting tracing headers: %v", err)
		}
	}
	if tracingOptions.TracingHeader != "" {
		req.Header.Set("X-Pulumi-Tracing", tracingOptions.TracingHeader)
	}

	// Opt-in to accepting gzip-encoded responses from the service.
	req.Header.Set("Accept-Encoding", "gzip")
	if opts.GzipCompress {
		// If we're sending something that's gzipped, set that header too.
		req.Header.Set("Content-Encoding", "gzip")
	}

	logging.V(apiRequestLogLevel).Infof("Making Pulumi API call: %s", url)
	if logging.V(apiRequestDetailLogLevel) {
		var buf bytes.Buffer
		_, err := body.WriteTo(&buf)
		contract.IgnoreError(err)
		logging.V(apiRequestDetailLogLevel).Infof(
			"Pulumi API call details (%s): headers=%v; body=%v", url, req.Header, buf.String())
	}

	resp, err := client.Do(req, opts.RetryAllMethods)
	if err != nil {
		// Don't wrap *apitype.ErrorResponse.
		if _, ok := err.(*apitype.ErrorResponse); ok {
			return "", nil, err
		}
		return "", nil, fmt.Errorf("performing HTTP request: %w", err)
	}
	logging.V(apiRequestLogLevel).Infof("Pulumi API call response code (%s): %v", url, resp.Status)

	requestSpan.SetTag("responseCode", resp.Status)

	if warningHeader, ok := resp.Header["X-Pulumi-Warning"]; ok {
		for _, warning := range warningHeader {
			d.Warningf(diag.RawMessage("", warning))
		}
	}

	// Provide a better error if using an authenticated call without having logged in first.
	if resp.StatusCode == 401 && tok.Kind() == accessTokenKindAPIToken && tok.String() == "" {
		return "", nil, errors.New("this command requires logging in; try running `pulumi login` first")
	}

	// Provide a better error if rate-limit is exceeded(429: Too Many Requests)
	if resp.StatusCode == 429 {
		return "", nil, errors.New("pulumi service: request rate-limit exceeded")
	}

	// For 4xx and 5xx failures, attempt to provide better diagnostics about what may have gone wrong.
	if resp.StatusCode >= 400 && resp.StatusCode <= 599 {
		// 4xx and 5xx responses should be of type ErrorResponse. See if we can unmarshal as that
		// type, and if not just return the raw response text.
		respBody, err := readBody(resp)
		if err != nil {
			return "", nil, fmt.Errorf("API call failed (%s), could not read response: %w", resp.Status, err)
		}

		var errResp apitype.ErrorResponse
		if err = json.Unmarshal(respBody, &errResp); err != nil {
			errResp.Code = resp.StatusCode
			errResp.Message = strings.TrimSpace(string(respBody))
		}
		return "", nil, &errResp
	}

	return url, resp, nil
}

func setupBody(req *http.Request, body io.WriterTo) error {
	// For large requests setupBody will serialize simultaneously with sending the request using a helper goroutine
	// behind a pipe. It turns out that even for large requests it is important to set ContentLength, otherwise the
	// requests slow down a lot (possibly due to the use of Chunked Transfer Encoding).
	//
	// How is it possible to know ContentLength without serializing the large request into a temporary buffer in
	// memory? The solution is to serialize twice.
	//
	// First serialize into a limitWriter that retains only up to 1mb of the request, discarding the rest, but
	// computing the true ContentLength.
	oneMB := 1024 * 1024
	w := &limitWriter{maxBytes: oneMB}
	_, err := body.WriteTo(w)
	if err != nil {
		return err
	}
	if w.Overflow() {
		// Overflow indicates that the true content did not fit into 1mb. Serialize the body again and pipe it.
		req.GetBody = func() (io.ReadCloser, error) {
			return pipedBody(body), nil
		}
	} else {
		// If the content fit in the 1mb buffer, no more serialization is needed, keep it in memory.
		data := w.buf.Bytes()
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(data)), nil
		}
	}
	req.Body, err = req.GetBody()
	if err != nil {
		return err
	}
	req.ContentLength = w.written
	return nil
}

func pipedBody(body io.WriterTo) io.ReadCloser {
	pipeBufferSize := 1024 * 1024
	bodyReader, bodyWriter := io.Pipe()

	go func() {
		bufWriter := bufio.NewWriterSize(bodyWriter, pipeBufferSize)
		_, err := body.WriteTo(bufWriter)

		flushErr := bufWriter.Flush()
		if err != nil {
			err = flushErr
		}

		if err != nil && err != io.ErrClosedPipe {
			bodyWriter.CloseWithError(err)
		} else {
			bodyWriter.Close()
		}
	}()

	type readcloser struct {
		io.Reader
		io.Closer
	}
	return readcloser{
		Reader: bufio.NewReaderSize(bodyReader, pipeBufferSize),
		Closer: bodyReader,
	}
}

// restClient is an abstraction for calling the Pulumi REST API.
type restClient interface {
	Call(ctx context.Context, diag diag.Sink, cloudAPI, method, path string, queryObj, reqObj,
		respObj interface{}, tok accessToken, opts httpCallOptions) error
}

// defaultRESTClient is the default implementation for calling the Pulumi REST API.
type defaultRESTClient struct {
	client httpClient
}

// Call calls the Pulumi REST API marshalling reqObj to JSON and using that as
// the request body (use nil for GETs), and if successful, marshalling the responseObj
// as JSON and storing it in respObj (use nil for NoContent). The error return type might
// be an instance of apitype.ErrorResponse, in which case will have the response code.
func (c *defaultRESTClient) Call(ctx context.Context, diag diag.Sink, cloudAPI, method, path string, queryObj, reqObj,
	respObj interface{}, tok accessToken, opts httpCallOptions) error {

	requestSpan, ctx := opentracing.StartSpanFromContext(ctx, getEndpointName(method, path),
		opentracing.Tag{Key: "method", Value: method},
		opentracing.Tag{Key: "path", Value: path},
		opentracing.Tag{Key: "api", Value: cloudAPI},
		opentracing.Tag{Key: "retry", Value: opts.RetryAllMethods})
	defer requestSpan.Finish()

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
	var reqBody io.WriterTo
	var err error
	if reqObj != nil {
		// Send verbatim if already marshalled. This is
		// important when sending indented JSON is needed.
		if raw, ok := reqObj.(json.RawMessage); ok {
			reqBody = &bytesWriterTo{[]byte(raw)}
		} else if raw, ok := reqObj.(io.WriterTo); ok {
			reqBody = raw
		} else {
			reqBody = &jsonMarshalWriterTo{reqObj}
		}
	}

	// Make API call
	url, resp, err := pulumiAPICall(
		ctx, requestSpan, diag, c.client, cloudAPI, method, path+querystring, reqBody, tok, opts)
	if err != nil {
		return err
	}

	// Read API response
	respBody, err := readBody(resp)
	if err != nil {
		return fmt.Errorf("reading response from API: %w", err)
	}
	if logging.V(apiRequestDetailLogLevel) {
		logging.V(apiRequestDetailLogLevel).Infof("Pulumi API call response body (%s): %v", url, string(respBody))
	}

	if respObj != nil {
		bytes := reflect.TypeOf([]byte(nil))
		if typ := reflect.TypeOf(respObj); typ == reflect.PtrTo(bytes) {
			// Return the raw bytes of the response body.
			*respObj.(*[]byte) = respBody
		} else if typ == bytes {
			return fmt.Errorf("Can't unmarshal response body to []byte. Try *[]byte")
		} else {
			// Else, unmarshal as JSON.
			if err = json.Unmarshal(respBody, respObj); err != nil {
				return fmt.Errorf("unmarshalling response object: %w", err)
			}
		}
	}

	return nil
}

// readBody reads the contents of an http.Response into a byte array, returning an error if one occurred while in the
// process of doing so. readBody uses the Content-Encoding of the response to pick the correct reader to use.
func readBody(resp *http.Response) ([]byte, error) {
	contentEncoding, ok := resp.Header["Content-Encoding"]
	defer contract.IgnoreClose(resp.Body)
	if !ok {
		// No header implies that there's no additional encoding on this response.
		return ioutil.ReadAll(resp.Body)
	}

	if len(contentEncoding) > 1 {
		// We only know how to deal with gzip. We can't handle additional encodings layered on top of it.
		return nil, fmt.Errorf("can't handle content encodings %v", contentEncoding)
	}

	switch contentEncoding[0] {
	case "x-gzip":
		// The HTTP/1.1 spec recommends we treat x-gzip as an alias of gzip.
		fallthrough
	case "gzip":
		logging.V(apiRequestDetailLogLevel).Infoln("decompressing gzipped response from service")
		reader, err := gzip.NewReader(resp.Body)
		if reader != nil {
			defer contract.IgnoreClose(reader)
		}
		if err != nil {
			return nil, fmt.Errorf("reading gzip-compressed body: %w", err)
		}

		return ioutil.ReadAll(reader)
	default:
		return nil, fmt.Errorf("unrecognized encoding %s", contentEncoding[0])
	}
}
