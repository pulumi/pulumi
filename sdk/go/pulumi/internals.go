// Copyright 2016-2022, Pulumi Corporation.
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

package pulumi

import "context"

// Functions in this file are exposed in pulumi/internals via go:linkname
func awaitWithContext(ctx context.Context, o Output) (interface{}, bool, bool, []Resource, error) {
	value, known, secret, deps, err := o.getState().await(ctx)

	return value, known, secret, deps, err
}
