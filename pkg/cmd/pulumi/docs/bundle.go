// Copyright 2024, Pulumi Corporation.
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

package docs

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// Section name constants used as map keys in sectionNav and for heading matching.
const (
	sectionModules   = "Modules"
	sectionResources = "Resources"
	sectionFunctions = "Functions"
)

// CLIDocsBundle represents the bundled CLI docs JSON for a package.
// All API docs for a package are served as a single JSON file.
type CLIDocsBundle struct {
	Version        int               `json:"version"`
	Package        string            `json:"package"`
	PackageVersion string            `json:"packageVersion"`
	Resources      map[string]string `json:"resources"`
	Functions      map[string]string `json:"functions"`
}

type bundleCacheMeta struct {
	PackageVersion string    `json:"packageVersion"`
	FetchedAt      time.Time `json:"fetchedAt"`
}

// BundleNotAvailableError is returned when a package's cli-docs.json is not available (404).
type BundleNotAvailableError struct {
	Package string
}

func (e *BundleNotAvailableError) Error() string {
	return fmt.Sprintf("CLI docs bundle not available for package: %s", e.Package)
}

const bundleCacheDir = "cli-docs-cache"
const bundleCacheTTL = 1 * time.Hour

var bundleMemCache sync.Map

// --- Path parsing ---

// ParseAPIDocsPath extracts the package name and doc key from an API docs path.
// For example:
//
//	"registry/packages/aws/api-docs/s3/bucket" → "aws", "s3/bucket", true
//	"registry/packages/random/api-docs/randomstring" → "random", "randomstring", true
//	"registry/packages/aws/api-docs" → "aws", "", true
//	"registry/packages/aws" → "", "", false
func ParseAPIDocsPath(path string) (packageName, docKey string, ok bool) {
	trimmed := strings.Trim(path, "/")
	const prefix = "registry/packages/"
	if !strings.HasPrefix(trimmed, prefix) {
		return "", "", false
	}
	after := trimmed[len(prefix):]

	if idx := strings.Index(after, "/api-docs/"); idx >= 0 {
		packageName = after[:idx]
		docKey = strings.Trim(after[idx+len("/api-docs/"):], "/")
		return packageName, docKey, true
	}

	if strings.HasSuffix(after, "/api-docs") {
		packageName = after[:len(after)-len("/api-docs")]
		return packageName, "", true
	}

	return "", "", false
}

// --- Bundle fetch and cache ---

// FetchCLIDocsBundle fetches the CLI docs bundle for a package, using in-memory
// and disk caches to avoid redundant HTTP requests.
func FetchCLIDocsBundle(baseURL, packageName string) (*CLIDocsBundle, error) {
	if cached, ok := bundleMemCache.Load(packageName); ok {
		return cached.(*CLIDocsBundle), nil
	}

	bundle, err := loadBundleFromDisk(packageName)
	if err == nil && bundle != nil {
		bundleMemCache.Store(packageName, bundle)
		return bundle, nil
	}

	fmt.Fprintf(os.Stderr, "Fetching %s API docs...\n", packageName)
	bundle, err = fetchBundleHTTP(baseURL, packageName)
	if err != nil {
		return nil, err
	}

	_ = saveBundleToDisk(packageName, bundle)
	bundleMemCache.Store(packageName, bundle)
	return bundle, nil
}

func fetchBundleHTTP(baseURL, packageName string) (*CLIDocsBundle, error) {
	url := fmt.Sprintf("%s/registry/packages/%s/api-docs/cli-docs.json",
		strings.TrimRight(baseURL, "/"), packageName)

	//nolint:gosec // URL is constructed from user-provided base URL and package name
	resp, err := bundleHTTPClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching CLI docs bundle: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		return nil, &BundleNotAvailableError{Package: packageName}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d fetching CLI docs bundle for %s", resp.StatusCode, packageName)
	}

	// Read with a progress indicator for large downloads.
	var data []byte
	total := resp.ContentLength
	if total > 1024*1024 { // Show progress for downloads > 1 MB
		data, err = readWithProgress(resp.Body, total)
	} else {
		data, err = io.ReadAll(resp.Body)
	}
	if err != nil {
		return nil, fmt.Errorf("reading CLI docs bundle response: %w", err)
	}

	var bundle CLIDocsBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("parsing CLI docs bundle: %w", err)
	}

	return &bundle, nil
}

