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

package docsrender

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pgavlin/goldmark/ast"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	SectionModules   = "Modules"
	SectionResources = "Resources"
	SectionFunctions = "Functions"
)

// CLIDocEntry represents a single resource or function in the CLI docs bundle.
type CLIDocEntry struct {
	Title              string `json:"title"`
	Description        string `json:"description"`
	Content            string `json:"content"`
	Deprecated         bool   `json:"deprecated,omitempty"`
	DeprecationMessage string `json:"deprecationMessage,omitempty"`
}

// CLIDocsBundle is the bundled CLI docs JSON for a package.
type CLIDocsBundle struct {
	Package        string                 `json:"package"`
	PackageVersion string                 `json:"packageVersion"`
	Overview       string                 `json:"overview,omitempty"`
	Resources      map[string]CLIDocEntry `json:"-"`
	Functions      map[string]CLIDocEntry `json:"-"`
}

// cliBundleRaw is used for initial JSON unmarshaling before entries are normalized.
type cliBundleRaw struct {
	Package        string                     `json:"package"`
	PackageVersion string                     `json:"packageVersion"`
	Overview       string                     `json:"overview,omitempty"`
	Resources      map[string]json.RawMessage `json:"resources"`
	Functions      map[string]json.RawMessage `json:"functions"`
}

// UnmarshalJSON handles both new (CLIDocEntry) and legacy (string) bundle formats.
func (b *CLIDocsBundle) UnmarshalJSON(data []byte) error {
	var raw cliBundleRaw
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	b.Package = raw.Package
	b.PackageVersion = raw.PackageVersion
	b.Overview = raw.Overview
	b.Resources = unmarshalEntries(raw.Resources)
	b.Functions = unmarshalEntries(raw.Functions)
	return nil
}

func unmarshalEntries(raw map[string]json.RawMessage) map[string]CLIDocEntry {
	result := make(map[string]CLIDocEntry, len(raw))
	for key, data := range raw {
		var entry CLIDocEntry
		if err := json.Unmarshal(data, &entry); err == nil && entry.Title != "" {
			result[key] = entry
			continue
		}
		var content string
		if err := json.Unmarshal(data, &content); err == nil {
			content = normalizeWhitespace(content)
			title, body := extractLegacyTitle(content)
			result[key] = CLIDocEntry{
				Title:   title,
				Content: body,
			}
		}
	}
	return result
}

// MarshalJSON serializes the bundle in the new structured format.
func (b CLIDocsBundle) MarshalJSON() ([]byte, error) {
	type alias struct {
		Package        string                 `json:"package"`
		PackageVersion string                 `json:"packageVersion"`
		Overview       string                 `json:"overview,omitempty"`
		Resources      map[string]CLIDocEntry `json:"resources"`
		Functions      map[string]CLIDocEntry `json:"functions"`
	}
	return json.Marshal(alias(b))
}

func extractLegacyTitle(content string) (title, body string) {
	if !strings.HasPrefix(content, "# ") {
		return "", content
	}
	if idx := strings.Index(content, "\n"); idx >= 0 {
		return content[2:idx], strings.TrimLeft(content[idx+1:], "\n")
	}
	return content[2:], ""
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
	return "CLI docs bundle not available for package: " + e.Package
}

const (
	bundleCacheDir = "cli-docs-cache"
	bundleCacheTTL = 1 * time.Hour
)

var bundleMemCache sync.Map

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

// FetchCLIDocsBundle fetches and caches the CLI docs bundle for a package.
func FetchCLIDocsBundle(baseURL, packageName string) (*CLIDocsBundle, error) {
	if cached, ok := bundleMemCache.Load(packageName); ok {
		return cached.(*CLIDocsBundle), nil
	}

	bundle, err := loadBundleFromDisk(packageName)
	if err == nil {
		bundleMemCache.Store(packageName, bundle)
		return bundle, nil
	}

	fmt.Fprintf(os.Stderr, "Fetching %s API docs...\n", packageName)
	bundle, err = fetchBundleHTTP(baseURL, packageName)
	if err != nil {
		return nil, err
	}

	if err := saveBundleToDisk(packageName, bundle); err != nil {
		logging.V(7).Infof("failed to cache docs bundle for %s: %v", packageName, err)
	}
	bundleMemCache.Store(packageName, bundle)
	return bundle, nil
}

