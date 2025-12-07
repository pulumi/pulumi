// Copyright 2025, Pulumi Corporation.
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

package gitlfs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// LFSMediaType is the media type for Git LFS API requests/responses
	LFSMediaType = "application/vnd.git-lfs+json"

	// defaultTimeout is the default HTTP client timeout
	defaultTimeout = 30 * time.Second

	// maxRetries is the maximum number of retries for transient errors
	maxRetries = 3

	// retryDelay is the base delay between retries
	retryDelay = 500 * time.Millisecond
)

var (
	// ErrNotFound indicates the object was not found
	ErrNotFound = errors.New("object not found")

	// ErrUnauthorized indicates authentication failed
	ErrUnauthorized = errors.New("authentication failed")

	// ErrForbidden indicates the operation is not permitted
	ErrForbidden = errors.New("operation not permitted")

	// ErrRateLimited indicates the rate limit was exceeded
	ErrRateLimited = errors.New("rate limit exceeded")

	// ErrServerError indicates a server-side error
	ErrServerError = errors.New("server error")

	// ErrInsufficientStorage indicates the server has insufficient storage
	ErrInsufficientStorage = errors.New("insufficient storage")
)

// Client handles Git LFS Batch API operations
type Client struct {
	// baseURL is the LFS server URL (e.g., "https://github.com/org/repo.git/info/lfs")
	baseURL string

	// httpClient is the HTTP client to use
	httpClient *http.Client

	// auth is the authenticator to use
	auth Authenticator
}

// Ref represents a Git reference for LFS operations
type Ref struct {
	Name string `json:"name"`
}

// ObjectSpec specifies an object for batch operations
type ObjectSpec struct {
	OID  string `json:"oid"`
	Size int64  `json:"size"`
}

// BatchRequest is the request body for the Batch API
type BatchRequest struct {
	Operation string       `json:"operation"`
	Transfers []string     `json:"transfers,omitempty"`
	Ref       *Ref         `json:"ref,omitempty"`
	Objects   []ObjectSpec `json:"objects"`
	HashAlgo  string       `json:"hash_algo,omitempty"`
}

// BatchResponse is the response from the Batch API
type BatchResponse struct {
	Transfer string         `json:"transfer"`
	Objects  []ObjectResult `json:"objects"`
	HashAlgo string         `json:"hash_algo,omitempty"`
}

// ObjectResult contains actions for an object
type ObjectResult struct {
	OID           string            `json:"oid"`
	Size          int64             `json:"size"`
	Authenticated bool              `json:"authenticated,omitempty"`
	Actions       map[string]Action `json:"actions,omitempty"`
	Error         *ObjectError      `json:"error,omitempty"`
}

// Action contains href and headers for an action
type Action struct {
	Href      string            `json:"href"`
	Header    map[string]string `json:"header,omitempty"`
	ExpiresIn int               `json:"expires_in,omitempty"`
	ExpiresAt string            `json:"expires_at,omitempty"`
}

// ObjectError represents an error for a specific object
type ObjectError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// APIError represents an error response from the LFS API
type APIError struct {
	Message          string `json:"message"`
	DocumentationURL string `json:"documentation_url,omitempty"`
	RequestID        string `json:"request_id,omitempty"`
}

func (e *APIError) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("%s (request_id: %s)", e.Message, e.RequestID)
	}
	return e.Message
}

// NewClient creates a new LFS client
func NewClient(baseURL string, auth Authenticator) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		auth: auth,
	}
}

// NewClientWithHTTPClient creates a new LFS client with a custom HTTP client
func NewClientWithHTTPClient(baseURL string, auth Authenticator, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    baseURL,
		httpClient: httpClient,
		auth:       auth,
	}
}

// Batch performs a batch operation (download or upload)
func (c *Client) Batch(ctx context.Context, req *BatchRequest) (*BatchResponse, error) {
	// Default to basic transfer if not specified
	if len(req.Transfers) == 0 {
		req.Transfers = []string{"basic"}
	}

	// Default hash algorithm
	if req.HashAlgo == "" {
		req.HashAlgo = LFSHashAlgo
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshaling batch request: %w", err)
	}

	url := c.baseURL + "/objects/batch"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating batch request: %w", err)
	}

	httpReq.Header.Set("Accept", LFSMediaType)
	httpReq.Header.Set("Content-Type", LFSMediaType)

	if c.auth != nil {
		if err := c.auth.Authenticate(httpReq); err != nil {
			return nil, fmt.Errorf("authenticating request: %w", err)
		}
	}

	resp, err := c.doWithRetry(ctx, httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	var batchResp BatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
		return nil, fmt.Errorf("decoding batch response: %w", err)
	}

	return &batchResp, nil
}

// Download downloads an LFS object
func (c *Client) Download(ctx context.Context, oid string, size int64) ([]byte, error) {
	// First, get the download URL via batch API
	batchResp, err := c.Batch(ctx, &BatchRequest{
		Operation: "download",
		Objects:   []ObjectSpec{{OID: oid, Size: size}},
	})
	if err != nil {
		return nil, fmt.Errorf("batch download request: %w", err)
	}

	if len(batchResp.Objects) == 0 {
		return nil, ErrNotFound
	}

	obj := batchResp.Objects[0]
	if obj.Error != nil {
		return nil, c.objectErrorToError(obj.Error)
	}

	downloadAction, ok := obj.Actions["download"]
	if !ok {
		return nil, errors.New("no download action in response")
	}

	// Download the object
	return c.downloadFromAction(ctx, &downloadAction)
}