// readWithProgress reads from r while showing a progress bar on stderr.
func readWithProgress(r io.Reader, total int64) ([]byte, error) {
	buf := make([]byte, 0, total)
	tmp := make([]byte, 256*1024) // 256 KB chunks
	var read int64
	lastPct := -1

	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
			read += int64(n)
			if total > 0 {
				pct := int(read * 100 / total)
				if pct != lastPct {
					fmt.Fprintf(os.Stderr, "\r  Downloading... %d%%", pct)
					lastPct = pct
				}
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintln(os.Stderr) // clear progress line
			return nil, err
		}
	}
	fmt.Fprintln(os.Stderr, "\r  Downloading... done.  ")
	return buf, nil
}

func bundleCachePath() (string, error) {
	home, err := workspace.GetPulumiHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, bundleCacheDir), nil
}

func loadBundleFromDisk(packageName string) (*CLIDocsBundle, error) {
	cacheDir, err := bundleCachePath()
	if err != nil {
		return nil, err
	}

	metaPath := filepath.Join(cacheDir, packageName+".meta.json")
	metaData, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}
	var meta bundleCacheMeta
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return nil, err
	}
	if time.Since(meta.FetchedAt) > bundleCacheTTL {
		return nil, fmt.Errorf("cache expired")
	}

	bundlePath := filepath.Join(cacheDir, packageName+".json")
	bundleData, err := os.ReadFile(bundlePath)
	if err != nil {
		return nil, err
	}
	var bundle CLIDocsBundle
	if err := json.Unmarshal(bundleData, &bundle); err != nil {
		return nil, err
	}
	return &bundle, nil
}

func saveBundleToDisk(packageName string, bundle *CLIDocsBundle) error {
	cacheDir, err := bundleCachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return err
	}

	bundleData, err := json.Marshal(bundle)
	if err != nil {
		return err
	}
	bundlePath := filepath.Join(cacheDir, packageName+".json")
	if err := os.WriteFile(bundlePath, bundleData, 0o600); err != nil {
		return err
	}

	meta := bundleCacheMeta{
		PackageVersion: bundle.PackageVersion,
		FetchedAt:      time.Now(),
	}
	metaData, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	metaPath := filepath.Join(cacheDir, packageName+".meta.json")
	return os.WriteFile(metaPath, metaData, 0o600)
}

// --- Bundle lookup ---

// LookupBundleDoc looks up a resource or function by key in the bundle.
// Returns the body (with title line stripped), the title, and whether the key was found.
func LookupBundleDoc(bundle *CLIDocsBundle, docKey string) (body string, title string, found bool) {
	content, ok := bundle.Resources[docKey]
	if !ok {
		content, ok = bundle.Functions[docKey]
	}
	if !ok {
		return "", "", false
	}

	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\t", "    ")

	title = extractBundleTitle(content)
	if title != "" {
		if idx := strings.Index(content, "\n"); idx >= 0 {
			content = strings.TrimLeft(content[idx+1:], "\n")
		}
	}

	return content, title, true
}

// --- Shared key classification ---

// classifiedEntry represents a single resource or function found during bundle key scanning.
type classifiedEntry struct {
	key     string // bundle key (e.g. "s3/bucket")
	title   string // extracted title (e.g. "Bucket")
	content string // raw markdown content
}

// classifiedKeys is the result of scanning bundle keys for a given module prefix.
type classifiedKeys struct {
	subModules []string         // sorted sub-module names
	resources  []classifiedEntry // sorted direct-child resources
	functions  []classifiedEntry // sorted direct-child functions
}

