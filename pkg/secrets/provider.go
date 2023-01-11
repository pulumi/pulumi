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

package secrets

import (
	"encoding/json"
)

// Provider allows for the creation of secrets managers based on a well-known type name.
type Provider interface {
	// OfType returns a secrets manager for the given type, initialized with its previous state.
	OfType(ty string, state json.RawMessage) (Manager, error)
}
