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

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	SectionModules   = "Modules"
	SectionResources = "Resources"
	SectionFunctions = "Functions"
)

// CLIDocsBundle represents the bundled CLI docs JSON for a package.
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
	return "CLI docs bundle not available for package: " + e.Package
}

const (
	bundleCacheDir = "cli-docs-cache"
	bundleCacheTTL = 1 * time.Hour
)

var bundleMemCache sync.Map

// ParseAPIDocsPath extracts the package name and doc key from an API docs path.
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

	if err := saveBundleToDisk(packageName, bundle); err != nil {
		logging.V(7).Infof("failed to cache docs bundle for %s: %v", packageName, err)
	}
	bundleMemCache.Store(packageName, bundle)
	return bundle, nil
}

func fetchBundleHTTP(baseURL, packageName string) (*CLIDocsBundle, error) {
	url := fmt.Sprintf("%s/registry/packages/%s/api-docs/cli-docs.json",
		strings.TrimRight(baseURL, "/"), packageName)

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
	content, ok := bundle.Resources[docKey]
	if !ok {
		content, ok = bundle.Functions[docKey]
	}
	if !ok {
		return "", "", false
	}

	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\t", "    ")

	title = ExtractBundleTitle(content)
	if title != "" {
		if idx := strings.Index(content, "\n"); idx >= 0 {
			content = strings.TrimLeft(content[idx+1:], "\n")
		}
	}

	return content, title, true
}

