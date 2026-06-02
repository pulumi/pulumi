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

package client

// Version is the version string embedded in the ESC client's User-Agent header.
//
// It is empty by default. The Pulumi CLI sets it at startup from the CLI's own
// version (see pkg/v3/cmd/esc/cli), preserving the historical
// "esc-sdk/1 (<version>; <os>)" User-Agent. SDK consumers may also set it.
var Version string
