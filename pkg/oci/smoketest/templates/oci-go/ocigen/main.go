// oci-required-packages generator for OCI Go program images. It discovers the Pulumi
// provider plugins the program depends on and writes a manifest
// {"plugins":[<PulumiPluginJSON>...]} to a well-known path. Each pulumi-plugin.json IS
// a PulumiPluginJSON (the standalone metadata file Go/Python/.NET Pulumi SDKs carry —
// unlike Node, which embeds the same content in package.json), so entries aggregate
// verbatim.
//
// This is template-owned and stdlib-only, run in the Go build stage via `go run`.
//
// It mirrors pulumi-language-go's discovery: `go list -m -json all` yields the module
// closure and each module's on-disk directory, and a pulumi-plugin.json is looked for
// at a small set of paths WITHIN each module dir. That shallowness is load-bearing — a
// naive recursive walk of the module cache also picks up pulumi-plugin.json TEST
// FIXTURES buried in dependencies (notably the pulumi SDK's own testdata), reporting
// providers that do not exist. Scoping to module dirs + candidate paths avoids that.
//
// Best-effort: lazy discovery at RegisterResource time stays authoritative.
//
// Usage: go run ./ocigen [AUTO|<module-dir>] [output-path]
//
//	AUTO (default): discover modules via `go list -m -json all`.
//	<module-dir>:   treat the given directory as a single module dir (used by tests).
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

// pulumiPlugin mirrors plugin.PulumiPluginJSON. RawMessage passes parameterization
// through verbatim without this tool needing to model its shape.
type pulumiPlugin struct {
	Resource                  bool            `json:"resource"`
	Name                      string          `json:"name,omitempty"`
	Version                   string          `json:"version,omitempty"`
	Server                    string          `json:"server,omitempty"`
	Parameterization          json.RawMessage `json:"parameterization,omitempty"`
	ExtensionParameterization json.RawMessage `json:"extensionParameterization,omitempty"`
}

// moduleDirs returns the on-disk directories of the modules to scan. For AUTO it asks
// the go tool for the program's module closure; otherwise the argument is treated as a
// single module dir (the testable path).
func moduleDirs(root string) []string {
	if root != "AUTO" {
		return []string{root}
	}
	out, err := exec.Command("go", "list", "-m", "-json", "all").Output()
	if err != nil {
		// Best-effort: e.g. a non-module build. Nothing to report rather than fail.
		return nil
	}
	var dirs []string
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var m struct{ Dir string }
		if err := dec.Decode(&m); err != nil {
			break
		}
		if m.Dir != "" {
			dirs = append(dirs, m.Dir)
		}
	}
	return dirs
}

// candidatePaths lists where a pulumi-plugin.json may live within a module dir,
// matching pulumi-language-go's lookup. Deliberately shallow, so deep test fixtures
// inside dependencies are not mistaken for real providers.
func candidatePaths(dir string) []string {
	paths := []string{
		filepath.Join(dir, "pulumi-plugin.json"),
		filepath.Join(dir, "go", "pulumi-plugin.json"),
	}
	for _, glob := range []string{
		filepath.Join(dir, "go", "*", "pulumi-plugin.json"),
		filepath.Join(dir, "*", "pulumi-plugin.json"),
	} {
		if matches, err := filepath.Glob(glob); err == nil {
			paths = append(paths, matches...)
		}
	}
	return paths
}

func main() {
	root := "AUTO"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	out := ""
	if len(os.Args) > 2 {
		out = os.Args[2]
	}

	// Dedup by name@version: a module dir can yield the same plugin from >1 candidate.
	found := map[string]pulumiPlugin{}
	for _, dir := range moduleDirs(root) {
		for _, path := range candidatePaths(dir) {
			b, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			var p pulumiPlugin
			if json.Unmarshal(b, &p) != nil {
				continue
			}
			// Only resource plugins with a resolvable name; omit-over-wrong otherwise.
			if !p.Resource || p.Name == "" {
				continue
			}
			found[p.Name+"@"+p.Version] = p
		}
	}

	plugins := make([]pulumiPlugin, 0, len(found))
	for _, p := range found {
		plugins = append(plugins, p)
	}
	sort.Slice(plugins, func(i, j int) bool {
		if plugins[i].Name != plugins[j].Name {
			return plugins[i].Name < plugins[j].Name
		}
		return plugins[i].Version < plugins[j].Version
	})

	manifest, err := json.MarshalIndent(map[string]any{"plugins": plugins}, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "oci-required-packages: %v\n", err)
		os.Exit(1)
	}
	manifest = append(manifest, '\n')

	if out == "" {
		os.Stdout.Write(manifest)
		return
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "oci-required-packages: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(out, manifest, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "oci-required-packages: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "oci-required-packages: wrote %d plugin(s) to %s\n", len(plugins), out)
}
