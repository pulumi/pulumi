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

//go:build tools

// Pin a post-split version of google.golang.org/genproto (the monolithic module).
// SDKs generated from this module transitively pull grpc-gateway, which requires
// an older genproto that still contains googleapis/api/annotations and so collides
// with the split genproto/googleapis/api module. Until this dependency was removed,
// github.com/pulumi/esc kept this floor; pin it explicitly here instead. The
// `tools` build tag keeps this in go.mod (via `go mod tidy`) without compiling it.
package tools

import _ "google.golang.org/genproto/googleapis/logging/v2"
