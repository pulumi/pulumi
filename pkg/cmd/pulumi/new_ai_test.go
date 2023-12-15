package main

import (
	"bytes"
	"context"
	"net/http"
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
