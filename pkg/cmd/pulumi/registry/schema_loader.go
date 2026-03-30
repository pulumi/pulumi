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

package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/blang/semver"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/packages"
	pkgSchema "github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	commonregistry "github.com/pulumi/pulumi/sdk/v3/go/common/registry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/plugin"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// loadSchemaForPackage resolves a package name via the registry, then loads its schema.
// It attempts the fast path (HTTP GET of SchemaURL) first, falling back to plugin-based loading.
func loadSchemaForPackage(
	ctx context.Context,
	reg commonregistry.Registry,
	packageName string,
	version *semver.Version,
) (*pkgSchema.PackageSpec, error) {
	// Step 1: Resolve the package via the registry API.
	meta, err := commonregistry.ResolvePackageFromName(ctx, reg, packageName, version)
	if err != nil {
		return nil, fmt.Errorf("could not resolve package %q: %w", packageName, err)
	}

	// Step 2: Try the fast path — HTTP GET the SchemaURL.
	if meta.SchemaURL != "" {
		spec, fetchErr := fetchSchemaFromURL(ctx, meta.SchemaURL)
		if fetchErr == nil {
			return spec, nil
		}
		// Log a warning and fall back to plugin-based loading.
		cmdutil.Diag().Warningf(
			diag.RawMessage("", fmt.Sprintf("could not fetch schema from URL, falling back to plugin: %v", fetchErr)))
	}

	// Step 3: Fallback — use the plugin-based schema loading.
	return loadSchemaViaPlugin(ctx, packageName, reg, version)
}

// fetchSchemaFromURL performs an HTTP GET on the schema URL and unmarshals the result.
func fetchSchemaFromURL(ctx context.Context, schemaURL string) (*pkgSchema.PackageSpec, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, schemaURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "pulumi-cli")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching schema: %w", err)
	}
	defer contract.IgnoreClose(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching schema: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading schema body: %w", err)
	}

	var spec pkgSchema.PackageSpec
	if err := json.Unmarshal(body, &spec); err != nil {
		return nil, fmt.Errorf("parsing schema JSON: %w", err)
	}

	return &spec, nil
}

// loadSchemaViaPlugin falls back to the plugin-based schema loading path.
func loadSchemaViaPlugin(
	ctx context.Context,
	packageName string,
	reg commonregistry.Registry,
	version *semver.Version,
) (*pkgSchema.PackageSpec, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	sink := cmdutil.Diag()
	pctx, err := plugin.NewContext(ctx, sink, sink, nil, nil, wd, nil, false,
		nil, pkgSchema.NewLoaderServerFromHost)
	if err != nil {
		return nil, err
	}
	defer contract.IgnoreClose(pctx)

	source := packageName
	if version != nil {
		source = packageName + "@" + version.String()
	}
	spec, _, err := packages.SchemaFromSchemaSource(
		pctx, source, &plugin.ParameterizeArgs{}, reg, env.Global(), 0,
	)
	return spec, err
}
