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
	"errors"
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

type WrongResourceTypeError struct {
	Got      string
	Expected string
}

func (e *WrongResourceTypeError) Error() string {
	return fmt.Sprintf("resource type '%s' is not valid for %s", e.Got, e.Expected)
}

type MalformedRegistryURLError struct {
	URL    string
	Reason string
}

func (e *MalformedRegistryURLError) Error() string {
	return fmt.Sprintf("malformed registry URL '%s': %s", e.URL, e.Reason)
}

type UnsupportedVersionError struct {
	Version string
}

func (e *UnsupportedVersionError) Error() string {
	return fmt.Sprintf("version '%s' is not supported; only 'latest' is supported", e.Version)
}

type MissingVersionAfterAtSignError struct {
	URL string
}

func (e *MissingVersionAfterAtSignError) Error() string {
	return "missing version after @"
}

type StructuralError struct {
	Reason string
}

func (e *StructuralError) Error() string {
	return e.Reason
}

type InvalidRegistryURLError struct {
	URL    string
	Reason string
}

func (e *InvalidRegistryURLError) Error() string {
	return fmt.Sprintf("invalid registry URL '%s': %s", e.URL, e.Reason)
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
		return nil, errors.New("invalid registry URL: missing resource type")
	}

	urlPath := strings.TrimPrefix(parsedURL.Path, "/")
	parts := strings.Split(urlPath, "/")
	if len(parts) != 3 {
		return nil, &InvalidRegistryURLError{
			URL:    registryURL,
			Reason: fmt.Sprintf("expected format: registry://%s/source/publisher/name[@version]", resourceType),
		}
	}

	source := parts[0]
	if source == "" {
		return nil, &InvalidRegistryURLError{
			URL:    registryURL,
			Reason: "missing source",
		}
	}

	publisher := parts[1]
	if publisher == "" {
		return nil, &InvalidRegistryURLError{
			URL:    registryURL,
			Reason: "missing publisher",
		}
	}

	nameAndVersion := parts[2]
	nameVersionParts := strings.Split(nameAndVersion, "@")
	if len(nameVersionParts) > 2 {
		return nil, &InvalidRegistryURLError{
			URL:    registryURL,
			Reason: fmt.Sprintf("expected format: registry://%s/source/publisher/name[@version]", resourceType),
		}
	}

	encodedName := nameVersionParts[0]
	if encodedName == "" {
		return nil, &InvalidRegistryURLError{
			URL:    registryURL,
			Reason: "missing name",
		}
	}

	name, err := decodeArtifactNamePart(encodedName)
	if err != nil {
		return nil, &InvalidRegistryURLError{
			URL:    registryURL,
			Reason: fmt.Sprintf("failed to decode name: %v", err),
		}
	}

	var version *semver.Version
	if len(nameVersionParts) == 2 {
		versionStr := nameVersionParts[1]
		if versionStr == "" {
			return nil, &InvalidRegistryURLError{
				URL:    registryURL,
				Reason: "missing version",
			}
		}

		if versionStr != "latest" {
			version, err = parseVersion(versionStr)
			if err != nil {
				return nil, &InvalidRegistryURLError{
					URL:    registryURL,
					Reason: fmt.Sprintf("invalid version %q: %v", versionStr, err),
				}
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

func ParsePartialRegistryURL(registryURL string, assumedResourceType string) (*URLInfo, error) {
	var version *semver.Version
	nameSpec := registryURL
	if idx := strings.LastIndex(registryURL, "@"); idx != -1 {
		versionStr := registryURL[idx+1:]
		if versionStr == "" {
			return nil, &MissingVersionAfterAtSignError{URL: registryURL}
		}
		if versionStr != "latest" {
			var err error
			version, err = parseVersion(versionStr)
			if err != nil {
				return nil, fmt.Errorf("invalid version %q: %w", versionStr, err)
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
		return nil, &StructuralError{Reason: "too many path segments"}
	}

	if name == "" {
		return nil, &StructuralError{Reason: "missing name"}
	}

	decodedName, err := decodeArtifactNamePart(name)
	if err != nil {
		return nil, &StructuralError{Reason: fmt.Sprintf("failed to decode name: %v", err)}
	}

	return &URLInfo{
		resourceType: assumedResourceType,
		source:       source,
		publisher:    publisher,
		name:         decodedName,
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
