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

package iter

import (
	"iter"

	"github.com/pgavlin/fx/v2"
)

// Cast wraps an iter.Seq[T] so each yielded value is converted to U. T and U must
// share an underlying string type; the typical use is to move between plain `string`
// and a named string type like `resource.PropertyKey` or `tokens.Name` mid-pipeline,
// without materializing an intermediate slice. Specify U explicitly at the call site;
// T is inferred from the argument, e.g. `iter.Cast[resource.PropertyKey](names)`.
func Cast[U, T ~string](s iter.Seq[T]) iter.Seq[U] {
	return fx.Map(s, func(v T) U {
		return U(v)
	})
}
