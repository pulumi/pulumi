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
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

// ResolvePackageFromName resolves a registry package from user input. User input may be in the following forms:
//
//	<source>/<publisher>/<name> -> [<source>/<publisher>/<name>]
//
//	<publisher>/<name>          -> [private/<publisher>/<name>, pulumi/<publisher>/<name>]
//
//	<name>                      -> [private/*/<name>, pulumi/*/<name>]
//
// The returned error will include [NotFoundError] if and only if ResolvePackageFromName
// has determined that registry does not contain a matching package.
//
// If ResolvePackageFromName could not parse name into a fragment kind, it will return an
// error that includes [InvalidIdentifierError].
//
// When ResolvePackageFromName is unable to find a corresponding package (and
// [NotFoundError] is returned), a list of similar packages *may* be available from
// calling [GetSuggestedPackages] on the returned error.
func ResolvePackageFromName(
	ctx context.Context, registry Registry, name string, version *semver.Version,
) (apitype.PackageMetadata, error) {
	parts := strings.Split(name, "/")
	switch len(parts) {
	case 3:
		p, err := registry.GetPackage(ctx, parts[0], parts[1], parts[2], version)
		return p, err
	case 2:
		// First check on "private"
		pkg, err := registry.GetPackage(ctx, "private", parts[0], parts[1], version)
		if err == nil {
			return pkg, nil
		} else if !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrUnauthorized) && !errors.Is(err, ErrForbidden) {
			return apitype.PackageMetadata{}, fmt.Errorf("unable to check on private/%s: %w", name, err)
		}

		// Then check on "pulumi"
		pkg, err = registry.GetPackage(ctx, "pulumi", parts[0], parts[1], version)
		if err == nil {
			return pkg, nil
		} else if !errors.Is(err, ErrNotFound) {
			return apitype.PackageMetadata{}, fmt.Errorf("unable to check on pulumi/%s: %w", name, err)
		}

		// Both "private" and "pulumi" didn't exist, so we have successfully resolved to NotFound.
		return apitype.PackageMetadata{}, fmt.Errorf("could not resolve %s: %w", name, ErrNotFound)
	case 1:
		var pulumiPackageMetadata *apitype.PackageMetadata
		var privatePackageMetadata []apitype.PackageMetadata
		var suggested []apitype.PackageMetadata
		for meta, err := range registry.ListPackages(ctx, &name) {
			if err != nil {
				return apitype.PackageMetadata{}, err
			}

			if meta.Source == "private" {
				privatePackageMetadata = append(privatePackageMetadata, meta)
			} else if meta.Source == "pulumi" && meta.Publisher == "pulumi" {
				// We don't break here, since we might still have a dominant source: the key in "private".
				pulumiPackageMetadata = &meta
			} else {
				suggested = append(suggested, meta)
			}
		}

		if len(privatePackageMetadata) > 1 {
			err := fmt.Errorf("%q is ambiguous, it matches both %s/%s/%s and %d other packages",
				name,
				privatePackageMetadata[0].Source, privatePackageMetadata[0].Publisher, privatePackageMetadata[0].Name,
				len(privatePackageMetadata)-1,
			)
			if len(privatePackageMetadata) == 2 {
				err = fmt.Errorf("%q is ambiguous, it matches both %s/%s/%s and %s/%s/%s",
					name,
					privatePackageMetadata[0].Source, privatePackageMetadata[0].Publisher, privatePackageMetadata[0].Name,
					privatePackageMetadata[1].Source, privatePackageMetadata[1].Publisher, privatePackageMetadata[1].Name,
				)
			}

			return apitype.PackageMetadata{},
				errorSuggestedPackages{
					err:      err,
					packages: privatePackageMetadata,
				}
		}

		// Search by name returns the latest package versions with the correct name.
		//
		// If a version was specified, we need to fetch that specific version with GetPackage.
		applyVersion := func(meta apitype.PackageMetadata) (apitype.PackageMetadata, error) {
			if version == nil || meta.Version.Equals(*version) {
				return meta, nil
			}
			m, err := registry.GetPackage(ctx, meta.Source, meta.Publisher, meta.Name, version)
			if err == nil {
				return m, nil
			}
			if errors.Is(err, ErrNotFound) {
				return apitype.PackageMetadata{}, versionMismatchError{
					found: meta, desired: *version,
					err: errorSuggestedPackages{
						packages: []apitype.PackageMetadata{meta},
						err:      ErrNotFound,
					},
				}
			}
			return apitype.PackageMetadata{}, err
		}

		if len(privatePackageMetadata) == 1 {
			return applyVersion(privatePackageMetadata[0])
		}
		if pulumiPackageMetadata != nil {
			return applyVersion(*pulumiPackageMetadata)
		}
		var versionStr string
		if version != nil {
			versionStr = "@" + version.String()
		}
		return apitype.PackageMetadata{}, fmt.Errorf(
			"%w: %s%s does not match a registry package", errorSuggestedPackages{
				packages: suggested,
				err:      ErrNotFound,
			}, name, versionStr)
	default:
		return apitype.PackageMetadata{}, InvalidIdentifierError{name}
	}
}

func GetSuggestedPackages(err error) []apitype.PackageMetadata {
	var suggestions errorSuggestedPackages
	errors.As(err, &suggestions)
	return suggestions.packages
}

type errorSuggestedPackages struct {
	err      error
	packages []apitype.PackageMetadata
}

func (err errorSuggestedPackages) Error() string {
	return err.err.Error()
}

func (err errorSuggestedPackages) Unwrap() error {
	return err.err
}

// An error indicating that the registry contains the correct "package", but not the
// correct "package version".
type versionMismatchError struct {
	found   apitype.PackageMetadata
	desired semver.Version
	err     error
}

func (err versionMismatchError) Error() string {
	return fmt.Sprintf("%s/%s/%s exists, but version %s was not found",
		err.found.Source, err.found.Publisher, err.found.Name, err.desired,
	)
}

func (err versionMismatchError) Unwrap() error { return err.err }

type InvalidIdentifierError struct{ given string }

func (err InvalidIdentifierError) Error() string {
	if err.given == "" {
		return "invalid identifier"
	}
	return fmt.Sprintf("invalid identifier: found %q", err.given)
}
