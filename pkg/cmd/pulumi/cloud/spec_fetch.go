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

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/fsutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// specCacheTTL bounds how long a cached spec may be used before ensureSpec
// refreshes it from Pulumi Cloud. Declared as var so tests can override it.
var specCacheTTL = 24 * time.Hour

// ensureSpec returns the OpenAPI spec bytes, fetching from Pulumi Cloud on
// cache miss, when the cached file is older than specCacheTTL, or when
// refresh is true. Results are written to the per-user cache directory, keyed
// by the cloud URL's hostname so self-hosted installs don't collide with
// api.pulumi.com.
//
// When refresh is false and the network fetch fails, ensureSpec falls back
// to the cached copy (if any) and writes a warning to warnW. When refresh
// is true the fetch error is returned.
func ensureSpec(ctx context.Context, warnW io.Writer, refresh bool) ([]byte, error) {
	resolved, err := ResolveContext(ctx, "")
	if err != nil {
		return nil, NewAPIError(cmdutil.ExitInternalError, ErrToolError,
			fmt.Sprintf("resolving cloud context: %v", err))
	}

	cachePath, err := specCachePath(resolved.CloudURL)
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

	data, err := fetchSpec(ctx, resolved)
	if err != nil {
		if cacheHit && !refresh {
			fmt.Fprintf(warnW,
				"warning: using cached spec (%s old) — refresh failed: %v\n", cachedAge.Truncate(time.Second), err)
			return cached, nil
		}
		return nil, err
	}
	if werr := writeCachedSpec(cachePath, data); werr != nil {
		// Cache write is best-effort; surface a warning but return the
		// freshly-fetched bytes so the current invocation still succeeds.
		fmt.Fprintf(warnW, "warning: could not cache spec at %s: %v\n", cachePath, werr)
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

// writeCachedSpec writes data to path atomically via tempfile + rename. The
// parent directory is created if it doesn't exist.
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

	mutex := fsutil.NewFileMutex(path + ".lock")
	if err := mutex.Lock(); err != nil {
		return fmt.Errorf("acquiring cache lock: %w", err)
	}
	defer func() { _ = mutex.Unlock() }()

	//nolint:forbidigo // We acquire a mutex to avoid concurrent writes
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}

// fetchSpec retrieves the OpenAPI document via Client.GetCloudAPISpec.
func fetchSpec(ctx context.Context, resolved *ResolvedContext) ([]byte, error) {
	body, err := resolved.Client.GetCloudAPISpec(ctx)
	if err != nil {
		exit := cmdutil.ExitCodeError
		if backenderr.IsAuthError(err) {
			exit = cmdutil.ExitAuthenticationError
		}
		return nil, NewAPIError(exit, ErrNetwork,
			fmt.Sprintf("fetching spec from %s: %v", resolved.CloudURL, err)).
			WithSuggestions(
				"check network connectivity",
				"run `pulumi login` if the endpoint requires authentication",
			)
	}
	return body, nil
}