// classifyBundleKeys scans Resources and Functions maps, classifying each key
// as a sub-module, direct resource, or direct function relative to modulePrefix.
// This is the single source of truth for key classification — all callers
// (BundleNavItems, BundleSectionNav, RenderBundleTable) use this.
func classifyBundleKeys(bundle *CLIDocsBundle, modulePrefix string) classifiedKeys {
	subModules := make(map[string]bool)
	var resources, functions []classifiedEntry

	classify := func(keys map[string]string, isFunction bool) {
		for key, content := range keys {
			var rel string
			if modulePrefix == "" {
				if strings.Contains(key, "/") {
					subModules[strings.SplitN(key, "/", 2)[0]] = true
					continue
				}
				rel = key
			} else {
				if !strings.HasPrefix(key, modulePrefix+"/") {
					continue
				}
				rel = key[len(modulePrefix)+1:]
				if strings.Contains(rel, "/") {
					subModules[strings.SplitN(rel, "/", 2)[0]] = true
					continue
				}
			}

			title := extractBundleTitle(content)
			if title == "" {
				title = rel
			}

			entry := classifiedEntry{key: key, title: title, content: content}
			if isFunction {
				functions = append(functions, entry)
			} else {
				resources = append(resources, entry)
			}
		}
	}

	classify(bundle.Resources, false)
	classify(bundle.Functions, true)

	sort.Slice(resources, func(i, j int) bool { return resources[i].title < resources[j].title })
	sort.Slice(functions, func(i, j int) bool { return functions[i].title < functions[j].title })

	sortedMods := make([]string, 0, len(subModules))
	for mod := range subModules {
		sortedMods = append(sortedMods, mod)
	}
	sort.Strings(sortedMods)

	return classifiedKeys{
		subModules: sortedMods,
		resources:  resources,
		functions:  functions,
	}
}

// --- Navigation items ---

// BundleNavItems generates navigation items from bundle keys for a given module prefix.
func BundleNavItems(bundle *CLIDocsBundle, modulePrefix string, pkgName string) []navOption {
	basePath := fmt.Sprintf("registry/packages/%s/api-docs", pkgName)
	ck := classifyBundleKeys(bundle, modulePrefix)

	var opts []navOption

	for _, mod := range ck.subModules {
		modPath := basePath
		if modulePrefix != "" {
			modPath += "/" + modulePrefix
		}
		modPath += "/" + mod
		opts = append(opts, navOption{label: "🔗 " + mod + navDrill, path: modPath})
	}
	for _, r := range ck.resources {
		opts = append(opts, navOption{label: "🔗 " + r.title, path: basePath + "/" + r.key})
	}
	for _, f := range ck.functions {
		opts = append(opts, navOption{label: "🔗 " + f.title, path: basePath + "/" + f.key})
	}

	return opts
}

// BundleSectionNav returns per-section nav items for drilling into modules, resources, and functions.
func BundleSectionNav(bundle *CLIDocsBundle, modulePrefix string, pkgName string) map[string][]navOption {
	basePath := fmt.Sprintf("registry/packages/%s/api-docs", pkgName)
	ck := classifyBundleKeys(bundle, modulePrefix)

	result := map[string][]navOption{}

	if len(ck.subModules) > 0 {
		var modNav []navOption
		for _, mod := range ck.subModules {
			modPath := basePath
			if modulePrefix != "" {
				modPath += "/" + modulePrefix
			}
			modPath += "/" + mod
			modNav = append(modNav, navOption{label: "🔗 " + mod + navDrill, path: modPath})
		}
		result[sectionModules] = modNav
	}

	if len(ck.resources) > 0 {
		var resNav []navOption
		for _, r := range ck.resources {
			resNav = append(resNav, navOption{label: "🔗 " + r.title, path: basePath + "/" + r.key})
		}
		result[sectionResources] = resNav
	}

	if len(ck.functions) > 0 {
		var fnNav []navOption
		for _, f := range ck.functions {
			fnNav = append(fnNav, navOption{label: "🔗 " + f.title, path: basePath + "/" + f.key})
		}
		result[sectionFunctions] = fnNav
	}

	return result
}

