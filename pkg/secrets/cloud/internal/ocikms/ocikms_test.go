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

package ocikms_test

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/pulumi/pulumi/pkg/v3/secrets/cloud/internal/ocikms"
	"gocloud.dev/secrets"
)

func TestInvalidURLsFailBeforeAuthentication(t *testing.T) {
	t.Setenv("OCI_CLI_AUTH", "api_key")
	t.Setenv("OCI_CLI_CONFIG_FILE", filepath.Join(t.TempDir(), "missing-config"))
	for _, tc := range []struct{ name, uri, want string }{
		{"missing-key", "ocikms://crypto.example.invalid", "requires"},
		{"missing-host", "ocikms:///key", "requires"},
		{"credentials", "ocikms://user:do-not-print@crypto.example.invalid/key", "without credentials"},
		{"unknown-query", "ocikms://crypto.example.invalid/key?token=do-not-print", "unsupported"},
		{"invalid-query", "ocikms://crypto.example.invalid/key?profile=%ZZ", "invalid"},
		{"duplicate-profile", "ocikms://crypto.example.invalid/key?profile=a&profile=b", "one nonempty"},
		{"empty-profile", "ocikms://crypto.example.invalid/key?profile=", "one nonempty"},
		{"fragment", "ocikms://crypto.example.invalid/key#do-not-print", "without credentials"},
		{"algorithm", "ocikms://crypto.example.invalid/key?algo=invalid", "invalid encryption algorithm"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := secrets.OpenKeeper(t.Context(), tc.uri)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("wanted %q validation error, got %v", tc.want, err)
			}
			if strings.Contains(err.Error(), "do-not-print") {
				t.Fatal("validation error disclosed URL input")
			}
		})
	}
}

func TestCanceledOpenDoesNotLoadCredentials(t *testing.T) {
	t.Setenv("OCI_CLI_AUTH", "unsupported")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := secrets.OpenKeeper(ctx, "ocikms://crypto.example.invalid/key")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("wanted cancellation before authentication, got %v", err)
	}
}

func TestUnsupportedAuthenticationFailsExplicitly(t *testing.T) {
	t.Setenv("OCI_CLI_AUTH", "unsupported")
	_, err := secrets.OpenKeeper(t.Context(), "ocikms://crypto.example.invalid/key")
	if err == nil || !strings.Contains(err.Error(), "unsupported OCI_CLI_AUTH") {
		t.Fatalf("wanted authentication selection error, got %v", err)
	}
}
