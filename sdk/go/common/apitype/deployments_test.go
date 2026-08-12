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

package apitype

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestSecretValueUnmarshalJSON(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  SecretValue
	}{
		{
			name:  "legacy plaintext string",
			input: `"plain"`,
			want:  SecretValue{Value: "plain", Secret: false},
		},
		{
			name:  "legacy empty string",
			input: `""`,
			want:  SecretValue{Value: "", Secret: false},
		},
		{
			name:  "legacy secret object",
			input: `{"secret":"plain"}`,
			want:  SecretValue{Value: "plain", Secret: true},
		},
		{
			name:  "legacy ciphertext object",
			input: `{"ciphertext":"encrypted"}`,
			want:  SecretValue{Ciphertext: "encrypted", Secret: true},
		},
		{
			name:  "legacy secret + ciphertext",
			input: `{"secret":"plain","ciphertext":"encrypted"}`,
			want:  SecretValue{Value: "plain", Ciphertext: "encrypted", Secret: true},
		},
		{
			name:  "new form explicit non-secret",
			input: `{"isSecret":false,"value":"plain"}`,
			want:  SecretValue{Value: "plain", Secret: false},
		},
		{
			name:  "new form explicit secret with plaintext",
			input: `{"isSecret":true,"value":"plain"}`,
			want:  SecretValue{Value: "plain", Secret: true},
		},
		{
			name:  "new form explicit secret with ciphertext",
			input: `{"isSecret":true,"ciphertext":"encrypted"}`,
			want:  SecretValue{Ciphertext: "encrypted", Secret: true},
		},
		{
			name:  "new form explicit non-secret with empty value",
			input: `{"isSecret":false}`,
			want:  SecretValue{Value: "", Secret: false},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got SecretValue
			require.NoError(t, json.Unmarshal([]byte(tc.input), &got))
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSecretValueUnmarshalYAML(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  SecretValue
	}{
		{
			name:  "legacy plaintext string",
			input: `plain`,
			want:  SecretValue{Value: "plain", Secret: false},
		},
		{
			name:  "legacy secret object",
			input: `secret: plain`,
			want:  SecretValue{Value: "plain", Secret: true},
		},
		{
			name:  "legacy ciphertext object",
			input: `ciphertext: encrypted`,
			want:  SecretValue{Ciphertext: "encrypted", Secret: true},
		},
		{
			name:  "new form explicit non-secret",
			input: "isSecret: false\nvalue: plain",
			want:  SecretValue{Value: "plain", Secret: false},
		},
		{
			name:  "new form explicit secret with plaintext",
			input: "isSecret: true\nvalue: plain",
			want:  SecretValue{Value: "plain", Secret: true},
		},
		{
			name:  "new form explicit secret with ciphertext",
			input: "isSecret: true\nciphertext: encrypted",
			want:  SecretValue{Ciphertext: "encrypted", Secret: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got SecretValue
			require.NoError(t, yaml.Unmarshal([]byte(tc.input), &got))
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestSecretValueUnmarshalJSONError covers inputs that are valid JSON but cannot decode as
// either the object form (bare string, legacy, or new explicit shape) or the fallback string
// form. UnmarshalJSON should surface the string-decode error to the caller.
func TestSecretValueUnmarshalJSONError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
	}{
		{name: "bare number", input: `42`},
		{name: "bare bool", input: `true`},
		{name: "array", input: `[1,2,3]`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got SecretValue
			assert.Error(t, json.Unmarshal([]byte(tc.input), &got))
		})
	}
}

// TestSecretValueUnmarshalYAMLError covers inputs that are valid YAML but cannot decode as
// either the object form or the fallback string form. UnmarshalYAML should surface the
// string-decode error to the caller.
func TestSecretValueUnmarshalYAMLError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
	}{
		{name: "sequence", input: "- 1\n- 2\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got SecretValue
			assert.Error(t, yaml.Unmarshal([]byte(tc.input), &got))
		})
	}
}