// --- Title and description extraction ---

// extractBundleTitle extracts the title from a "# Title" first line.
func extractBundleTitle(content string) string {
	if !strings.HasPrefix(content, "# ") {
		return ""
	}
	if idx := strings.Index(content, "\n"); idx >= 0 {
		return strings.TrimPrefix(content[:idx], "# ")
	}
	return strings.TrimPrefix(content, "# ")
}

// extractBundleDescription extracts the first sentence of the description.
// Skips the title line, blank lines, and deprecation notices.
func extractBundleDescription(content string) string {
	body := content
	if strings.HasPrefix(body, "# ") {
		if idx := strings.Index(body, "\n"); idx >= 0 {
			body = body[idx+1:]
		} else {
			return ""
		}
	}
	body = strings.TrimLeft(body, "\n")

	if strings.HasPrefix(body, "> **Deprecated:") {
		if idx := strings.Index(body, "\n"); idx >= 0 {
			body = strings.TrimLeft(body[idx+1:], "\n")
		}
	}

	if body == "" {
		return ""
	}

	firstLine := body
	if idx := strings.Index(body, "\n"); idx >= 0 {
		firstLine = body[:idx]
	}
	firstLine = strings.TrimSpace(firstLine)

	if strings.HasPrefix(firstLine, "#") || strings.HasPrefix(firstLine, "<!--") {
		return ""
	}

	if idx := strings.Index(firstLine, ". "); idx >= 0 {
		firstLine = firstLine[:idx+1]
	}

	const maxLen = 80
	if len(firstLine) > maxLen {
		firstLine = firstLine[:maxLen-3] + "..."
	}

	return firstLine
}

// --- Table rendering ---

// tableEntry represents a resource, function, or module for table rendering.
type tableEntry struct {
	name string
	desc string
}

// RenderBundleTable renders a formatted table of modules, resources, and/or functions.
func RenderBundleTable(bundle *CLIDocsBundle, modulePrefix string) string {
	ck := classifyBundleKeys(bundle, modulePrefix)

	if len(ck.subModules) == 0 && len(ck.resources) == 0 && len(ck.functions) == 0 {
		return ""
	}

	modules := make([]tableEntry, len(ck.subModules))
	for i, mod := range ck.subModules {
		modules[i] = tableEntry{name: mod}
	}

	resources := make([]tableEntry, len(ck.resources))
	for i, r := range ck.resources {
		resources[i] = tableEntry{name: r.title, desc: extractBundleDescription(r.content)}
	}

	functions := make([]tableEntry, len(ck.functions))
	for i, f := range ck.functions {
		functions[i] = tableEntry{name: f.title, desc: extractBundleDescription(f.content)}
	}

	maxName := 0
	for _, list := range [][]tableEntry{resources, functions} {
		for _, e := range list {
			if len(e.name) > maxName {
				maxName = len(e.name)
			}
		}
	}

	var buf strings.Builder

	if len(modules) > 0 {
		fmt.Fprintf(&buf, "\n  %s\n", sectionModules)
		renderColumns(&buf, modules)
	}

	renderDescSection := func(label string, entries []tableEntry) {
		if len(entries) == 0 {
			return
		}
		fmt.Fprintf(&buf, "\n  %s\n\n```\n", label)
		for _, e := range entries {
			if e.desc != "" {
				fmt.Fprintf(&buf, "%-*s  %s\n", maxName, e.name, e.desc)
			} else {
				fmt.Fprintf(&buf, "%s\n", e.name)
			}
		}
		buf.WriteString("```\n")
	}

	renderDescSection(sectionResources, resources)
	renderDescSection(sectionFunctions, functions)
	buf.WriteString("\n")

	return buf.String()
}