// ClassifiedEntry represents a single resource or function found during bundle key scanning.
type ClassifiedEntry struct {
	Key     string
	Title   string
	Content string
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

			title := ExtractBundleTitle(content)
			if title == "" {
				title = rel
			}

			entry := ClassifiedEntry{Key: key, Title: title, Content: content}
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

// RenderBundleTable renders a formatted table of modules, resources, and/or functions.
func RenderBundleTable(bundle *CLIDocsBundle, modulePrefix string) string {
	ck := ClassifyBundleKeys(bundle, modulePrefix)

	if len(ck.SubModules) == 0 && len(ck.Resources) == 0 && len(ck.Functions) == 0 {
		return ""
	}

	modules := make([]tableEntry, len(ck.SubModules))
	for i, mod := range ck.SubModules {
		modules[i] = tableEntry{name: mod}
	}

	resources := make([]tableEntry, len(ck.Resources))
	for i, r := range ck.Resources {
		resources[i] = tableEntry{name: r.Title, desc: ExtractBundleDescription(r.Content)}
	}

	functions := make([]tableEntry, len(ck.Functions))
	for i, f := range ck.Functions {
		functions[i] = tableEntry{name: f.Title, desc: ExtractBundleDescription(f.Content)}
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
		fmt.Fprintf(&buf, "\n  %s\n", SectionModules)
		renderColumns(&buf, modules)
	}

	renderDescSection := func(label string, entries []tableEntry) {
		if len(entries) == 0 {
			return
		}
		fmt.Fprintf(&buf, "\n  %s\n\n", label)
		for _, e := range entries {
			if e.desc != "" {
				fmt.Fprintf(&buf, "%-*s  %s\n", maxName, e.name, e.desc)
			} else {
				fmt.Fprintf(&buf, "%s\n", e.name)
			}
		}
	}

	renderDescSection(SectionResources, resources)
	renderDescSection(SectionFunctions, functions)
	buf.WriteString("\n")

	return buf.String()
}

// BuildAPIDocsPage constructs a clean API docs overview page from the bundle data.
// If intro is non-empty, it's prepended as the page description.
func BuildAPIDocsPage(bundle *CLIDocsBundle, modulePrefix, intro string) string {
	ck := ClassifyBundleKeys(bundle, modulePrefix)
	var buf strings.Builder

	if intro != "" {
		buf.WriteString(intro)
		buf.WriteString("\n\n")
	}

	if len(ck.SubModules) > 0 {
		buf.WriteString("## " + SectionModules + "\n\n")
		buf.WriteString(strings.Join(ck.SubModules, ", "))
		buf.WriteString("\n\n")
	}

	writeEntrySection := func(title string, entries []ClassifiedEntry) {
		if len(entries) == 0 {
			return
		}
		buf.WriteString("## " + title + "\n\n")
		for _, e := range entries {
			desc := ExtractBundleDescription(e.Content)
			if desc != "" {
				fmt.Fprintf(&buf, "- **%s** — %s\n", e.Title, desc)
			} else {
				fmt.Fprintf(&buf, "- %s\n", e.Title)
			}
		}
		buf.WriteString("\n")
	}

	writeEntrySection(SectionResources, ck.Resources)
	writeEntrySection(SectionFunctions, ck.Functions)

	return buf.String()
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

// RenderBundleSingleSection renders a single section (Modules, Resources, or Functions)
// as plain text suitable for direct terminal output.
func RenderBundleSingleSection(bundle *CLIDocsBundle, modulePrefix, section string) string {
	ck := ClassifyBundleKeys(bundle, modulePrefix)
	return renderBundleSectionDirect(ck, section)
}

// renderBundleSectionDirect renders a single section as plain text for direct output.
func renderBundleSectionDirect(ck ClassifiedKeys, section string) string {
	var buf strings.Builder

	switch section {
	case SectionModules:
		if len(ck.SubModules) == 0 {
			return ""
		}
		bold := ANSIBold
		reset := ANSIReset
		modules := make([]tableEntry, len(ck.SubModules))
		for i, mod := range ck.SubModules {
			modules[i] = tableEntry{name: bold + mod + reset, width: len(mod)}
		}
		renderColumns(&buf, modules)

	case SectionResources, SectionFunctions:
		var entries []ClassifiedEntry
		if section == SectionResources {
			entries = ck.Resources
		} else {
			entries = ck.Functions
		}
		if len(entries) == 0 {
			return ""
		}

		maxName := 0
		for _, e := range entries {
			if len(e.Title) > maxName {
				maxName = len(e.Title)
			}
		}

		bold := ANSIBold
		reset := ANSIReset
		for _, e := range entries {
			desc := ExtractBundleDescription(e.Content)
			if desc != "" {
				// Pad based on visible width, then add ANSI bold
				pad := maxName - len(e.Title)
				buf.WriteString(bold + e.Title + reset + strings.Repeat(" ", pad) + "  " + desc + "\n")
			} else {
				buf.WriteString(bold + e.Title + reset + "\n")
			}
		}
	}

	return buf.String()
}

type tableEntry struct {
	name  string // display string (may contain ANSI codes)
	width int    // visible width (0 means use len(name))
	desc  string
}

func (e tableEntry) visibleWidth() int {
	if e.width > 0 {
		return e.width
	}
	return len(e.name)
}

func renderColumns(buf *strings.Builder, entries []tableEntry) {
	if len(entries) == 0 {
		return
	}

	availWidth := GetTerminalWidth() - 8
	const gap = 2

	bestCols := 1
	for tryC := 2; tryC <= len(entries); tryC++ {
		rows := (len(entries) + tryC - 1) / tryC
		totalWidth := 0
		fits := true
		for c := range tryC {
			colMax := 0
			for r := range rows {
				idx := c*rows + r
				if idx < len(entries) && entries[idx].visibleWidth() > colMax {
					colMax = entries[idx].visibleWidth()
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

	rows := (len(entries) + bestCols - 1) / bestCols

	colWidths := make([]int, bestCols)
	for c := range bestCols {
		for r := range rows {
			idx := c*rows + r
			if idx < len(entries) && entries[idx].visibleWidth() > colWidths[c] {
				colWidths[c] = entries[idx].visibleWidth()
			}
		}
	}

	for r := range rows {
		for c := range bestCols {
			idx := c*rows + r
			if idx >= len(entries) {
				break
			}
			e := entries[idx]
			if c < bestCols-1 {
				pad := colWidths[c] + gap - e.visibleWidth()
				buf.WriteString(e.name + strings.Repeat(" ", pad))
			} else {
				buf.WriteString(e.name)
			}
		}
		buf.WriteString("\n")
	}
}
