// Copyright 2016-2023, Pulumi Corporation.
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
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strings"
	"time"

	"github.com/google/go-querystring/query"
	"github.com/opentracing/opentracing-go"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/util/tracing"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/httputil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/version"
)

const (
	apiRequestLogLevel       = 10 // log level for logging API requests and responses
	apiRequestDetailLogLevel = 11 // log level for logging extra details about API requests and responses
)

func UserAgent() string {
	return fmt.Sprintf("pulumi-cli/1 (%s; %s)", version.Version, runtime.GOOS)
}

// StackIdentifier is the set of data needed to identify a Pulumi Cloud stack.
type StackIdentifier struct {
	Owner   string
	Project string
	Stack   tokens.StackName
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
	Get(ctx context.Context) (string, error)
}

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

// apiAccessToken is an implementation of accessToken for Pulumi API tokens (i.e. tokens of kind
// accessTokenKindAPIToken)
type apiAccessToken string

func (apiAccessToken) Kind() accessTokenKind {
	return accessTokenKindAPIToken
}

func (t apiAccessToken) Get(_ context.Context) (string, error) {
	return string(t), nil
}

// UpdateTokenSource allows the API client to request tokens for an in-progress update as near as possible to the
// actual API call (e.g. after marshaling, etc.).
type UpdateTokenSource interface {
	GetToken(ctx context.Context) (string, error)
}

type updateTokenStaticSource string

func (t updateTokenStaticSource) GetToken(_ context.Context) (string, error) {
	return string(t), nil
}

// updateToken is an implementation of accessToken for update lease tokens (i.e. tokens of kind
// accessTokenKindUpdateToken)
type updateToken struct {
	source UpdateTokenSource
}

func updateAccessToken(source UpdateTokenSource) updateToken {
	return updateToken{source: source}
}

func (updateToken) Kind() accessTokenKind {
	return accessTokenKindUpdateToken
}

func (t updateToken) Get(ctx context.Context) (string, error) {
	return t.source.GetToken(ctx)
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

func (c *defaultHTTPClient) Do(req *http.Request, policy retryPolicy) (*http.Response, error) {
	// Wait 1s before retrying on failure. Then increase by 2x until the
	// maximum delay is reached. Stop after maxRetryCount requests have
	// been made.
	opts := httputil.RetryOpts{
		Delay:    durationPtr(time.Second),
		Backoff:  float64Ptr(2.0),
		MaxDelay: durationPtr(30 * time.Second),

		MaxRetryCount:         intPtr(4),
		HandshakeTimeoutsOnly: !policy.shouldRetry(req),
	}
	return httputil.DoWithRetryOpts(req, c.client, opts)
}

// pulumiAPICall makes an HTTP request to the Pulumi API.
func pulumiAPICall(ctx context.Context,
	requestSpan opentracing.Span,
	d diag.Sink, client httpClient, cloudAPI, method, path string, body []byte,
	tok accessToken, opts httpCallOptions,
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
		logging.V(apiRequestDetailLogLevel).Infoln("compressing payload using gzip")
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

		logging.V(apiRequestDetailLogLevel).Infof("gzip compression ratio: %f, original size: %d bytes",
			float64(len(body))/float64(len(buf.Bytes())), len(body))
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
	req.Header.Set("User-Agent", UserAgent())
	// Specify the specific API version we accept.
	req.Header.Add("Accept", "application/vnd.pulumi+8")

	// Apply credentials if provided.
	creds, err := tok.Get(ctx)
	if err != nil {
		return "", nil, fmt.Errorf("fetching credentials: %w", err)
	}
	if creds != "" {
		req.Header.Set("Authorization", fmt.Sprintf("%s %s", tok.Kind(), creds))
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
	req.Header.Add("Accept-Encoding", "gzip")
	if opts.GzipCompress {
		// If we're sending something that's gzipped, set that header too.
		req.Header.Set("Content-Encoding", "gzip")
	}

	logging.V(apiRequestLogLevel).Infof("Making Pulumi API call: %s", url)
	if logging.V(apiRequestDetailLogLevel) {
		logging.V(apiRequestDetailLogLevel).Infof(
			"Pulumi API call details (%s): headers=%v; body=%v", url, req.Header, string(body))
	}

	resp, err := client.Do(req, opts.RetryPolicy)
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
	if resp.StatusCode == 401 && tok.Kind() == accessTokenKindAPIToken && creds == "" {
		return "", nil, backenderr.ErrLoginRequired
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
		err = decodeError(respBody, resp.StatusCode, opts)
		if resp.StatusCode == 403 {
			err = backenderr.ForbiddenError{Err: err}
		}

		return "", nil, err
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
	respObj interface{}, tok accessToken, opts httpCallOptions,
) error {
	requestSpan, ctx := opentracing.StartSpanFromContext(ctx, getEndpointName(method, path),
		opentracing.Tag{Key: "method", Value: method},
		opentracing.Tag{Key: "path", Value: path},
		opentracing.Tag{Key: "api", Value: cloudAPI},
		opentracing.Tag{Key: "retry", Value: opts.RetryPolicy.String()})
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
	url, resp, err := pulumiAPICall(
		ctx, requestSpan, diag, c.client, cloudAPI, method, path+querystring, reqBody, tok, opts)
	if err != nil {
		return err
	}

	switch respObj := respObj.(type) {
	case **http.Response:
		*respObj = resp
		return nil
	case *io.ReadCloser:
		*respObj, err = bodyIntoReader(resp)
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
		switch respObj := respObj.(type) {
		case *[]byte:
			// Return the raw bytes of the response body.
			*respObj = respBody
		case []byte:
			return errors.New("Can't unmarshal response body to []byte. Try *[]byte")
		default:
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
	reader, err := bodyIntoReader(resp)
	defer contract.IgnoreClose(resp.Body)
	if err != nil {
		return nil, err
	}
	return io.ReadAll(reader)
}

func bodyIntoReader(resp *http.Response) (io.ReadCloser, error) {
	contentEncoding, ok := resp.Header["Content-Encoding"]
	if !ok {
		// No header implies that there's no additional encoding on this response.
		return resp.Body, nil
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

		return reader, nil
	default:
		return nil, fmt.Errorf("unrecognized encoding %s", contentEncoding[0])
	}
}
