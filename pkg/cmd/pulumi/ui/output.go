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

package ui

import "fmt"

// OutputRenderers holds the renderers a CLI command offers under the --output flag.
type OutputRenderers[F any] struct {
	// Default is the renderer used for unspecified or "default" --output values.
	Default F
	// JSON is the renderer used for "json".
	JSON F
}

// Renderer returns the renderer selected by an --output flag value. The empty string
// and "default" select Default; "json" selects JSON; any other value yields an error.
//
// Use it from a cobra RunE like:
//
//	render, err := ui.Renderer(output, ui.OutputRenderers[myRenderFunc]{
//	    Default: renderItemsTable,
//	    JSON:    renderItemsJSON,
//	})
//	if err != nil {
//	    return err
//	}
//	return render(out, data)
func Renderer[F any](output string, r OutputRenderers[F]) (F, error) {
	switch output {
	case "", "default":
		return r.Default, nil
	case "json":
		return r.JSON, nil
	default:
		var zero F
		return zero, fmt.Errorf(
			"invalid --output value %q: expected \"default\" or \"json\"", output)
	}
}
