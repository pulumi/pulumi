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

package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/pmezard/go-difflib/difflib"
)

// Filesystem is the local handler for the Neo `filesystem` tool. Method names and argument
// shapes mirror the upstream mcp-claude-code filesystem tools used by the cloud agent — see
// pulumi-service:cmd/agents/vendored/mcp-claude-code/mcp_claude_code/tools/filesystem/.
//
// Supported methods (this slice):
//
//	read            — args: {file_path, offset?, limit?}
//	write           — args: {file_path, content}
//	directory_tree  — args: {path, depth?, include_filtered?}
//	edit            — args: {file_path, old_string, new_string, expected_replacements?}
//	grep            — args: {pattern, path?, include?}
//	content_replace — args: {pattern, replacement, path, file_pattern?, dry_run?}
//
// The remaining filesystem method (grep_ast) returns a structured "not yet implemented"
// error so the agent can degrade gracefully.
//
// All paths in incoming requests are absolute (the cloud agent assumes a sandboxed VM).
// We resolve them and reject anything that lands outside the allowed roots, so the agent
// can never read or write files outside Root or the configured extras (e.g. /tmp).
type Filesystem struct {
	// Root is the user's working directory. Relative paths are joined against it.
	Root string
	// allowedRoots is Root followed by any extra roots passed to NewFilesystem.
	allowedRoots []string
}

