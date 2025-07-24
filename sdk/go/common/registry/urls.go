// Copyright 2025, Pulumi Corporation.
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

package registry

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/blang/semver"
)

const RegistryURLScheme = "registry"

type URLInfo struct {
	resourceType string
	source       string
	publisher    string
	name         string
	version      *semver.Version // optional
}

func (u *URLInfo) ResourceType() string {
	return u.resourceType
}

func (u *URLInfo) Source() string {
	return u.source
}

func (u *URLInfo) Publisher() string {
	return u.publisher
}

func (u *URLInfo) Name() string {
	return u.name
}

func (u *URLInfo) Version() *semver.Version {
	return u.version
}

func ParseRegistryURL(registryURL string) (*URLInfo, error) {
	parsedURL, err := url.Parse(registryURL)
	if err != nil {
		return nil, fmt.Errorf("invalid registry URL: %w", err)
	}

	if parsedURL.Scheme != RegistryURLScheme {
		return nil, fmt.Errorf("invalid registry URL scheme: expected %s, got %s", RegistryURLScheme, parsedURL.Scheme)
	}

	resourceType := parsedURL.Host
	if resourceType == "" {
		return nil, fmt.Errorf("invalid registry URL: missing resource type")
	}

	urlPath := strings.TrimPrefix(parsedURL.Path, "/")
	parts := strings.Split(urlPath, "/")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid registry URL format: expected registry://resource-type/source/publisher/name[@version]")
	}

	source := parts[0]
	if source == "" {
		return nil, fmt.Errorf("invalid registry URL: missing source")
	}

	publisher := parts[1]
	if publisher == "" {
		return nil, fmt.Errorf("invalid registry URL: missing publisher")
	}

	nameAndVersion := parts[2]
	nameVersionParts := strings.Split(nameAndVersion, "@")
	if len(nameVersionParts) > 2 {
		return nil, fmt.Errorf("invalid registry URL format: expected registry://resource-type/source/publisher/name[@version]")
	}

	encodedName := nameVersionParts[0]
	if encodedName == "" {
		return nil, fmt.Errorf("invalid registry URL: missing name")
	}

	name, err := decodeArtifactNamePart(encodedName)
	if err != nil {
		return nil, fmt.Errorf("invalid registry URL: failed to decode name: %w", err)
	}

	var version *semver.Version
	if len(nameVersionParts) == 2 {
		versionStr := nameVersionParts[1]
		if versionStr == "" {
			return nil, fmt.Errorf("invalid registry URL: missing version")
		}
		
		if versionStr != "latest" {
			version, err = parseVersion(versionStr)
			if err != nil {
				return nil, fmt.Errorf("invalid registry URL: invalid version %q: %w", versionStr, err)
			}
		}
	}

	return &URLInfo{
		resourceType: resourceType,
		source:       source,
		publisher:    publisher,
		name:         name,
		version:      version,
	}, nil
}

func (u *URLInfo) String() string {
	encodedName := encodeArtifactNamePart(u.Name())

	if u.Version() == nil {
		return fmt.Sprintf("registry://%s/%s/%s/%s",
			u.ResourceType(), u.Source(), u.Publisher(), encodedName)
	}
	return fmt.Sprintf("registry://%s/%s/%s/%s@%s",
		u.ResourceType(), u.Source(), u.Publisher(), encodedName, u.Version().String())
}

func ParseRegistryURLOrPartial(registryURL string, defaultResourceType string) (*URLInfo, error) {
	if IsRegistryURL(registryURL) {
		return ParseRegistryURL(registryURL)
	}
	return parsePartialRegistryURL(registryURL, defaultResourceType)
}

func parsePartialRegistryURL(registryURL string, defaultResourceType string) (*URLInfo, error) {
	var version *semver.Version
	nameSpec := registryURL
	if idx := strings.LastIndex(registryURL, "@"); idx != -1 {
		versionStr := registryURL[idx+1:]
		if versionStr == "" {
			return nil, fmt.Errorf("invalid registry URL: missing version after @")
		}
		if versionStr != "latest" {
			var err error
			version, err = parseVersion(versionStr)
			if err != nil {
				return nil, fmt.Errorf("invalid registry URL: invalid version %q: %w", versionStr, err)
			}
		}
		nameSpec = registryURL[:idx]
	}
	
	parts := strings.Split(nameSpec, "/")
	var source, publisher, name string
	switch len(parts) {
	case 1:
		name = parts[0]
	case 2:
		publisher = parts[0]
		name = parts[1]
	case 3:
		source = parts[0]
		publisher = parts[1]
		name = parts[2]
	default:
		return nil, fmt.Errorf("invalid registry URL format: expected [source/]publisher/name[@version] or name[@version]")
	}
	
	if name == "" {
		return nil, fmt.Errorf("invalid registry URL: missing name")
	}
	
	return &URLInfo{
		resourceType: defaultResourceType,
		source:       source,
		publisher:    publisher,
		name:         name,
		version:      version,
	}, nil
}

func IsRegistryURL(input string) bool {
	return strings.HasPrefix(input, RegistryURLScheme+"://")
}

func parseVersion(versionStr string) (*semver.Version, error) {
	version, err := semver.Parse(versionStr)
	if err != nil {
		return nil, err
	}
	return &version, nil
}

func decodeArtifactNamePart(encoded string) (string, error) {
	decodedOnce, err := url.QueryUnescape(encoded)
	if err != nil {
		return "", fmt.Errorf("failed first decode: %w", err)
	}
	
	decodedTwice, err := url.QueryUnescape(decodedOnce)
	if err != nil {
		return decodedOnce, nil
	}
	
	return decodedTwice, nil
}

func encodeArtifactNamePart(name string) string {
	return url.QueryEscape(url.QueryEscape(name))
}