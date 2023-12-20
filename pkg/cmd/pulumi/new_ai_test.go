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

package main

import (
	"bytes"
	"context"
	"net/http"
	"runtime"
	"testing"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/stretchr/testify/assert"
)

func TestErrorsOnNonHTTPBackend(t *testing.T) {
	t.Parallel()

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)
	mockBackendInstance(t, &backend.MockBackend{
		DoesProjectExistF: func(ctx context.Context, org string, name string) (bool, error) {
			return name == projectName, nil
		},
	})

	testNewArgs := newArgs{
		aiPrompt:        "prompt",
		aiLanguage:      "typescript",
		interactive:     true,
		secretsProvider: "default",
	}

	assert.ErrorContains(t,
		runNew(
			context.Background(), testNewArgs,
		),
		"please log in to Pulumi Cloud to use Pulumi AI")
}

type mockReaderCloser struct {
	*bytes.Buffer
}

func (mockReaderCloser) Close() error { return nil }

func TestExpectEOFOnHTTPBackend(t *testing.T) {
	if runtime.GOOS == "windows" {
		// This test behaves differently on Windows, due to an interaction between survey & os.Stdin.
		t.Skip()
	}

	t.Parallel()

	tempdir := tempProjectDir(t)
	chdir(t, tempdir)
	mockBackendInstance(t, &httpstate.MockHTTPBackend{
		FPromptAI: func(ctx context.Context, requestBody httpstate.AIPromptRequestBody) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       mockReaderCloser{bytes.NewBufferString("")},
			}, nil
		},
	})

	testNewArgs := newArgs{
		aiPrompt:        "prompt",
		aiLanguage:      "typescript",
		interactive:     true,
		secretsProvider: "default",
	}

	assert.ErrorContains(t,
		runNew(
			context.Background(), testNewArgs,
		), "EOF")
}
