// Copyright 2026, Pulumi Corporation.
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

// Package authtest provides a fake credential helper and a fake HTTP backend for tests.
package authtest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/pkg/v3/auth/credentialhelper"
)

const (
	helperFlag     = "--credential-helper-test"
	discoveredName = "pulumi-credential-helper"
	repliesFile    = "replies.json"
	requestsFile   = "requests.jsonl"
)

// Reply is what the fake helper does for one request.
type Reply struct {
	// Response is written to stdout as JSON.
	Response any `json:"response"`
	// Stderr is written to stderr before the response.
	Stderr string `json:"stderr,omitempty"`
}

// Helper is a fake credential helper: the test binary itself, started with arguments that make
// RunHelper answer from a file of replies.
type Helper struct {
	Path string
	Args []string
	dir  string
}

// RunHelper answers a credential helper request when the test binary was started as a fake helper.
// It reports whether it was, and the code to exit with. Call it first in TestMain.
func RunHelper() (exitCode int, ran bool) {
	var dir string
	switch {
	case len(os.Args) >= 3 && os.Args[1] == helperFlag:
		dir = os.Args[2]
	case strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe") == discoveredName:
		// A discovered helper gets no arguments, so its replies are beside it.
		dir = filepath.Dir(os.Args[0])
	default:
		return 0, false
	}
	if err := runHelper(dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1, true
	}
	return 0, true
}

func runHelper(dir string) error {
	var request credentialhelper.Request
	if err := json.NewDecoder(os.Stdin).Decode(&request); err != nil {
		return err
	}
	log, err := os.OpenFile(filepath.Join(dir, requestsFile), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer log.Close()
	if err := json.NewEncoder(log).Encode(request); err != nil {
		return err
	}
	data, err := os.ReadFile(filepath.Join(dir, repliesFile))
	if err != nil {
		return err
	}
	var replies map[string]Reply
	if err := json.Unmarshal(data, &replies); err != nil {
		return err
	}
	reply, ok := replies[string(request.Reason)+" "+request.SelectedBackendURL]
	if !ok {
		reply, ok = replies[string(request.Reason)]
	}
	if !ok {
		return fmt.Errorf("unexpected helper request: %s %s", request.Reason, request.SelectedBackendURL)
	}
	if _, err := io.WriteString(os.Stderr, reply.Stderr); err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(reply.Response)
}

// NewHelper returns a fake helper that answers from replies. A reply is found by the request's reason
// and selected backend URL, as "<reason> <url>", or else by its reason alone. Its value is the
// response to return, or a Reply. A request without a reply fails the helper.
func NewHelper(t testing.TB, replies map[string]any) *Helper {
	t.Helper()
	return NewHelperIn(t, filepath.Join(t.TempDir(), "helper files, with spaces"), replies)
}

// NewHelperIn is like NewHelper and keeps the helper's files in dir. A copy of the test binary named
// pulumi-credential-helper in dir answers from the same replies.
func NewHelperIn(t testing.TB, dir string, replies map[string]any) *Helper {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	executable, err = filepath.EvalSymlinks(executable)
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(dir, 0o700))
	normalized := map[string]Reply{}
	for key, reply := range replies {
		if reply, ok := reply.(Reply); ok {
			normalized[key] = reply
			continue
		}
		normalized[key] = Reply{Response: reply}
	}
	data, err := json.Marshal(normalized)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, repliesFile), data, 0o600))
	return &Helper{Path: executable, Args: []string{helperFlag, dir}, dir: dir}
}

// Requests returns the requests the helper has received, in order.
func (h *Helper) Requests(t testing.TB) []credentialhelper.Request {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(h.dir, requestsFile))
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(t, err)
	decoder := json.NewDecoder(bytes.NewReader(data))
	var requests []credentialhelper.Request
	for {
		var request credentialhelper.Request
		err := decoder.Decode(&request)
		if err == io.EOF {
			return requests
		}
		require.NoError(t, err)
		requests = append(requests, request)
	}
}

// ServeBackend answers the requests every command makes of an HTTP backend: the logged-in user and
// the backend's capabilities. routes maps further paths to their response bodies and may replace
// those two. Any other request fails the test.
func ServeBackend(t testing.TB, w http.ResponseWriter, r *http.Request, user string, routes map[string]string) {
	body, ok := routes[r.URL.Path]
	switch {
	case ok:
	case r.URL.Path == "/api/user":
		body = fmt.Sprintf(`{"githubLogin":%q,"organizations":[]}`, user)
	case r.URL.Path == "/api/capabilities":
		body = `{}`
	default:
		t.Errorf("unexpected backend request: %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if _, err := io.WriteString(w, body); err != nil {
		t.Error(err)
	}
}