// TestSecretValueMarshalJSONUnchanged locks down the write-side wire format. The staged
// rollout keeps MarshalJSON emitting the legacy heterogeneous form until adoption of the
// new UnmarshalJSON reaches high coverage; this test fails loudly if that changes.
func TestSecretValueMarshalJSONUnchanged(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   SecretValue
		want string
	}{
		{
			name: "non-secret plaintext emits bare string",
			in:   SecretValue{Value: "plain", Secret: false},
			want: `"plain"`,
		},
		{
			name: "secret plaintext emits legacy object",
			in:   SecretValue{Value: "plain", Secret: true},
			want: `{"secret":"plain"}`,
		},
		{
			name: "ciphertext emits legacy object",
			in:   SecretValue{Ciphertext: "encrypted", Secret: true},
			want: `{"ciphertext":"encrypted"}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := json.Marshal(tc.in)
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(got))
		})
	}
}

// TestSecretValueMarshalYAMLUnchanged locks down the on-disk YAML format written by
// `pulumi deployment settings pull` into the user-facing `Pulumi.stack.deploy.yaml` file.
// That file is source-controlled; its schema is a user-facing UX commitment. Even when the
// service eventually flips its JSON wire format to an explicit object form, the CLI's
// MarshalYAML must continue to emit the compact heterogeneous form: a bare string for
// non-secret values, a `{secret: ...}` object for secrets, and a `{ciphertext: ...}` object
// for encrypted secrets. This test fails loudly if that promise is broken.
func TestSecretValueMarshalYAMLUnchanged(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   SecretValue
		want string
	}{
		{
			name: "non-secret plaintext emits bare string",
			in:   SecretValue{Value: "plain", Secret: false},
			want: "plain\n",
		},
		{
			name: "secret plaintext emits legacy object",
			in:   SecretValue{Value: "plain", Secret: true},
			want: "secret: plain\n",
		},
		{
			name: "ciphertext emits legacy object",
			in:   SecretValue{Ciphertext: "encrypted", Secret: true},
			want: "ciphertext: encrypted\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := yaml.Marshal(tc.in)
			require.NoError(t, err)
			assert.Equal(t, tc.want, string(got))
		})
	}
}

// TestSecretValueRoundTripJSON verifies that values produced by MarshalJSON are read back
// to structurally-equivalent SecretValues. The legacy forms should round-trip exactly; the
// new explicit form isn't emitted yet but is parsed, so we round-trip it manually.
func TestSecretValueRoundTripJSON(t *testing.T) {
	t.Parallel()

	t.Run("legacy writer", func(t *testing.T) {
		t.Parallel()
		inputs := []SecretValue{
			{Value: "plain", Secret: false},
			{Value: "plain", Secret: true},
			{Ciphertext: "encrypted", Secret: true},
		}
		for _, in := range inputs {
			bytes, err := json.Marshal(in)
			require.NoError(t, err)
			var out SecretValue
			require.NoError(t, json.Unmarshal(bytes, &out))
			assert.Equal(t, in, out)
		}
	})

	t.Run("new form parses", func(t *testing.T) {
		t.Parallel()
		// Future senders will emit the explicit form. These fixtures cover the cases the
		// CLI must be tolerant of once the service side flips.
		cases := []struct {
			wire string
			want SecretValue
		}{
			{`{"isSecret":false,"value":"plain"}`, SecretValue{Value: "plain", Secret: false}},
			{`{"isSecret":true,"value":"plain"}`, SecretValue{Value: "plain", Secret: true}},
			{`{"isSecret":true,"ciphertext":"encrypted"}`, SecretValue{Ciphertext: "encrypted", Secret: true}},
		}
		for _, tc := range cases {
			var got SecretValue
			require.NoError(t, json.Unmarshal([]byte(tc.wire), &got))
			assert.Equal(t, tc.want, got)
		}
	})
}

// DockerImage shares SecretValue's heterogeneous wire-format anti-pattern: MarshalJSON
// emits a bare string when only Reference is set and an object otherwise. The same
// staged convergence is planned (see pulumi/pulumi-service TYPE_MIGRATION_PLAN.md
// Phase 1a). Unlike SecretValue, DockerImage's UnmarshalJSON already accepts both
// forms, so no compat-read code change is needed in the CLI — but we lock in the
// current MarshalJSON output and the dual-form UnmarshalJSON behavior so a future
// flip is intentional and reviewable.

// TestDockerImageMarshalJSONUnchanged locks down the current heterogeneous JSON output:
// a bare string when only Reference is set, an object when Credentials is also present.
// When the Pulumi Service flips its DockerImage MarshalJSON to always emit object form
// (Phase 1a Stage 2), this test will fail and prompt a coordinated update.
func TestDockerImageMarshalJSONUnchanged(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		in   DockerImage
		want string
	}{
		{
			name: "reference only emits bare string",
			in:   DockerImage{Reference: "alpine:latest"},
			want: `"alpine:latest"`,
		},
		{
			name: "reference + credentials emits object",
			in: DockerImage{
				Reference: "alpine:latest",
				Credentials: &DockerImageCredentials{
					Username: "user",
					Password: SecretValue{Value: "pw", Secret: true},
				},
			},
			want: `{"reference":"alpine:latest","credentials":{"username":"user","password":{"secret":"pw"}}}`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := json.Marshal(&tc.in)
			require.NoError(t, err)
			assert.JSONEq(t, tc.want, string(got))
		})
	}
}

// TestDockerImageUnmarshalJSON verifies that DockerImage's existing UnmarshalJSON accepts
// both the bare-string and object wire forms. This is regression coverage: when the
// service eventually starts emitting only the object form (Phase 1a Stage 2), bare-string
// support must remain so older saved data continues to parse.
func TestDockerImageUnmarshalJSON(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
		want  DockerImage
	}{
		{
			name:  "bare string",
			input: `"alpine:latest"`,
			want:  DockerImage{Reference: "alpine:latest"},
		},
		{
			name:  "object with reference only",
			input: `{"reference":"alpine:latest"}`,
			want:  DockerImage{Reference: "alpine:latest"},
		},
		{
			name:  "object with credentials",
			input: `{"reference":"alpine:latest","credentials":{"username":"user","password":{"secret":"pw"}}}`,
			want: DockerImage{
				Reference: "alpine:latest",
				Credentials: &DockerImageCredentials{
					Username: "user",
					Password: SecretValue{Value: "pw", Secret: true},
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got DockerImage
			require.NoError(t, json.Unmarshal([]byte(tc.input), &got))
			assert.Equal(t, tc.want, got)
		})
	}
}
