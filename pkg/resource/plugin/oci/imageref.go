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

package oci

import (
	"errors"
	"fmt"
	"strings"
)

// ResolveRef computes the effective image reference for a policy pack from the
// manifest's "image" runtime option, the pack version, and an explicit tag
// override (from `pulumi policy publish --tag`).
//
// An explicit tag or digest in image pins the ref outright. Otherwise the pack
// version is the tag — a Pulumi-defined default (a bare Docker ref would
// normally imply :latest). With no tag anywhere the ref falls back to :latest
// and tagged is false, so callers that need a real tag (publish) can reject it
// while local dev accepts it.
func ResolveRef(image, version, tagOverride string) (ref string, tagged bool, err error) {
	if image == "" {
		return "", false, errors.New("no image specified")
	}
	if strings.Contains(image, "@") {
		if tagOverride != "" {
			return "", false, fmt.Errorf("image %q is digest-pinned; --tag cannot override it", image)
		}
		return image, true, nil
	}
	// A ":" after the last "/" is a tag; before it, a registry port.
	lastSlash := strings.LastIndex(image, "/")
	if strings.Contains(image[lastSlash+1:], ":") {
		if tagOverride != "" {
			return "", false, fmt.Errorf("image %q already has a tag; --tag cannot override it", image)
		}
		return image, true, nil
	}
	switch {
	case tagOverride != "":
		return image + ":" + tagOverride, true, nil
	case version != "":
		return image + ":" + version, true, nil
	default:
		return image + ":latest", false, nil
	}
}
