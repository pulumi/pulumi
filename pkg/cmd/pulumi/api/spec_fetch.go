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

package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/spf13/pflag"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// specEndpoint is the OpenAPI document exposed by Pulumi Cloud.
const specEndpoint = "/api/openapi/pulumi-spec.json"

// refreshSpecFlagName is the persistent flag registered on `pulumi cloud api`
// that forces re-fetching the OpenAPI spec from Pulumi Cloud, bypassing the
// per-user cache.
const refreshSpecFlagName = "refresh-spec"

// specCacheTTL bounds how long a cached spec may be used before ensureSpec
// refreshes it from Pulumi Cloud. Declared as var so tests can override it.
var specCacheTTL = 24 * time.Hour

// refreshSpecFlag reports whether --refresh-spec was supplied on cmd or any
// ancestor. The flag is persistent on the root api command so ls / describe
// inherit it.
func refreshSpecFlag(cmd interface{ Flags() *pflag.FlagSet }) bool {
	f := cmd.Flags().Lookup(refreshSpecFlagName)
	if f == nil {
		return false
	}
	b, _ := strconv.ParseBool(f.Value.String())
	return b
}

// ensureSpec returns the OpenAPI spec bytes, fetching from Pulumi Cloud on
// cache miss, when the cached file is older than specCacheTTL, or when
// refresh is true. Results are written to the per-user cache directory, keyed
// by the cloud URL's hostname so self-hosted installs don't collide with
// api.pulumi.com. If the network fetch fails and a cached copy exists, the
// cached bytes are returned with a stderr warning.
func ensureSpec(ctx context.Context, refresh bool) ([]byte, error) {
	cloudURL := httpstate.ValueOrDefaultURL(pkgWorkspace.Instance, "")
	if cloudURL == "" {
		cloudURL = "https://api.pulumi.com"
	}

	cachePath, err := specCachePath(cloudURL)
	if err != nil {
		return nil, NewAPIError(cmdutil.ExitInternalError, ErrToolError,
			fmt.Sprintf("resolving spec cache path: %v", err))
	}

	cached, cachedAge, cacheHit, rerr := readCachedSpec(cachePath)
	if rerr != nil {
		return nil, NewAPIError(cmdutil.ExitInternalError, ErrToolError,
			fmt.Sprintf("reading cached spec at %s: %v", cachePath, rerr))
	}
	if cacheHit && !refresh && cachedAge < specCacheTTL {
		return cached, nil
	}

	data, err := fetchSpec(ctx, cloudURL)
	if err != nil {
		if cacheHit {
			fmt.Fprintf(os.Stderr,
				"warning: using cached spec (%s old) — refresh failed: %v\n", cachedAge.Truncate(time.Second), err)
			return cached, nil
		}
		return nil, err
	}
	if werr := writeCachedSpec(cachePath, data); werr != nil {
		// Cache write is best-effort; surface a warning but return the
		// freshly-fetched bytes so the current invocation still succeeds.
		fmt.Fprintf(os.Stderr, "warning: could not cache spec at %s: %v\n", cachePath, werr)
	}
	return data, nil
}

// specCachePath returns the file path where the spec for cloudURL should be
// cached. Lives under `~/.pulumi/cloud-api-cache/<host>/spec.json` so each
// configured backend keeps its own copy.
func specCachePath(cloudURL string) (string, error) {
	host := hostFromURL(cloudURL)
	if host == "" {
		// Fall back to a hash when the URL doesn't have a parseable host;
		// still gives us a stable per-URL key.
		sum := sha256.Sum256([]byte(cloudURL))
		host = hex.EncodeToString(sum[:8])
	}
	return workspace.GetPulumiPath("cloud-api-cache", host, "spec.json")
}

// hostFromURL returns the hostname portion of rawURL for use as a cache-key
// path component, or "" if unparseable. Callers configure cloudURL as a
// standard HTTPS endpoint (e.g. api.pulumi.com, a self-hosted domain), so a
// bare hostname is both unambiguous and filesystem-safe.
func hostFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// readCachedSpec returns (bytes, age, true, nil) when path exists,
// (nil, 0, false, nil) when it does not, and (nil, 0, false, err) on any
// other filesystem error. Age is derived from the file mtime.
func readCachedSpec(path string) ([]byte, time.Duration, bool, error) {
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, false, nil
	}
	if err != nil {
		return nil, 0, false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, false, err
	}
	return data, time.Since(info.ModTime()), true, nil
}

// writeCachedSpec writes data to path atomically via tempfile + rename.
// The parent directory is created if it doesn't exist.
func writeCachedSpec(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating cache dir: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "spec-*.json.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}
	// atomic-enough for a cache write on unix; windows will retry on next miss
	//nolint:forbidigo // see comment above
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}

// fetchSpec GETs the OpenAPI document from cloudURL + specEndpoint. It uses
// an access token when one is available (via ResolveContext) but falls back
// to an anonymous request when the user is not logged in — the spec endpoint
// is treated as public.
func fetchSpec(ctx context.Context, cloudURL string) ([]byte, error) {
	var token string
	if resolved, err := ResolveContext(ctx, "", false); err == nil {
		token = resolved.Token
	}

	c := NewAPIClient(cloudURL, token)
	resp, err := c.Do(ctx, http.MethodGet, specEndpoint, nil, nil, "", "application/json", nil)
	if err != nil {
		return nil, NewAPIError(cmdutil.ExitCodeError, ErrNetwork,
			fmt.Sprintf("fetching spec from %s%s: %v", cloudURL, specEndpoint, err)).
			WithSuggestions(
				"check network connectivity",
				"run `pulumi login` if the endpoint requires authentication",
			)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, NewAPIError(cmdutil.ExitInternalError, ErrNetwork,
			fmt.Sprintf("reading spec response: %v", err))
	}

	if resp.StatusCode >= 400 {
		exit := cmdutil.ExitCodeError
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			exit = cmdutil.ExitAuthenticationError
		}
		return nil, NewAPIError(exit, ErrNetwork,
			fmt.Sprintf("fetching spec from %s%s: HTTP %d", cloudURL, specEndpoint, resp.StatusCode)).
			WithSuggestions(
				"run `pulumi login` if the endpoint requires authentication",
				"pass --refresh-spec once you are logged in",
			)
	}
	return body, nil
}