func fetchBundleHTTP(baseURL, packageName string) (*CLIDocsBundle, error) {
	url := fmt.Sprintf("%s/registry/packages/%s/api-docs/cli-docs.json",
		normalizeBaseURL(baseURL), packageName)

	//nolint:gosec // URL is constructed from user-provided base URL and package name
	resp, err := BundleHTTPClient.Get(url)
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

	var data []byte
	total := resp.ContentLength
	if total > 1024*1024 {
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

func readWithProgress(r io.Reader, total int64) ([]byte, error) {
	buf := make([]byte, 0, total)
	tmp := make([]byte, 256*1024)
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
			fmt.Fprintln(os.Stderr)
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
		return nil, errors.New("cache expired")
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

// LookupBundleDoc looks up a resource or function by key in the bundle.
func LookupBundleDoc(bundle *CLIDocsBundle, docKey string) (body string, title string, found bool) {
	entry, ok := bundle.Resources[docKey]
	if !ok {
		entry, ok = bundle.Functions[docKey]
	}
	if !ok {
		return "", "", false
	}

	return entry.Content, entry.Title, true
}

// ClassifiedEntry represents a single resource or function found during bundle key scanning.
type ClassifiedEntry struct {
	Key         string
	Title       string
	Description string
}

// ClassifiedKeys is the result of scanning bundle keys for a given module prefix.
type ClassifiedKeys struct {
	SubModules []string
	Resources  []ClassifiedEntry
	Functions  []ClassifiedEntry
}

// ClassifyBundleKeys scans Resources and Functions maps, classifying each key
// as a sub-module, direct resource, or direct function relative to modulePrefix.
func ClassifyBundleKeys(bundle *CLIDocsBundle, modulePrefix string) ClassifiedKeys {
	subModules := make(map[string]bool)
	var resources, functions []ClassifiedEntry

	classify := func(keys map[string]CLIDocEntry, isFunction bool) {
		for key, doc := range keys {
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

			title := doc.Title
			if title == "" {
				title = rel
			}

			entry := ClassifiedEntry{Key: key, Title: title, Description: doc.Description}
			if isFunction {
				functions = append(functions, entry)
			} else {
				resources = append(resources, entry)
			}
		}
	}

	classify(bundle.Resources, false)
	classify(bundle.Functions, true)

	sort.Slice(resources, func(i, j int) bool { return resources[i].Title < resources[j].Title })
	sort.Slice(functions, func(i, j int) bool { return functions[i].Title < functions[j].Title })

	sortedMods := make([]string, 0, len(subModules))
	for mod := range subModules {
		sortedMods = append(sortedMods, mod)
	}
	sort.Strings(sortedMods)

	return ClassifiedKeys{
		SubModules: sortedMods,
		Resources:  resources,
		Functions:  functions,
	}
}

// APIDocsBasePath returns the base URL path for a package's API docs.
func APIDocsBasePath(pkgName string) string {
	return "/registry/packages/" + pkgName + "/api-docs"
}

// APIDocsModulePath returns the URL path for a submodule under a package's API docs,
// honoring the optional parent modulePrefix.
func APIDocsModulePath(pkgName, modulePrefix, mod string) string {
	p := APIDocsBasePath(pkgName)
	if modulePrefix != "" {
		p += "/" + modulePrefix
	}
	return p + "/" + mod
}

// APIDocsEntryPath returns the URL path for a resource or function under a package's API docs.
func APIDocsEntryPath(pkgName, key string) string {
	return APIDocsBasePath(pkgName) + "/" + key
}

// absolutizeBundleLinks rewrites relative link destinations in a bundle's overview
// markdown so they resolve under /registry/packages/<pkg>/api-docs/. Links that already
// have a scheme, are rooted (start with "/"), or are anchors (start with "#") are left
// alone. This lets NumberLinks recognize bundle-authored entries as registry links.
func absolutizeBundleLinks(md, pkgName string) string {
	if md == "" || pkgName == "" {
		return md
	}
	source := []byte(md)
	tree := ParseMarkdown(source)
	basePath := APIDocsBasePath(pkgName) + "/"

	rewrote := false
	_ = ast.Walk(tree, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		link, ok := node.(*ast.Link)
		if !ok {
			return ast.WalkContinue, nil
		}
		dest := string(link.Destination)
		if dest == "" || strings.Contains(dest, "://") || strings.HasPrefix(dest, "/") || strings.HasPrefix(dest, "#") {
			return ast.WalkSkipChildren, nil
		}
		link.Destination = []byte(basePath + dest)
		rewrote = true
		return ast.WalkSkipChildren, nil
	})

	if !rewrote {
		return md
	}
	return string(renderTree(source, tree))
}

// BuildAPIDocsPage constructs an API docs overview page from the bundle data.
func BuildAPIDocsPage(bundle *CLIDocsBundle, modulePrefix, intro string) string {
	if modulePrefix == "" && bundle.Overview != "" {
		return absolutizeBundleLinks(bundle.Overview, bundle.Package)
	}
	ck := ClassifyBundleKeys(bundle, modulePrefix)
	var buf strings.Builder

	if intro != "" {
		buf.WriteString(intro)
		buf.WriteString("\n\n")
	}

	if len(ck.SubModules) > 0 {
		buf.WriteString("## " + SectionModules + "\n\n")
		links := make([]string, 0, len(ck.SubModules))
		for _, mod := range ck.SubModules {
			links = append(links, fmt.Sprintf("[%s](%s)", mod, APIDocsModulePath(bundle.Package, modulePrefix, mod)))
		}
		buf.WriteString(strings.Join(links, ", "))
		buf.WriteString("\n\n")
	}

	writeEntrySection := func(title string, entries []ClassifiedEntry) {
		if len(entries) == 0 {
			return
		}
		buf.WriteString("## " + title + "\n\n")
		for _, e := range entries {
			link := fmt.Sprintf("[**%s**](%s)", e.Title, APIDocsEntryPath(bundle.Package, e.Key))
			if e.Description != "" {
				fmt.Fprintf(&buf, "- %s — %s\n", link, truncateDescription(e.Description))
			} else {
				fmt.Fprintf(&buf, "- %s\n", link)
			}
		}
		buf.WriteString("\n")
	}

	writeEntrySection(SectionResources, ck.Resources)
	writeEntrySection(SectionFunctions, ck.Functions)

	return buf.String()
}

// truncateDescription truncates a description to a single sentence for display.
func truncateDescription(desc string) string {
	if desc == "" {
		return ""
	}
	if idx := strings.Index(desc, "\n"); idx >= 0 {
		desc = desc[:idx]
	}
	desc = strings.TrimSpace(desc)
	if idx := strings.Index(desc, ". "); idx >= 0 {
		desc = desc[:idx+1]
	}
	const maxLen = 80
	if len(desc) > maxLen {
		desc = desc[:maxLen-3] + "..."
	}
	return desc
}

// FormatPackageDetails reformats tab-indented Package Details into clean markdown.
func FormatPackageDetails(body string) string {
	start, _, endIdx := FindSectionBounds(body, "## Package Details")
	if start < 0 {
		return body
	}
	afterHeading := start + len("## Package Details")
	sectionContent := body[afterHeading:endIdx]

	var formatted strings.Builder
	lines := strings.Split(strings.TrimSpace(sectionContent), "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if i+1 < len(lines) {
			value := strings.TrimSpace(lines[i+1])
			formatted.WriteString("**" + line + ":** " + value + "\n\n")
			i++
		} else {
			formatted.WriteString("**" + line + "**\n\n")
		}
	}
	return body[:afterHeading] + "\n\n" + formatted.String() + body[endIdx:]
}