// Upload uploads an LFS object
func (c *Client) Upload(ctx context.Context, oid string, data []byte) error {
	size := int64(len(data))

	// First, get the upload URL via batch API
	batchResp, err := c.Batch(ctx, &BatchRequest{
		Operation: "upload",
		Objects:   []ObjectSpec{{OID: oid, Size: size}},
	})
	if err != nil {
		return fmt.Errorf("batch upload request: %w", err)
	}

	if len(batchResp.Objects) == 0 {
		return errors.New("no objects in batch response")
	}

	obj := batchResp.Objects[0]
	if obj.Error != nil {
		return c.objectErrorToError(obj.Error)
	}

	// If there's no upload action, the server already has the object
	uploadAction, hasUpload := obj.Actions["upload"]
	if !hasUpload {
		return nil // Object already exists on server
	}

	// Upload the object
	if err := c.uploadToAction(ctx, &uploadAction, data); err != nil {
		return err
	}

	// Verify if requested
	verifyAction, hasVerify := obj.Actions["verify"]
	if hasVerify {
		if err := c.Verify(ctx, &verifyAction, oid, size); err != nil {
			return fmt.Errorf("verify failed: %w", err)
		}
	}

	return nil
}

// Verify verifies an uploaded object
func (c *Client) Verify(ctx context.Context, action *Action, oid string, size int64) error {
	body, err := json.Marshal(map[string]any{
		"oid":  oid,
		"size": size,
	})
	if err != nil {
		return fmt.Errorf("marshaling verify request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, action.Href, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("creating verify request: %w", err)
	}

	req.Header.Set("Accept", LFSMediaType)
	req.Header.Set("Content-Type", LFSMediaType)

	// Add action-specific headers
	for k, v := range action.Header {
		req.Header.Set(k, v)
	}

	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.handleErrorResponse(resp)
	}

	return nil
}

// downloadFromAction downloads content from a download action
func (c *Client) downloadFromAction(ctx context.Context, action *Action) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, action.Href, nil)
	if err != nil {
		return nil, fmt.Errorf("creating download request: %w", err)
	}

	// Add action-specific headers (includes auth if provided by server)
	for k, v := range action.Header {
		req.Header.Set(k, v)
	}

	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleErrorResponse(resp)
	}

	return io.ReadAll(resp.Body)
}

// uploadToAction uploads content to an upload action
func (c *Client) uploadToAction(ctx context.Context, action *Action, data []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, action.Href, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("creating upload request: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = int64(len(data))

	// Add action-specific headers (includes auth if provided by server)
	for k, v := range action.Header {
		req.Header.Set(k, v)
	}

	resp, err := c.doWithRetry(ctx, req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Accept 200, 201, or 204 as success
	if resp.StatusCode != http.StatusOK &&
		resp.StatusCode != http.StatusCreated &&
		resp.StatusCode != http.StatusNoContent {
		return c.handleErrorResponse(resp)
	}

	return nil
}

// doWithRetry performs an HTTP request with retry logic
func (c *Client) doWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	var lastErr error
	delay := retryDelay

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
				delay *= 2 // Exponential backoff
			}

			// Clone the request for retry (body needs to be re-readable)
			req = req.Clone(ctx)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Don't retry on non-retryable status codes
		if !isRetryableStatus(resp.StatusCode) {
			return resp, nil
		}

		// Close body for retry
		resp.Body.Close()
		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// isRetryableStatus returns true if the status code indicates a retryable error
func isRetryableStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests, // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:      // 504
		return true
	default:
		return false
	}
}

// handleErrorResponse converts an HTTP error response to a Go error
func (c *Client) handleErrorResponse(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)

	// Try to parse as LFS API error
	var apiErr APIError
	if json.Unmarshal(body, &apiErr) == nil && apiErr.Message != "" {
		switch resp.StatusCode {
		case http.StatusUnauthorized:
			return fmt.Errorf("%w: %s", ErrUnauthorized, apiErr.Message)
		case http.StatusForbidden:
			return fmt.Errorf("%w: %s", ErrForbidden, apiErr.Message)
		case http.StatusNotFound:
			return fmt.Errorf("%w: %s", ErrNotFound, apiErr.Message)
		case http.StatusTooManyRequests:
			return fmt.Errorf("%w: %s", ErrRateLimited, apiErr.Message)
		case http.StatusInsufficientStorage:
			return fmt.Errorf("%w: %s", ErrInsufficientStorage, apiErr.Message)
		default:
			return &apiErr
		}
	}

	// Generic error based on status code
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return ErrUnauthorized
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusTooManyRequests:
		return ErrRateLimited
	case http.StatusInsufficientStorage:
		return ErrInsufficientStorage
	default:
		if resp.StatusCode >= 500 {
			return fmt.Errorf("%w: HTTP %d", ErrServerError, resp.StatusCode)
		}
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
}

// objectErrorToError converts an ObjectError to a Go error
func (c *Client) objectErrorToError(objErr *ObjectError) error {
	switch objErr.Code {
	case 404:
		return fmt.Errorf("%w: %s", ErrNotFound, objErr.Message)
	case 403:
		return fmt.Errorf("%w: %s", ErrForbidden, objErr.Message)
	case 410:
		return fmt.Errorf("object removed: %s", objErr.Message)
	case 422:
		return fmt.Errorf("validation error: %s", objErr.Message)
	default:
		return fmt.Errorf("object error %d: %s", objErr.Code, objErr.Message)
	}
}

// BuildLFSURL builds the LFS API URL for a Git repository
func BuildLFSURL(host, owner, repo string) string {
	// Standard LFS URL pattern: https://<host>/<owner>/<repo>.git/info/lfs
	return fmt.Sprintf("https://%s/%s/%s.git/info/lfs", host, owner, repo)
}