// NewFilesystem creates a Filesystem handler rooted at the given absolute directory.
// extraRoots are additional directories the agent may read or write under (e.g. /tmp);
// each must exist and is canonicalized the same way as root.
func NewFilesystem(root string, extraRoots ...string) (*Filesystem, error) {
	abs, err := canonicalRoot(root)
	if err != nil {
		return nil, fmt.Errorf("resolving filesystem root: %w", err)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("filesystem root %q: %w", abs, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("filesystem root %q is not a directory", abs)
	}
	allowed := []string{abs}
	for _, extra := range extraRoots {
		canonical, err := canonicalRoot(extra)
		if err != nil {
			return nil, fmt.Errorf("resolving filesystem extra root %q: %w", extra, err)
		}
		extraInfo, err := os.Stat(canonical)
		if err != nil {
			return nil, fmt.Errorf("filesystem extra root %q: %w", canonical, err)
		}
		if !extraInfo.IsDir() {
			return nil, fmt.Errorf("filesystem extra root %q is not a directory", canonical)
		}
		allowed = append(allowed, canonical)
	}
	return &Filesystem{Root: abs, allowedRoots: allowed}, nil
}

// Invoke dispatches a single filesystem method call.
func (f *Filesystem) Invoke(_ context.Context, method string, args json.RawMessage) (any, error) {
	switch method {
	case "read":
		var p struct {
			FilePath string `json:"file_path"`
			Offset   int    `json:"offset,omitempty"`
			Limit    int    `json:"limit,omitempty"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, fmt.Errorf("decoding read args: %w", err)
		}
		return f.read(p.FilePath, p.Offset, p.Limit)
	case "write":
		var p struct {
			FilePath string `json:"file_path"`
			Content  string `json:"content"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, fmt.Errorf("decoding write args: %w", err)
		}
		return f.write(p.FilePath, p.Content)
	case "directory_tree":
		var p struct {
			Path  string `json:"path"`
			Depth int    `json:"depth,omitempty"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, fmt.Errorf("decoding directory_tree args: %w", err)
		}
		return f.directoryTree(p.Path, p.Depth)
	case "edit":
		var p editArgs
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, fmt.Errorf("decoding edit args: %w", err)
		}
		return f.edit(p)
	case "grep":
		var p struct {
			Pattern string `json:"pattern"`
			Path    string `json:"path,omitempty"`
			Include string `json:"include,omitempty"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, fmt.Errorf("decoding grep args: %w", err)
		}
		return f.grep(p.Pattern, p.Path, p.Include)
	case "content_replace":
		var p struct {
			Pattern     string `json:"pattern"`
			Replacement string `json:"replacement"`
			Path        string `json:"path"`
			FilePattern string `json:"file_pattern,omitempty"`
			DryRun      bool   `json:"dry_run,omitempty"`
		}
		if err := json.Unmarshal(args, &p); err != nil {
			return nil, fmt.Errorf("decoding content_replace args: %w", err)
		}
		return f.contentReplace(p.Pattern, p.Replacement, p.Path, p.FilePattern, p.DryRun)
	case "grep_ast":
		return nil, fmt.Errorf("filesystem method %q is not yet implemented in pulumi neo CLI mode", method)
	default:
		return nil, fmt.Errorf("unknown filesystem method %q", method)
	}
}

// resolve safely interprets a path supplied by the agent. The agent sends absolute paths
// (they were absolute inside its sandboxed VM); we accept those but require they resolve
// to a location under one of the allowed roots. Relative paths are resolved against Root.
func (f *Filesystem) resolve(p string) (string, error) {
	if p == "" {
		return "", errors.New("path is required")
	}
	target := p
	if !filepath.IsAbs(target) {
		target = filepath.Join(f.Root, target)
	}
	return resolveUnderRoots(f.allowedRoots, target, true)
}

type readResult struct {
	Content string `json:"content"`
}

func (f *Filesystem) read(p string, offset, limit int) (readResult, error) {
	abs, err := f.resolve(p)
	if err != nil {
		return readResult{}, err
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return readResult{}, err
	}
	content := string(b)
	if offset > 0 || limit > 0 {
		// Apply line-based slicing matching the upstream tool's offset/limit semantics.
		lines := strings.Split(content, "\n")
		if offset >= len(lines) {
			content = ""
		} else {
			if offset > 0 {
				lines = lines[offset:]
			}
			if limit > 0 && limit < len(lines) {
				lines = lines[:limit]
			}
			content = strings.Join(lines, "\n")
		}
	}
	return readResult{Content: content}, nil
}

type writeResult struct {
	BytesWritten int `json:"bytes_written"`
}

func (f *Filesystem) write(p, content string) (writeResult, error) {
	abs, err := f.resolve(p)
	if err != nil {
		return writeResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return writeResult{}, err
	}
	if err := os.WriteFile(abs, []byte(content), 0o600); err != nil {
		return writeResult{}, err
	}
	return writeResult{BytesWritten: len(content)}, nil
}

// editArgs is the JSON shape for the `edit` tool. ExpectedReplacements is a pointer so
// "omitted" (defaults to 1) is distinguishable from an explicit 0 — the latter is a
// validation error rather than "replace nothing".
type editArgs struct {
	FilePath             string `json:"file_path"`
	OldString            string `json:"old_string"`
	NewString            string `json:"new_string"`
	ExpectedReplacements *int   `json:"expected_replacements,omitempty"`
}

// edit performs a single exact-string replacement. The response string and error wording
// are deliberately kept byte-identical to the upstream mcp-claude-code `edit` tool so the
// agent sees the same output whether the call ran on Cloud or CLI.
func (f *Filesystem) edit(p editArgs) (string, error) {
	abs, err := f.resolve(p.FilePath)
	if err != nil {
		return "", err
	}

	expected := 1
	if p.ExpectedReplacements != nil {
		expected = *p.ExpectedReplacements
	}
	if expected < 0 {
		return "Error: Parameter 'expected_replacements' must be a non-negative number", nil
	}

	info, statErr := os.Stat(abs)
	fileExists := statErr == nil
	if statErr != nil && !os.IsNotExist(statErr) {
		return "", statErr
	}

	// Creation mode: file doesn't exist and old_string is empty.
	if !fileExists && p.OldString == "" {
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(abs, []byte(p.NewString), 0o600); err != nil {
			return "", err
		}
		return fmt.Sprintf("Successfully created file: %s (%d bytes)", p.FilePath, len(p.NewString)), nil
	}

	if !fileExists {
		return "", statErr
	}
	if info.IsDir() {
		return "", fmt.Errorf("path %q is a directory, not a file", p.FilePath)
	}

	// Whitespace-only old_string on an existing file is ambiguous — reject it rather
	// than silently matching every run of whitespace in the file.
	if strings.TrimSpace(p.OldString) == "" {
		return "Error: Parameter 'old_string' cannot be empty for existing files", nil
	}

	raw, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(raw) {
		return "Error: Cannot edit binary file: " + p.FilePath, nil
	}
	original := string(raw)

	occurrences := strings.Count(original, p.OldString)
	if occurrences == 0 {
		return "Error: The specified old_string was not found in the file content. " +
			"Please check that it matches exactly, including all whitespace and indentation.", nil
	}
	if occurrences != expected {
		return fmt.Sprintf(
			"Error: Found %d occurrences of the specified old_string, but expected %d. "+
				"Change your old_string to uniquely identify the target text, "+
				"or set expected_replacements=%d to replace all occurrences.",
			occurrences, expected, occurrences), nil
	}

	modified := strings.ReplaceAll(original, p.OldString, p.NewString)

	diff, err := renderUnifiedDiff(p.FilePath, original, modified)
	if err != nil {
		return "", err
	}
	if diff == "" {
		return "No changes made to file: " + p.FilePath, nil
	}

	if err := os.WriteFile(abs, []byte(modified), 0o600); err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"Successfully edited file: %s (%d replacements applied)\n\n%s",
		p.FilePath, expected, diff,
	), nil
}

