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

package convert

import "context"

// An interface to map provider names (N.B. These aren't Pulumi provider names, but the names of "providers"
// in the source language being converted from) to plugin specific mapping data.
type Mapper interface {
	// Returns plugin specific mapping data for the given provider name. The "pulumiProvider" is used as a
	// hint for which pulumi plugin will provider this mapping. Returns an empty result if no mapping
	// information was available.
	GetMapping(ctx context.Context, provider string, pulumiProvider string) ([]byte, error)
}
