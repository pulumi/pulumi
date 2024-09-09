// Copyright 2019-2024, Pulumi Corporation.
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

package resource

type CustomTimeouts struct {
	Create float64 `json:"create,omitempty" yaml:"create,omitempty"`
	Update float64 `json:"update,omitempty" yaml:"update,omitempty"`
	Delete float64 `json:"delete,omitempty" yaml:"delete,omitempty"`
}

func (c *CustomTimeouts) IsNotEmpty() bool {
	return c.Delete != 0 || c.Update != 0 || c.Create != 0
}
