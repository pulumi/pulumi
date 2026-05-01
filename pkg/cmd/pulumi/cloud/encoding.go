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

package cloud

import "net/url"

// paramsRequiringDoubleEncoding lists path-parameter names whose server-side
// handlers decode the captured value more than once. The client must
// URL-encode these twice so a `/` inside the value survives the full decode
// chain. Known routes as of this writing:
//
//   - {accountName} — /api/preview/insights/{orgName}/accounts/{accountName}/...
//   - {resourceTypeAndId} — /api/preview/insights/.../accounts/{accountName}/resources/{resourceTypeAndId}/...
//   - {object} — /api/console/registry/packages/{source}/{publisher}/{name}/versions/{version}/objects/{object}
//
// Any other route is single-decoded. Remove an entry when the service stops
// double-decoding that parameter.
var paramsRequiringDoubleEncoding = map[string]bool{
	"accountName":       true,
	"resourceTypeAndId": true,
	"object":            true,
}

// requiresDoubleEncoding reports whether a path parameter with this name must
// be URL-encoded twice by the client.
func requiresDoubleEncoding(paramName string) bool {
	return paramsRequiringDoubleEncoding[paramName]
}

// escapePathParam URL-encodes val for use in a URL path. When double is true
// the result is encoded twice, matching the server-side double-decode chain
// for the params listed in paramsRequiringDoubleEncoding.
func escapePathParam(val string, double bool) string {
	enc := url.PathEscape(val)
	if double {
		enc = url.PathEscape(enc)
	}
	return enc
}
