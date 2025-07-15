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
	"iter"
	"testing"

	"github.com/blang/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

func TestResolvePackageFromName(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Three-part identifier tests
	t.Run("three-part/success", func(t *testing.T) {
		t.Parallel()
		mockReg := mockRegistry{
			getPackage: func(
				ctx context.Context, source, publisher, name string, version *semver.Version,
			) (apitype.PackageMetadata, error) {
				assert.Equal(t, "aws", source)
				assert.Equal(t, "pulumi", publisher)
				assert.Equal(t, "awsx", name)
				return apitype.PackageMetadata{
					Source:    source,
					Publisher: publisher,
					Name:      name,
					Version:   semver.MustParse("1.0.0"),
				}, nil
			},
		}

		pkg, err := ResolvePackageFromName(ctx, mockReg, "aws/pulumi/awsx", nil)
		require.NoError(t, err)
		assert.Equal(t, "aws", pkg.Source)
		assert.Equal(t, "pulumi", pkg.Publisher)
		assert.Equal(t, "awsx", pkg.Name)
	})

	t.Run("three-part/with-version-matching", func(t *testing.T) {
		t.Parallel()
		desiredVersion := semver.MustParse("2.1.0")
		mockReg := mockRegistry{
			getPackage: func(
				ctx context.Context, source, publisher, name string, version *semver.Version,
			) (apitype.PackageMetadata, error) {
				assert.Equal(t, &desiredVersion, version)
				return apitype.PackageMetadata{
					Source:    source,
					Publisher: publisher,
					Name:      name,
					Version:   desiredVersion,
				}, nil
			},
		}

		pkg, err := ResolvePackageFromName(ctx, mockReg, "aws/pulumi/awsx", &desiredVersion)
		require.NoError(t, err)
		assert.Equal(t, desiredVersion, pkg.Version)
	})

	t.Run("three-part/version-not-found", func(t *testing.T) {
		t.Parallel()
		mockReg := mockRegistry{
			getPackage: func(
				ctx context.Context, source, publisher, name string, version *semver.Version,
			) (apitype.PackageMetadata, error) {
				return apitype.PackageMetadata{}, ErrNotFound
			},
		}

		_, err := ResolvePackageFromName(ctx, mockReg, "aws/pulumi/awsx", &semver.Version{Major: 999})
		assert.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotFound))
	})

	t.Run("three-part/package-not-found", func(t *testing.T) {
		t.Parallel()
		mockReg := mockRegistry{
			getPackage: func(
				ctx context.Context, source, publisher, name string, version *semver.Version,
			) (apitype.PackageMetadata, error) {
				return apitype.PackageMetadata{}, ErrNotFound
			},
		}

		_, err := ResolvePackageFromName(ctx, mockReg, "nonexistent/pub/pkg", nil)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotFound))
	})

	t.Run("three-part/other-errors", func(t *testing.T) {
		t.Parallel()
		testCases := []struct {
			name string
			err  error
		}{
			{"unauthorized", ErrUnauthorized},
			{"forbidden", ErrForbidden},
			{"network-error", errors.New("network error")},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				mockReg := mockRegistry{
					getPackage: func(
						ctx context.Context, source, publisher, name string, version *semver.Version,
					) (apitype.PackageMetadata, error) {
						return apitype.PackageMetadata{}, tc.err
					},
				}

				_, err := ResolvePackageFromName(ctx, mockReg, "aws/pulumi/awsx", nil)
				assert.Error(t, err)
				assert.True(t, errors.Is(err, tc.err))
			})
		}
	})

	// Two-part identifier tests
	t.Run("two-part/private-precedence", func(t *testing.T) {
		t.Parallel()
		mockReg := mockRegistry{
			getPackage: func(
				ctx context.Context, source, publisher, name string, version *semver.Version,
			) (apitype.PackageMetadata, error) {
				if source == "private" {
					return apitype.PackageMetadata{
						Source:    "private",
						Publisher: publisher,
						Name:      name,
					}, nil
				}
				// Should not reach pulumi since private succeeded
				t.Fatal("Should not check pulumi when private succeeds")
				return apitype.PackageMetadata{}, nil
			},
		}

		pkg, err := ResolvePackageFromName(ctx, mockReg, "myorg/mypkg", nil)
		require.NoError(t, err)
		assert.Equal(t, "private", pkg.Source)
		assert.Equal(t, "myorg", pkg.Publisher)
		assert.Equal(t, "mypkg", pkg.Name)
	})

	t.Run("two-part/fallback-to-pulumi", func(t *testing.T) {
		t.Parallel()
		mockReg := mockRegistry{
			getPackage: func(
				ctx context.Context, source, publisher, name string, version *semver.Version,
			) (apitype.PackageMetadata, error) {
				if source == "private" {
					return apitype.PackageMetadata{}, ErrNotFound
				}
				if source == "pulumi" {
					return apitype.PackageMetadata{
						Source:    "pulumi",
						Publisher: publisher,
						Name:      name,
					}, nil
				}
				return apitype.PackageMetadata{}, ErrNotFound
			},
		}

		pkg, err := ResolvePackageFromName(ctx, mockReg, "pulumi/aws", nil)
		require.NoError(t, err)
		assert.Equal(t, "pulumi", pkg.Source)
		assert.Equal(t, "pulumi", pkg.Publisher)
		assert.Equal(t, "aws", pkg.Name)
	})

	t.Run("two-part/both-not-found", func(t *testing.T) {
		t.Parallel()
		mockReg := mockRegistry{
			getPackage: func(
				ctx context.Context, source, publisher, name string, version *semver.Version,
			) (apitype.PackageMetadata, error) {
				return apitype.PackageMetadata{}, ErrNotFound
			},
		}

		_, err := ResolvePackageFromName(ctx, mockReg, "nonexistent/pkg", nil)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotFound))
		assert.Contains(t, err.Error(), "could not resolve nonexistent/pkg")
	})

	t.Run("two-part/private-unauthorized-fallback", func(t *testing.T) {
		t.Parallel()
		testCases := []error{ErrUnauthorized, ErrForbidden}

		for _, privateErr := range testCases {
			t.Run(privateErr.Error(), func(t *testing.T) {
				t.Parallel()
				mockReg := mockRegistry{
					getPackage: func(
						ctx context.Context, source, publisher, name string, version *semver.Version,
					) (apitype.PackageMetadata, error) {
						if source == "private" {
							return apitype.PackageMetadata{}, privateErr
						}
						if source == "pulumi" {
							return apitype.PackageMetadata{
								Source:    "pulumi",
								Publisher: publisher,
								Name:      name,
							}, nil
						}
						return apitype.PackageMetadata{}, ErrNotFound
					},
				}

				pkg, err := ResolvePackageFromName(ctx, mockReg, "pulumi/aws", nil)
				require.NoError(t, err)
				assert.Equal(t, "pulumi", pkg.Source)
			})
		}
	})

	t.Run("two-part/private-other-error", func(t *testing.T) {
		t.Parallel()
		networkErr := errors.New("network error")
		mockReg := mockRegistry{
			getPackage: func(
				ctx context.Context, source, publisher, name string, version *semver.Version,
			) (apitype.PackageMetadata, error) {
				if source == "private" {
					return apitype.PackageMetadata{}, networkErr
				}
				t.Fatal("Should not reach pulumi when private returns other error")
				return apitype.PackageMetadata{}, nil
			},
		}

		_, err := ResolvePackageFromName(ctx, mockReg, "myorg/mypkg", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unable to check on private/myorg/mypkg")
		assert.True(t, errors.Is(err, networkErr))
	})

	t.Run("two-part/pulumi-other-error", func(t *testing.T) {
		t.Parallel()
		networkErr := errors.New("network error")
		mockReg := mockRegistry{
			getPackage: func(
				ctx context.Context, source, publisher, name string, version *semver.Version,
			) (apitype.PackageMetadata, error) {
				if source == "private" {
					return apitype.PackageMetadata{}, ErrNotFound
				}
				if source == "pulumi" {
					return apitype.PackageMetadata{}, networkErr
				}
				return apitype.PackageMetadata{}, ErrNotFound
			},
		}

		_, err := ResolvePackageFromName(ctx, mockReg, "pulumi/aws", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unable to check on pulumi/pulumi/aws")
		assert.True(t, errors.Is(err, networkErr))
	})

	t.Run("two-part/with-version", func(t *testing.T) {
		t.Parallel()
		desiredVersion := semver.MustParse("1.5.0")
		mockReg := mockRegistry{
			getPackage: func(
				ctx context.Context, source, publisher, name string, version *semver.Version,
			) (apitype.PackageMetadata, error) {
				if source == "private" {
					return apitype.PackageMetadata{}, ErrNotFound
				}
				if source == "pulumi" {
					assert.Equal(t, &desiredVersion, version)
					return apitype.PackageMetadata{
						Source:    "pulumi",
						Publisher: publisher,
						Name:      name,
						Version:   desiredVersion,
					}, nil
				}
				return apitype.PackageMetadata{}, ErrNotFound
			},
		}

		pkg, err := ResolvePackageFromName(ctx, mockReg, "pulumi/aws", &desiredVersion)
		require.NoError(t, err)
		assert.Equal(t, desiredVersion, pkg.Version)
	})

	// Single identifier tests
	t.Run("single/private-precedence", func(t *testing.T) {
		t.Parallel()
		mockReg := mockRegistry{
			listPackages: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
				assert.Equal(t, "aws", *name)
				return func(yield func(apitype.PackageMetadata, error) bool) {
					// Return private match first
					if !yield(apitype.PackageMetadata{
						Source:    "private",
						Publisher: "myorg",
						Name:      "aws",
						Version:   semver.MustParse("1.0.0"),
					}, nil) {
						return
					}
					// Also return pulumi match, but private should take precedence
					yield(apitype.PackageMetadata{
						Source:    "pulumi",
						Publisher: "pulumi",
						Name:      "aws",
						Version:   semver.MustParse("2.0.0"),
					}, nil)
				}
			},
		}

		pkg, err := ResolvePackageFromName(ctx, mockReg, "aws", nil)
		require.NoError(t, err)
		assert.Equal(t, "private", pkg.Source)
		assert.Equal(t, "myorg", pkg.Publisher)
		assert.Equal(t, "aws", pkg.Name)
	})

	t.Run("single/pulumi-match", func(t *testing.T) {
		t.Parallel()
		mockReg := mockRegistry{
			listPackages: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
				return func(yield func(apitype.PackageMetadata, error) bool) {
					// No private match, only pulumi/pulumi
					yield(apitype.PackageMetadata{
						Source:    "pulumi",
						Publisher: "pulumi",
						Name:      "aws",
						Version:   semver.MustParse("2.0.0"),
					}, nil)
				}
			},
		}

		pkg, err := ResolvePackageFromName(ctx, mockReg, "aws", nil)
		require.NoError(t, err)
		assert.Equal(t, "pulumi", pkg.Source)
		assert.Equal(t, "pulumi", pkg.Publisher)
		assert.Equal(t, "aws", pkg.Name)
	})

	t.Run("single/no-matches-with-suggestions", func(t *testing.T) {
		t.Parallel()
		mockReg := mockRegistry{
			listPackages: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
				return func(yield func(apitype.PackageMetadata, error) bool) {
					// Return some suggestions but no exact matches
					if !yield(apitype.PackageMetadata{
						Source:    "community",
						Publisher: "thirdparty",
						Name:      "aws",
						Version:   semver.MustParse("1.0.0"),
					}, nil) {
						return
					}
					yield(apitype.PackageMetadata{
						Source:    "github",
						Publisher: "someuser",
						Name:      "aws",
						Version:   semver.MustParse("0.5.0"),
					}, nil)
				}
			},
		}

		_, err := ResolvePackageFromName(ctx, mockReg, "aws", nil)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotFound))
		assert.Contains(t, err.Error(), "aws does not match a registry package")

		// Test suggestions are available
		suggestions := GetSuggestedPackages(err)
		assert.Len(t, suggestions, 2)
		assert.Equal(t, "community", suggestions[0].Source)
		assert.Equal(t, "github", suggestions[1].Source)
	})

	t.Run("single/version-matching", func(t *testing.T) {
		t.Parallel()
		desiredVersion := semver.MustParse("1.5.0")
		mockReg := mockRegistry{
			listPackages: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
				return func(yield func(apitype.PackageMetadata, error) bool) {
					yield(apitype.PackageMetadata{
						Source:    "pulumi",
						Publisher: "pulumi",
						Name:      "aws",
						Version:   semver.MustParse("2.0.0"), // Different version
					}, nil)
				}
			},
			getPackage: func(
				ctx context.Context, source, publisher, name string, version *semver.Version,
			) (apitype.PackageMetadata, error) {
				assert.Equal(t, "pulumi", source)
				assert.Equal(t, "pulumi", publisher)
				assert.Equal(t, "aws", name)
				assert.Equal(t, &desiredVersion, version)
				return apitype.PackageMetadata{
					Source:    source,
					Publisher: publisher,
					Name:      name,
					Version:   desiredVersion,
				}, nil
			},
		}

		pkg, err := ResolvePackageFromName(ctx, mockReg, "aws", &desiredVersion)
		require.NoError(t, err)
		assert.Equal(t, desiredVersion, pkg.Version)
	})

	t.Run("single/version-mismatch", func(t *testing.T) {
		t.Parallel()
		desiredVersion := semver.MustParse("999.0.0")
		mockReg := mockRegistry{
			listPackages: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
				return func(yield func(apitype.PackageMetadata, error) bool) {
					yield(apitype.PackageMetadata{
						Source:    "pulumi",
						Publisher: "pulumi",
						Name:      "aws",
						Version:   semver.MustParse("2.0.0"),
					}, nil)
				}
			},
			getPackage: func(
				ctx context.Context, source, publisher, name string, version *semver.Version,
			) (apitype.PackageMetadata, error) {
				return apitype.PackageMetadata{}, ErrNotFound
			},
		}

		_, err := ResolvePackageFromName(ctx, mockReg, "aws", &desiredVersion)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "pulumi/pulumi/aws exists, but version 999.0.0 was not found")

		// Should have suggestions
		suggestions := GetSuggestedPackages(err)
		assert.Len(t, suggestions, 1)
		assert.Equal(t, semver.MustParse("2.0.0"), suggestions[0].Version)
	})

	t.Run("single/search-error", func(t *testing.T) {
		t.Parallel()
		searchErr := errors.New("search failed")
		mockReg := mockRegistry{
			listPackages: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
				return func(yield func(apitype.PackageMetadata, error) bool) {
					yield(apitype.PackageMetadata{}, searchErr)
				}
			},
		}

		_, err := ResolvePackageFromName(ctx, mockReg, "aws", nil)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, searchErr))
	})

	t.Run("single/multiple-suggestions", func(t *testing.T) {
		t.Parallel()
		mockReg := mockRegistry{
			listPackages: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
				return func(yield func(apitype.PackageMetadata, error) bool) {
					// Return multiple non-matching packages
					if !yield(apitype.PackageMetadata{
						Source:    "community",
						Publisher: "org1",
						Name:      "aws",
					}, nil) {
						return
					}
					if !yield(apitype.PackageMetadata{
						Source:    "github",
						Publisher: "org2",
						Name:      "aws",
					}, nil) {
						return
					}
					yield(apitype.PackageMetadata{
						Source:    "custom",
						Publisher: "org3",
						Name:      "aws",
					}, nil)
				}
			},
		}

		_, err := ResolvePackageFromName(ctx, mockReg, "aws", nil)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotFound))

		suggestions := GetSuggestedPackages(err)
		assert.Len(t, suggestions, 3)
		assert.Equal(t, "community", suggestions[0].Source)
		assert.Equal(t, "github", suggestions[1].Source)
		assert.Equal(t, "custom", suggestions[2].Source)
	})

	t.Run("single/ambiguous-resolution", func(t *testing.T) {
		t.Parallel()
		mockReg := mockRegistry{
			listPackages: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
				return func(yield func(apitype.PackageMetadata, error) bool) {
					// Return multiple non-matching packages
					if !yield(apitype.PackageMetadata{
						Source:    "private",
						Publisher: "org1",
						Name:      "aws",
					}, nil) {
						return
					}
					if !yield(apitype.PackageMetadata{
						Source:    "private",
						Publisher: "org2",
						Name:      "aws",
					}, nil) {
						return
					}
				}
			},
		}

		_, err := ResolvePackageFromName(ctx, mockReg, "aws", nil)
		assert.ErrorContains(t, err, `"aws" is ambiguous, it matches both private/org1/aws and private/org2/aws`)
		assert.Equal(t, []apitype.PackageMetadata{
			{
				Source:    "private",
				Publisher: "org1",
				Name:      "aws",
			},
			{
				Source:    "private",
				Publisher: "org2",
				Name:      "aws",
			},
		}, GetSuggestedPackages(err))
	})

	t.Run("single/very-ambiguous-resolution", func(t *testing.T) {
		t.Parallel()
		mockReg := mockRegistry{
			listPackages: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
				return func(yield func(apitype.PackageMetadata, error) bool) {
					// Return multiple non-matching packages
					if !yield(apitype.PackageMetadata{
						Source:    "private",
						Publisher: "org1",
						Name:      "aws",
					}, nil) {
						return
					}
					if !yield(apitype.PackageMetadata{
						Source:    "private",
						Publisher: "org2",
						Name:      "aws",
					}, nil) {
						return
					}

					if !yield(apitype.PackageMetadata{
						Source:    "private",
						Publisher: "org3",
						Name:      "aws",
					}, nil) {
						return
					}
				}
			},
		}

		_, err := ResolvePackageFromName(ctx, mockReg, "aws", nil)
		assert.ErrorContains(t, err, `"aws" is ambiguous, it matches both private/org1/aws and 2 other package`)
		assert.Equal(t, []apitype.PackageMetadata{
			{
				Source:    "private",
				Publisher: "org1",
				Name:      "aws",
			},
			{
				Source:    "private",
				Publisher: "org2",
				Name:      "aws",
			},
			{
				Source:    "private",
				Publisher: "org3",
				Name:      "aws",
			},
		}, GetSuggestedPackages(err))
	})

	// Invalid identifier tests
	t.Run("invalid/empty-string", func(t *testing.T) {
		t.Parallel()
		mockReg := mockRegistry{
			listPackages: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
				// Empty string gets treated as single identifier, but no matches will be found
				assert.Equal(t, "", *name)
				return func(yield func(apitype.PackageMetadata, error) bool) {
					// Return no matches
				}
			},
		}

		_, err := ResolvePackageFromName(ctx, mockReg, "", nil)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, ErrNotFound))
		assert.Contains(t, err.Error(), " does not match a registry package")
	})

	t.Run("invalid/too-many-parts", func(t *testing.T) {
		t.Parallel()
		mockReg := mockRegistry{}

		testCases := []string{
			"a/b/c/d",
			"source/publisher/name/extra/parts",
		}

		for _, tc := range testCases {
			t.Run(tc, func(t *testing.T) {
				t.Parallel()
				_, err := ResolvePackageFromName(ctx, mockReg, tc, nil)
				assert.Error(t, err)
				var invalidErr InvalidIdentifierError
				assert.True(t, errors.As(err, &invalidErr))
				assert.Contains(t, err.Error(), tc)
			})
		}
	})

	// Nil version handling tests
	t.Run("nil-version/three-part", func(t *testing.T) {
		t.Parallel()
		mockReg := mockRegistry{
			getPackage: func(
				ctx context.Context, source, publisher, name string, version *semver.Version,
			) (apitype.PackageMetadata, error) {
				assert.Nil(t, version)
				return apitype.PackageMetadata{
					Source:    source,
					Publisher: publisher,
					Name:      name,
					Version:   semver.MustParse("1.0.0"), // Latest version
				}, nil
			},
		}

		pkg, err := ResolvePackageFromName(ctx, mockReg, "aws/pulumi/awsx", nil)
		require.NoError(t, err)
		assert.Equal(t, semver.MustParse("1.0.0"), pkg.Version)
	})

	t.Run("nil-version/single-part", func(t *testing.T) {
		t.Parallel()
		mockReg := mockRegistry{
			listPackages: func(ctx context.Context, name *string) iter.Seq2[apitype.PackageMetadata, error] {
				return func(yield func(apitype.PackageMetadata, error) bool) {
					yield(apitype.PackageMetadata{
						Source:    "pulumi",
						Publisher: "pulumi",
						Name:      "aws",
						Version:   semver.MustParse("2.0.0"), // Should be returned as-is
					}, nil)
				}
			},
		}

		pkg, err := ResolvePackageFromName(ctx, mockReg, "aws", nil)
		require.NoError(t, err)
		assert.Equal(t, semver.MustParse("2.0.0"), pkg.Version)
	})
}
