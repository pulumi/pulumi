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

package workspace

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const RetryCount = 6

// Sanitize archive file pathing from "G305: Zip Slip vulnerability"
func sanitizeArchivePath(d, t string) (v string, err error) {
	v = filepath.Join(d, t)
	if strings.HasPrefix(v, filepath.Clean(d)) {
		return v, nil
	}

	return "", fmt.Errorf("%s: %s", "content filepath is tainted", t)
}

func isZIPTemplateURL(templateNamePathOrURL string) bool {
	parsedURL, err := url.Parse(templateNamePathOrURL)
	if err != nil {
		return false
	}
	return parsedURL.Path != "" && strings.HasSuffix(parsedURL.Path, ".zip")
}

func retrieveZIPTemplates(templateURL string) (TemplateRepository, error) {
	var err error
	// Create a temp dir.
	var temp string
	if temp, err = os.MkdirTemp("", "pulumi-template-"); err != nil {
		return TemplateRepository{}, err
	}

	parsedURL, err := url.Parse(templateURL)
	if err != nil {
		return TemplateRepository{}, err
	}

	var fullPath string
	if fullPath, err = RetrieveZIPTemplateFolder(parsedURL, temp); err != nil {
		return TemplateRepository{}, fmt.Errorf("failed to retrieve zip archive: %w", err)
	}

	return TemplateRepository{
		Root:         temp,
		SubDirectory: fullPath,
		ShouldDelete: true,
	}, nil
}

func backoff(retries int) time.Duration {
	return time.Duration(math.Pow(2, float64(retries))) * (time.Second / 4)
}

func shouldRetry(err error, resp *http.Response) bool {
	if err != nil {
		return true
	}

	if resp.StatusCode == http.StatusBadGateway ||
		resp.StatusCode == http.StatusServiceUnavailable ||
		resp.StatusCode == http.StatusGatewayTimeout ||
		resp.StatusCode == http.StatusNotFound {
		return true
	}
	return false
}

func drainBody(resp *http.Response) {
	if resp.Body != nil {
		_, err := io.Copy(io.Discard, resp.Body)
		if err != nil {
			return
		}
		resp.Body.Close()
	}
}

type retryableTransport struct {
	transport http.RoundTripper
}

func (t *retryableTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request body
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}
	// Send the request
	resp, err := t.transport.RoundTrip(req)
	// Retry logic
	retries := 0
	for shouldRetry(err, resp) && retries < RetryCount {
		// Wait for the specified backoff period
		time.Sleep(backoff(retries))
		// We're going to retry, consume any response to reuse the connection.
		drainBody(resp)
		// Clone the request body again
		if req.Body != nil {
			req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
		}
		// Retry the request
		resp, err = t.transport.RoundTrip(req)
		retries++
	}
	// Return the response
	return resp, err
}

func NewRetryableClient() *http.Client {
	transport := &retryableTransport{
		transport: &http.Transport{},
	}

	return &http.Client{
		Transport: transport,
	}
}

func RetrieveZIPTemplateFolder(templateURL *url.URL, tempDir string) (string, error) {
	if templateURL.Scheme == "" {
		return "", fmt.Errorf("invalid template URL: %s", templateURL.String())
	}
	client := NewRetryableClient()
	packageRequest, err := http.NewRequest(http.MethodGet, templateURL.String(), bytes.NewReader([]byte{}))
	if err != nil {
		return "", err
	}
	packageRequest.Header.Set("Accept", "application/zip")
	packageResponse, err := client.Do(packageRequest)
	if err != nil {
		return "", err
	}
	packageResponseBody, err := io.ReadAll(packageResponse.Body)
	if err != nil {
		return "", err
	}
	archive, err := zip.NewReader(bytes.NewReader(packageResponseBody), int64(len(packageResponseBody)))
	if err != nil {
		return "", err
	}
	for _, file := range archive.File {
		filePath, err := sanitizeArchivePath(tempDir, file.Name)
		if err != nil {
			return "", err
		}
		if file.FileHeader.FileInfo().IsDir() {
			err = os.MkdirAll(filePath, 0o777)
			if err != nil {
				return "", err
			}
		} else {
			fileReader, err := file.Open()
			if err != nil {
				return "", err
			}
			defer fileReader.Close()
			destinationFile, err := os.Create(filePath)
			if err != nil {
				return "", err
			}
			defer destinationFile.Close()
			_, err = io.Copy(destinationFile, fileReader) // #nosec G110
			if err != nil {
				return "", err
			}
		}
	}
	return tempDir, nil
}
