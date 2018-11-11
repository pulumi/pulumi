// Copyright 2016-2018, Pulumi Corporation.
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

import (
	"net/http"
	"net/url"
	"path"

	"github.com/gorilla/mux"
)

// cleanPath returns the canonical path for p, eliminating . and .. elements.
// Borrowed from gorilla/mux.
func cleanPath(p string) string {
	if p == "" {
		return "/"
	}

	if p[0] != '/' {
		p = "/" + p
	}
	np := path.Clean(p)

	// path.Clean removes trailing slash except for root;
	// put the trailing slash back if necessary.
	if p[len(p)-1] == '/' && np != "/" {
		np += "/"
	}

	return np
}

// getEndpoint gets the friendly name of the endpoint with the given method and path.
func getEndpointName(method, path string) string {
	path = cleanPath(path)

	u, err := url.Parse("http://localhost" + path)
	if err != nil {
		return "unknown"
	}

	req := http.Request{
		Method: method,
		URL:    u,
	}
	var match mux.RouteMatch
	if !routes.Match(&req, &match) {
		return "unknown"
	}

	return match.Route.GetName()
}

// routes is the canonical muxer we use to determine friendly names for Pulumi APIs.
var routes *mux.Router

// nolint: lll
func init() {
	routes = mux.NewRouter()

	// addEndpoint registers the endpoint with the indicated method, path, and friendly name with the route table.
	addEndpoint := func(method, path, name string) {
		routes.Path(path).Methods(method).Name(name)
	}

	addEndpoint("GET", "/api/user", "getCurrentUser")
	addEndpoint("GET", "/api/user/stacks", "listUserStacks")
	addEndpoint("GET", "/api/stacks/{orgName}", "listOrganizationStacks")
	addEndpoint("POST", "/api/stacks/{orgName}", "createStack")
	addEndpoint("DELETE", "/api/stacks/{orgName}/{stackName}", "deleteStack")
	addEndpoint("GET", "/api/stacks/{orgName}/{stackName}", "getStack")
	addEndpoint("GET", "/api/stacks/{orgName}/{stackName}/export", "exportStack")
	addEndpoint("POST", "/api/stacks/{orgName}/{stackName}/import", "importStack")
	addEndpoint("POST", "/api/stacks/{orgName}/{stackName}/encrypt", "encryptValue")
	addEndpoint("POST", "/api/stacks/{orgName}/{stackName}/decrypt", "decryptValue")
	addEndpoint("GET", "/api/stacks/{orgName}/{stackName}/logs", "getStackLogs")
	addEndpoint("GET", "/api/stacks/{orgName}/{stackName}/updates", "getStackUpdates")
	addEndpoint("GET", "/api/stacks/{orgName}/{stackName}/updates/latest", "getLatestStackUpdate")
	addEndpoint("GET", "/api/stacks/{orgName}/{stackName}/updates/{version}", "getStackUpdate")
	addEndpoint("GET", "/api/stacks/{orgName}/{stackName}/updates/{version}/contents/files", "getUpdateContentsFiles")
	addEndpoint("GET", "/api/stacks/{orgName}/{stackName}/updates/{version}/contents/file/{path:.*}", "getUpdateContentsFilePath")
	addEndpoint("POST", "/api/stacks/{orgName}/{stackName}/destroy", "destroyStack")
	addEndpoint("GET", "/api/stacks/{orgName}/{stackName}/destroy/{updateID}", "getDestroyStatus")
	addEndpoint("POST", "/api/stacks/{orgName}/{stackName}/destroy/{updateID}", "startDestroy")
	addEndpoint("PATCH", "/api/stacks/{orgName}/{stackName}/destroy/{updateID}/checkpoint", "patchUpdateCheckpoint")
	addEndpoint("POST", "/api/stacks/{orgName}/{stackName}/destroy/{updateID}/complete", "completeUpdate")
	addEndpoint("POST", "/api/stacks/{orgName}/{stackName}/destroy/{updateID}/log", "appendUpdateLogEntry")
	addEndpoint("POST", "/api/stacks/{orgName}/{stackName}/destroy/{updateID}/renew_lease", "renewUpdateLease")
	addEndpoint("POST", "/api/stacks/{orgName}/{stackName}/preview", "previewUpdate")
	addEndpoint("GET", "/api/stacks/{orgName}/{stackName}/preview/{updateID}", "getPreviewStatus")
	addEndpoint("POST", "/api/stacks/{orgName}/{stackName}/preview/{updateID}", "startPreview")
	addEndpoint("POST", "/api/stacks/{orgName}/{stackName}/update", "updateStack")
	addEndpoint("GET", "/api/stacks/{orgName}/{stackName}/update/{updateID}", "getUpdateStatus")
	addEndpoint("POST", "/api/stacks/{orgName}/{stackName}/update/{updateID}", "startUpdate")
	addEndpoint("PATCH", "/api/stacks/{orgName}/{stackName}/update/{updateID}/checkpoint", "patchUpdateCheckpoint")
	addEndpoint("POST", "/api/stacks/{orgName}/{stackName}/update/{updateID}/complete", "completeUpdate")
	addEndpoint("POST", "/api/stacks/{orgName}/{stackName}/update/{updateID}/log", "appendUpdateLogEntry")
	addEndpoint("POST", "/api/stacks/{orgName}/{stackName}/update/{updateID}/renew_lease", "renewUpdateLease")
}
