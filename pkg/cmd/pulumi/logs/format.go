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

package logs

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
)

// formatLogRecords reads JSON log lines from r, reconstructs formatted
// messages from pulumi.log.arg* fields, removes those fields, and
// writes the resulting JSON to w.
func formatLogRecords(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	enc := json.NewEncoder(w)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec map[string]any
		if err := json.Unmarshal(line, &rec); err != nil {
			// Not JSON — write through as-is (e.g. old plain-text logs).
			fmt.Fprintf(w, "%s\n", line)
			continue
		}

		formatSlogArgs(rec, false)

		if err := enc.Encode(rec); err != nil {
			return err
		}
	}
	return scanner.Err()
}

// formatSlogArgs reconstructs the message from slog-style structured log
// records. The slog handler stores the format string as "msg" and each
// argument as "pulumi.log.argN". This function formats them back into
// the message using fmt.Sprintf and removes the arg keys.
//
// When redact is true, complex arg values (maps, slices) are replaced
// with "[redacted]" before formatting. This prevents secret property
// values from leaking into the message string — the slog handler
// serializes Go structs like resource.PropertyValue as nested maps,
// losing the Secret type wrapper, so we cannot distinguish secret from
// non-secret property values in the JSON and must redact conservatively.
func formatSlogArgs(rec map[string]any, redact bool) {
	msg, ok := rec["msg"].(string)
	if !ok {
		return
	}

	// Collect pulumi.log.argN keys in order.
	var argKeys []string
	for k := range rec {
		if strings.HasPrefix(k, "pulumi.log.arg") {
			argKeys = append(argKeys, k)
		}
	}
	if len(argKeys) > 0 {
		sort.Strings(argKeys)
		args := make([]any, len(argKeys))
		for i, k := range argKeys {
			v := rec[k]
			if redact {
				v = redactArgValue(v)
			}
			args[i] = v
			delete(rec, k)
		}
		rec["msg"] = fmt.Sprintf(msg, args...)
	}
}

// redactArgValue replaces complex values (maps, slices) with "[redacted]"
// to prevent secret property values from leaking. Simple scalars (strings,
// numbers, bools) are left as-is since they are typically labels or
// identifiers, not secret content.
func redactArgValue(v any) any {
	switch v.(type) {
	case map[string]any, []any:
		return "[redacted]"
	default:
		return v
	}
}

// redactSecretsInValue recursively walks a JSON value and replaces any
// secret objects (identified by the Pulumi secret signature) with a
// redacted placeholder.
func redactSecretsInValue(v any) {
	switch val := v.(type) {
	case map[string]any:
		if isSecretValue(val) {
			delete(val, "ciphertext")
			delete(val, "plaintext")
			delete(val, "value")
			val["plaintext"] = "[secret]"
			return
		}
		for _, child := range val {
			redactSecretsInValue(child)
		}
	case []any:
		for _, child := range val {
			redactSecretsInValue(child)
		}
	}
}

// isSecretValue returns true if the map represents a Pulumi secret value.
func isSecretValue(m map[string]any) bool {
	sigVal, ok := m[resource.SigKey]
	if !ok {
		return false
	}
	sigStr, ok := sigVal.(string)
	return ok && sigStr == resource.SecretSig
}