// renderBundleSectionTable renders the table content for a single section (Modules, Resources, or Functions).
// Used by ReplaceBundleSections to avoid the string round-trip of rendering and re-parsing.
func renderBundleSectionTable(ck classifiedKeys, section string) string {
	var buf strings.Builder

	switch section {
	case sectionModules:
		if len(ck.subModules) == 0 {
			return ""
		}
		modules := make([]tableEntry, len(ck.subModules))
		for i, mod := range ck.subModules {
			modules[i] = tableEntry{name: mod}
		}
		renderColumns(&buf, modules)

	case sectionResources, sectionFunctions:
		var entries []classifiedEntry
		if section == sectionResources {
			entries = ck.resources
		} else {
			entries = ck.functions
		}
		if len(entries) == 0 {
			return ""
		}

		maxName := 0
		for _, e := range entries {
			if len(e.title) > maxName {
				maxName = len(e.title)
			}
		}

		buf.WriteString("```\n")
		for _, e := range entries {
			desc := extractBundleDescription(e.content)
			if desc != "" {
				fmt.Fprintf(&buf, "%-*s  %s\n", maxName, e.title, desc)
			} else {
				fmt.Fprintf(&buf, "%s\n", e.title)
			}
		}
		buf.WriteString("```")
	}

	return buf.String()
}

// ReplaceBundleSections replaces the Modules, Resources, and Functions sections
// in a body with formatted bundle content.
func ReplaceBundleSections(body string, bundle *CLIDocsBundle, modulePrefix string) string {
	ck := classifyBundleKeys(bundle, modulePrefix)

	for _, section := range []string{sectionModules, sectionResources, sectionFunctions} {
		heading := "## " + section
		idx := strings.Index(body, heading)
		if idx < 0 {
			continue
		}

		content := renderBundleSectionTable(ck, section)
		if content == "" {
			continue
		}

		afterHeading := idx + len(heading)
		endIdx := len(body)
		if nextH := strings.Index(body[afterHeading:], "\n## "); nextH >= 0 {
			endIdx = afterHeading + nextH
		}
		body = body[:afterHeading] + "\n\n" + content + "\n\n" + body[endIdx:]
	}

	return body
}

// --- Column rendering ---

// renderColumns writes entries in a compact multi-column layout with variable-width
// columns, similar to how `ls` formats output. Wrapped in a code fence for Glamour.
func renderColumns(buf *strings.Builder, entries []tableEntry) {
	if len(entries) == 0 {
		return
	}

	availWidth := getTerminalWidth() - 8 // Glamour margins + code block padding
	const gap = 2

	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.name
	}

	// Find optimal column count (column-first layout like `ls`).
	bestCols := 1
	for tryC := 2; tryC <= len(names); tryC++ {
		rows := (len(names) + tryC - 1) / tryC
		totalWidth := 0
		fits := true
		for c := 0; c < tryC; c++ {
			colMax := 0
			for r := 0; r < rows; r++ {
				idx := c*rows + r
				if idx < len(names) && len(names[idx]) > colMax {
					colMax = len(names[idx])
				}
			}
			if c < tryC-1 {
				totalWidth += colMax + gap
			} else {
				totalWidth += colMax
			}
			if totalWidth > availWidth {
				fits = false
				break
			}
		}
		if fits {
			bestCols = tryC
		} else {
			break
		}
	}

	rows := (len(names) + bestCols - 1) / bestCols

	colWidths := make([]int, bestCols)
	for c := 0; c < bestCols; c++ {
		for r := 0; r < rows; r++ {
			idx := c*rows + r
			if idx < len(names) && len(names[idx]) > colWidths[c] {
				colWidths[c] = len(names[idx])
			}
		}
	}

	buf.WriteString("```\n")
	for r := 0; r < rows; r++ {
		for c := 0; c < bestCols; c++ {
			idx := c*rows + r
			if idx >= len(names) {
				break
			}
			if c < bestCols-1 {
				fmt.Fprintf(buf, "%-*s", colWidths[c]+gap, names[idx])
			} else {
				buf.WriteString(names[idx])
			}
		}
		buf.WriteString("\n")
	}
	buf.WriteString("```\n")
}
