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

package filestate

import (
	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
)

// This is just a compat shim for ESC which calls this method as "filestate.IsFileStateBackendURL".
// Deprecated: Use diy.IsDIYBackendURL instead.
func IsFileStateBackendURL(url string) bool {
	return diy.IsDIYBackendURL(url)
}
