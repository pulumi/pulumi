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

package api

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/itchyny/gojq"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// reconcileJQOutput resolves the effective output mode for a subcommand
// that accepts both --jq and --output. --jq requires JSON output; when
// --output is unset we auto-upgrade to JSON, when it's set to something
// else we refuse rather than silently overriding the caller's choice.
func reconcileJQOutput(mode OutputFormat, jq, output string, outputSet bool) (OutputFormat, error) {
	if jq == "" {
		return mode, nil
	}
	if !outputSet {
		return OutputJSON, nil
	}
	if mode != OutputJSON {
		return mode, NewAPIError(cmdutil.ExitConfigurationError, ErrInvalidFlags,
			"--jq is incompatible with --output="+output).WithField("jq")
	}
	return mode, nil
}

// writeJSONEnvelope serializes env to stdout as JSON, optionally piping
// through a jq filter. label names the envelope for the error path.
func writeJSONEnvelope(env any, jq, label string) error {
	if jq == "" {
		return WriteJSON(os.Stdout, env, stdoutIsTTY())
	}
	body, err := json.Marshal(env)
	if err != nil {
		return NewAPIError(cmdutil.ExitInternalError, ErrToolError,
			fmt.Sprintf("serializing %s envelope: %v", label, err))
	}
	return ApplyJQ(os.Stdout, body, jq)
}

// ApplyJQ runs expr against the JSON document in data and writes each result
// to w as its own line. Strings emit as their raw value (no surrounding
// quotes) so pipelines like `jq '.githubLogin' | xargs ...` work; every
// other type emits as compact JSON.
//
// Returns an APIError on expression parse / evaluation failure.
func ApplyJQ(w io.Writer, data []byte, expr string) error {
	parsed, err := gojq.Parse(expr)
	if err != nil {
		return NewAPIError(cmdutil.ExitCodeError, ErrInvalidFlags,
			fmt.Sprintf("parsing --jq expression: %v", err)).WithField("jq")
	}

	var doc any
	if err := json.Unmarshal(data, &doc); err != nil {
		return NewAPIError(cmdutil.ExitCodeError, ErrInvalidFlags,
			fmt.Sprintf("response is not valid JSON (cannot --jq): %v", err)).WithField("jq")
	}

	iter := parsed.Run(doc)
	for {
		v, ok := iter.Next()
		if !ok {
			return nil
		}
		if qerr, isErr := v.(error); isErr {
			return NewAPIError(cmdutil.ExitCodeError, ErrInvalidFlags,
				fmt.Sprintf("--jq evaluation: %v", qerr)).WithField("jq")
		}
		switch vv := v.(type) {
		case string:
			fmt.Fprintln(w, vv)
		case nil:
			fmt.Fprintln(w, "null")
		default:
			b, err := json.Marshal(vv)
			if err != nil {
				return NewAPIError(cmdutil.ExitInternalError, ErrToolError,
					fmt.Sprintf("serializing --jq result: %v", err))
			}
			fmt.Fprintln(w, string(b))
		}
	}
}