// renderUnifiedDiff produces a unified diff formatted the same way as the upstream Python
// tool: `--- <path> (original)` / `+++ <path> (modified)` headers, 3 lines of context,
// and the body wrapped in a ```diff fenced block. Returns an empty string when the two
// inputs are identical.
func renderUnifiedDiff(path, original, modified string) (string, error) {
	if original == modified {
		return "", nil
	}
	body, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(original),
		B:        difflib.SplitLines(modified),
		FromFile: path + " (original)",
		ToFile:   path + " (modified)",
		Context:  3,
	})
	if err != nil {
		return "", err
	}
	if body == "" {
		return "", nil
	}
	return "```diff\n" + body + "```\n", nil
}

// isBinary heuristically decides if a byte slice is a text file. A NUL byte is how grep,
// git, and most Unix tools detect binary content; it's good enough here to steer the
// agent away from corrupting compiled artefacts.
func isBinary(b []byte) bool {
	return bytes.IndexByte(b, 0) != -1
}

// capString truncates s to limit bytes, reporting whether truncation happened.
func capString(s string, limit int) (string, bool) {
	if len(s) <= limit {
		return s, false
	}
	return s[:limit], true
}

type directoryEntry struct {
	Name  string `json:"name"`
	IsDir bool   `json:"is_dir"`
}

type directoryTreeResult struct {
	Entries []directoryEntry `json:"entries"`
}

// directoryTree returns a flat listing of the directory at p, optionally bounded by depth.
// Depth 0 (or unset) defaults to 1 (immediate children only).
func (f *Filesystem) directoryTree(p string, depth int) (directoryTreeResult, error) {
	abs, err := f.resolve(p)
	if err != nil {
		return directoryTreeResult{}, err
	}
	if depth <= 0 {
		depth = 1
	}
	var entries []directoryEntry
	err = filepath.Walk(abs, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, _ := filepath.Rel(abs, path)
		if rel == "." {
			return nil
		}
		if strings.Count(rel, string(filepath.Separator))+1 > depth {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		entries = append(entries, directoryEntry{Name: rel, IsDir: info.IsDir()})
		return nil
	})
	if err != nil {
		return directoryTreeResult{}, err
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return directoryTreeResult{Entries: entries}, nil
}

// walkMatching invokes fn for every regular file under abs whose basename matches glob.
// If abs is itself a file, fn is called directly when its basename matches. Hidden
// directories and node_modules are skipped below the starting path — if the caller
// explicitly names such a directory as abs, its contents are traversed.
//
// glob is assumed valid; callers validate it up front with filepath.Match.
func walkMatching(abs, glob string, fn func(path string) error) error {
	info, err := os.Stat(abs)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		if ok, _ := filepath.Match(glob, filepath.Base(abs)); ok {
			return fn(abs)
		}
		return nil
	}
	return filepath.WalkDir(abs, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		name := d.Name()
		if d.IsDir() {
			// Skip hidden directories and node_modules below the starting path. An
			// explicit abs that names a hidden dir is traversed.
			if p != abs && (strings.HasPrefix(name, ".") || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if ok, _ := filepath.Match(glob, name); ok {
			return fn(p)
		}
		return nil
	})
}

type grepResult struct {
	Content   string `json:"content"`
	Truncated bool   `json:"truncated,omitempty"`
}

// grep scans files under searchPath line-by-line for regex matches, restricting to files
// whose basename matches the include glob. Output mirrors the upstream mcp-claude-code
// grep tool: `<abs-path>:<lineno>: <line>` entries grouped under a count header. Binary
// files, hidden directories, and node_modules are skipped.
func (f *Filesystem) grep(pattern, searchPath, include string) (grepResult, error) {
	if pattern == "" {
		return grepResult{}, errors.New("pattern is required")
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return grepResult{}, fmt.Errorf("invalid regex %q: %w", pattern, err)
	}
	if include == "" {
		include = "*"
	}
	// Validate the glob early so callers get a clear error instead of silent zero-match.
	if _, err := filepath.Match(include, "x"); err != nil {
		return grepResult{}, fmt.Errorf("invalid include glob %q: %w", include, err)
	}
	if searchPath == "" {
		searchPath = "."
	}
	abs, err := f.resolve(searchPath)
	if err != nil {
		return grepResult{}, err
	}

	type match struct {
		path    string
		lineno  int
		content string
	}
	var matches []match

	scanFile := func(path string) error {
		b, err := os.ReadFile(path)
		if err != nil {
			return nil //nolint:nilerr // skip unreadable files, don't halt the scan
		}
		if isBinary(b) {
			return nil
		}
		lineno := 0
		for len(b) > 0 {
			lineno++
			var line []byte
			line, b, _ = bytes.Cut(b, []byte{'\n'})
			// Match bufio.Scanner's line semantics: strip a trailing \r so regexes
			// with $ behave the same on Windows-formatted files.
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			if re.Match(line) {
				matches = append(matches, match{path: path, lineno: lineno, content: string(line)})
			}
		}
		return nil
	}

	if err := walkMatching(abs, include, scanFile); err != nil {
		return grepResult{}, err
	}

	if len(matches) == 0 {
		return grepResult{Content: "No matches found."}, nil
	}

	// Sort by path, then by line number, so output is deterministic across filesystems.
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].path != matches[j].path {
			return matches[i].path < matches[j].path
		}
		return matches[i].lineno < matches[j].lineno
	})

	uniqueFiles := 0
	var prev string
	for _, m := range matches {
		if m.path != prev {
			uniqueFiles++
			prev = m.path
		}
	}

	var buf strings.Builder
	fmt.Fprintf(&buf, "Found %d matches in %d file(s):\n\n", len(matches), uniqueFiles)
	for _, m := range matches {
		fmt.Fprintf(&buf, "%s:%d: %s\n", m.path, m.lineno, m.content)
	}
	body, truncated := capString(buf.String(), maxOutputBytes)
	return grepResult{Content: body, Truncated: truncated}, nil
}

