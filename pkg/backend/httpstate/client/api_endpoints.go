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

	return "api/" + match.Route.GetName()
}

// routes is the canonical muxer we use to determine friendly names for Pulumi APIs.
var routes *mux.Router

//nolint:lll
func init() {
	routes = mux.NewRouter()

	// addEndpoint registers the endpoint with the indicated method, path, and friendly name with the route table.
	// We use this to provide more user-friendly names for the endpoints for annotating trace logs.
	addEndpoint := func(method, path, name string) {
		routes.Path(path).Methods(method).Name(name)
	}

	addEndpoint("GET", "/api/capabilities", "getCapabilities")

	addEndpoint("GET", "/api/user", "getCurrentUser")
	addEndpoint("GET", "/api/user/stacks", "listUserStacks")
	addEndpoint("GET", "/api/user/organizations/default", "getDefaultOrg")
	addEndpoint("GET", "/api/stacks/{orgName}", "listOrganizationStacks")
	addEndpoint("POST", "/api/stacks/{orgName}", "createStack")
	addEndpoint("DELETE", "/api/stacks/{orgName}/{projectName}/{stackName}", "deleteStack")
	addEndpoint("GET", "/api/stacks/{orgName}/{projectName}/{stackName}", "getStack")
	addEndpoint("GET", "/api/stacks/{orgName}/{projectName}/{stackName}/export", "exportStack")
	addEndpoint("POST", "/api/stacks/{orgName}/{projectName}/{stackName}/import", "importStack")
	addEndpoint("POST", "/api/stacks/{orgName}/{projectName}/{stackName}/encrypt", "encryptValue")
	addEndpoint("POST", "/api/stacks/{orgName}/{projectName}/{stackName}/decrypt", "decryptValue")
	addEndpoint("GET", "/api/stacks/{orgName}/{projectName}/{stackName}/logs", "getStackLogs")
	addEndpoint("GET", "/api/stacks/{orgName}/{projectName}/{stackName}/updates", "getStackUpdates")
	addEndpoint("GET", "/api/stacks/{orgName}/{projectName}/{stackName}/updates/latest", "getLatestStackUpdate")
	addEndpoint("GET", "/api/stacks/{orgName}/{projectName}/{stackName}/updates/{version}", "getStackUpdate")
	addEndpoint("GET", "/api/stacks/{orgName}/{projectName}/{stackName}/updates/{version}/contents/files", "getUpdateContentsFiles")
	addEndpoint("GET", "/api/stacks/{orgName}/{projectName}/{stackName}/updates/{version}/contents/file/{path:.*}", "getUpdateContentsFilePath")
	addEndpoint("GET", "/api/orgs/{orgName}/templates", "listTemplates")
	addEndpoint("GET", "/api/orgs/{orgName}/templates/download", "downloadTemplates")
	addEndpoint("POST", "/api/stacks/{orgName}/{projectName}/{stackName}/batch-decrypt", "batchDecrypt")
	addEndpoint("POST", "/api/stacks/{orgName}/{projectName}/{stackName}/batch-encrypt", "batchEncrypt")

	// The APIs for performing updates of various kind all have the same set of API endpoints. Only
	// differentiate the "create update of kind X" APIs, and introduce a pseudo route param "updateKind".
	addEndpoint("POST", "/api/stacks/{orgName}/{projectName}/{stackName}/destroy", "createDestroy")
	addEndpoint("POST", "/api/stacks/{orgName}/{projectName}/{stackName}/preview", "createPreview")
	addEndpoint("POST", "/api/stacks/{orgName}/{projectName}/{stackName}/update", "createUpdate")

	addEndpoint("GET", "/api/stacks/{orgName}/{projectName}/{stackName}/{updateKind}/{updateID}", "getUpdateStatus")
	addEndpoint("POST", "/api/stacks/{orgName}/{projectName}/{stackName}/{updateKind}/{updateID}", "startUpdate")
	addEndpoint("PATCH", "/api/stacks/{orgName}/{projectName}/{stackName}/{updateKind}/{updateID}/checkpoint", "patchCheckpoint")
	addEndpoint("PATCH", "/api/stacks/{orgName}/{projectName}/{stackName}/{updateKind}/{updateID}/checkpointdelta", "patchCheckpointDelta")
	addEndpoint("PATCH", "/api/stacks/{orgName}/{projectName}/{stackName}/{updateKind}/{updateID}/checkpointverbatim", "patchCheckpointVerbatim")
	addEndpoint("POST", "/api/stacks/{orgName}/{projectName}/{stackName}/{updateKind}/{updateID}/complete", "completeUpdate")
	addEndpoint("POST", "/api/stacks/{orgName}/{projectName}/{stackName}/{updateKind}/{updateID}/events", "postEngineEvent")
	addEndpoint("POST", "/api/stacks/{orgName}/{projectName}/{stackName}/{updateKind}/{updateID}/events/batch", "postEngineEventBatch")
	addEndpoint("POST", "/api/stacks/{orgName}/{projectName}/{stackName}/{updateKind}/{updateID}/renew_lease", "renewLease")

	// APIs for managing `PolicyPack`s.
	addEndpoint("POST", "/api/orgs/{orgName}/policypacks", "publishPolicyPack")

	// APIs for managing Search capabilities
	addEndpoint("GET", "/api/orgs/{orgName}/search/resources", "getSearchResources")
	addEndpoint("GET", "/api/orgs/{orgName}/search/resources/parse", "getSearchResourcesParse")

	// APIs for interacting with the Package Registry
	addEndpoint("POST", "/api/preview/registry/packages/{source}/{publisher}/{name}/versions", "publishPackage")
	addEndpoint("POST", "/api/preview/registry/packages/{source}/{publisher}/{name}/versions/{version}/complete", "completePackagePublish")
}