type contentReplaceResult struct {
	Content          string `json:"content"`
	ReplacementsMade int    `json:"replacements_made"`
	FilesModified    int    `json:"files_modified"`
	DryRun           bool   `json:"dry_run"`
	Truncated        bool   `json:"truncated,omitempty"`
}

// contentReplace performs a literal (not regex) search-and-replace across every file under
// searchPath whose basename matches filePattern. In dry-run mode no files are written; the
// output summarizes what would change. Mirrors the upstream mcp-claude-code
// `content_replace` tool.
func (f *Filesystem) contentReplace(
	pattern, replacement, searchPath, filePattern string, dryRun bool,
) (contentReplaceResult, error) {
	if pattern == "" {
		return contentReplaceResult{}, errors.New("pattern is required")
	}
	if searchPath == "" {
		return contentReplaceResult{}, errors.New("path is required")
	}
	if filePattern == "" {
		filePattern = "*"
	}
	if _, err := filepath.Match(filePattern, "x"); err != nil {
		return contentReplaceResult{}, fmt.Errorf("invalid file_pattern %q: %w", filePattern, err)
	}
	abs, err := f.resolve(searchPath)
	if err != nil {
		return contentReplaceResult{}, err
	}

	type fileResult struct {
		path  string
		count int
	}
	var results []fileResult
	totalReplacements := 0

	processFile := func(path string) error {
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil //nolint:nilerr // skip unreadable files (same stance as grep)
		}
		if isBinary(b) {
			return nil
		}
		before := string(b)
		count := strings.Count(before, pattern)
		if count == 0 {
			return nil
		}
		results = append(results, fileResult{path: path, count: count})
		totalReplacements += count
		if dryRun {
			return nil
		}
		after := strings.ReplaceAll(before, pattern, replacement)
		return os.WriteFile(path, []byte(after), 0o600)
	}

	if err := walkMatching(abs, filePattern, processFile); err != nil {
		return contentReplaceResult{}, err
	}

	if totalReplacements == 0 {
		return contentReplaceResult{}, fmt.Errorf("pattern %q not found in any matching file", pattern)
	}

	sort.Slice(results, func(i, j int) bool { return results[i].path < results[j].path })

	var buf strings.Builder
	if dryRun {
		fmt.Fprintf(&buf, "Dry run: %d replacements of '%s' with '%s' would be made in %d files:\n\n",
			totalReplacements, pattern, replacement, len(results))
	} else {
		fmt.Fprintf(&buf, "Made %d replacements of '%s' with '%s' in %d files:\n\n",
			totalReplacements, pattern, replacement, len(results))
	}
	for _, r := range results {
		fmt.Fprintf(&buf, "%s: %d replacements\n", r.path, r.count)
	}
	body, truncated := capString(buf.String(), maxOutputBytes)
	return contentReplaceResult{
		Content:          body,
		ReplacementsMade: totalReplacements,
		FilesModified:    len(results),
		DryRun:           dryRun,
		Truncated:        truncated,
	}, nil
}
