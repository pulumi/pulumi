// Copyright 2016-2023, Pulumi Corporation.
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

package main

import (
	"bytes"
	"fmt"
	"io"
	iofs "io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/hexops/gotextdiff"
	"github.com/hexops/gotextdiff/myers"
	"github.com/hexops/gotextdiff/span"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func copyDirectory(fs iofs.FS, src string, dst string, edits []compiledReplacement, filter []string) error {
	return iofs.WalkDir(fs, src, func(path string, d iofs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		include := true
		for _, f := range filter {
			if strings.Contains(path, f) {
				include = false
			}
		}
		if !include {
			return nil
		}

		relativePath, err := filepath.Rel(src, path)
		contract.AssertNoErrorf(err, "path %s should be relative to %s", path, src)

		srcPath := filepath.Join(src, relativePath)
		dstPath := filepath.Join(dst, relativePath)
		contract.Assertf(srcPath == path, "srcPath %s should be equal to path %s", srcPath, path)

		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o700)
		}

		srcFile, err := fs.Open(srcPath)
		if err != nil {
			return fmt.Errorf("open file %s: %w", srcPath, err)
		}
		defer srcFile.Close()

		dstFile, err := os.Create(dstPath)
		if err != nil {
			return fmt.Errorf("create file %s: %w", dstPath, err)
		}
		defer dstFile.Close()

		editsToApply := []compiledReplacement{}
		for _, replace := range edits {
			if replace.Path.MatchString(relativePath) {
				editsToApply = append(editsToApply, replace)
			}
		}

		if len(editsToApply) > 0 {
			// Apply edits to the file
			data, err := io.ReadAll(srcFile)
			if err != nil {
				return fmt.Errorf("read file %s: %w", srcPath, err)
			}

			src := string(data)
			for _, edit := range editsToApply {
				src = edit.Pattern.ReplaceAllString(src, edit.Replacement)
			}

			n, err := dstFile.WriteString(src)
			if err != nil || n != len(src) {
				return fmt.Errorf("write file %s: %w", dstPath, err)
			}
		} else {
			// Can just do a straight copy
			_, err = io.Copy(dstFile, srcFile)
			if err != nil {
				return fmt.Errorf("copy file %s->%s: %w", srcPath, dstPath, err)
			}
		}

		return nil
	})
}

// compareDirectories compares two directories, returning a list of validation failures where files had
// different contents, or we only present in on of the directories. If allowNewFiles is true then it's ok to
// have extra files in the actual directory, we use this for checking building SDKs doesn't mutate any files,
// but doing so might add new build files (which would then normally be .gitignored).
func compareDirectories(actualDir, expectedDir string, allowNewFiles bool) ([]string, error) {
	// Validate files, we need to walk twice to get this correct because we need to check all expected
	// files are present, but also that no unexpected files are present.

	var validations []string
	// Check that every file in expected is also in actual with the same content
	err := filepath.WalkDir(expectedDir, func(path string, d iofs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// No need to check directories, just recurse into them
		if d.IsDir() {
			return nil
		}

		relativePath, err := filepath.Rel(expectedDir, path)
		contract.AssertNoErrorf(err, "path %s should be relative to %s", path, expectedDir)

		// Check that the file is present in the expected directory and has the same contents
		expectedContents, err := os.ReadFile(filepath.Join(expectedDir, relativePath))
		if err != nil {
			return fmt.Errorf("read expected file: %w", err)
		}

		actualPath := filepath.Join(actualDir, relativePath)
		actualContents, err := os.ReadFile(actualPath)
		// An error here is a test failure rather than an error, add this to the validation list
		if err != nil {
			validations = append(validations, fmt.Sprintf("expected file %s could not be read", relativePath))
			// Move on to the next file
			return nil
		}

		if !bytes.Equal(actualContents, expectedContents) {
			edits := myers.ComputeEdits(
				span.URIFromPath("expected"), string(expectedContents), string(actualContents),
			)
			diff := gotextdiff.ToUnified("expected", "actual", string(expectedContents), edits)

			validations = append(validations, fmt.Sprintf(
				"expected file %s does not match actual file:\n\n%s", relativePath, diff),
			)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk expected dir: %w", err)
	}

	// Now walk the actual directory and check every file found is present in the expected directory, i.e.
	// there aren't any new files that aren't expected. We've already done contents checking so we just need
	// existence checks.
	if !allowNewFiles {
		err = filepath.WalkDir(actualDir, func(path string, d iofs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// No need to check directories
			if d.IsDir() {
				return nil
			}

			relativePath, err := filepath.Rel(actualDir, path)
			contract.AssertNoErrorf(err, "path %s should be relative to %s", path, actualDir)

			// Just need to see if this file exists in expected, if it doesn't return add a validation failure.
			_, err = os.Stat(filepath.Join(expectedDir, relativePath))
			if err == nil {
				// File exists in expected, we've already done a contents check so just move on to
				// the next file.
				return nil
			}

			// Check if this was a NotFound error in which case add a validation failure, else return the error
			if os.IsNotExist(err) {
				validations = append(validations, fmt.Sprintf("file %s is not expected", relativePath))
				return nil
			}

			return err
		})
		if err != nil {
			return nil, fmt.Errorf("walk actual dir: %w", err)
		}
	}

	return validations, nil
}

// editSnapshot applies the given edits to the snapshot files in the snapshot directory, it either returns the
// given path directly, or creates a new temporary folder and returns the path to that with the edits applied
// in that folder
//
// It _might_ be worth changing this approach to instead fold this edit logic directly into the comparison
// itself rather than having to copy all the files in a snapshot. i.e. an one the file mutation of each file
// as part of the compare rather than edit and write all then doing a direct comparison.
func editSnapshot(snapshotDirectory string, edits []compiledReplacement) (string, error) {
	// If we have any edits to apply then we need to copy to a temporary directory and apply the edits there.
	result := snapshotDirectory
	if len(edits) > 0 {
		var err error
		result, err = os.MkdirTemp("", "pulumi-test-language")
		if err != nil {
			return "", fmt.Errorf("create temp dir: %w", err)
		}

		err = copyDirectory(os.DirFS(snapshotDirectory), ".", result, edits, nil)
		if err != nil {
			return "", fmt.Errorf("copy source dir: %w", err)
		}
	}
	return result, nil
}

// Do a snapshot check of the generated source code against the snapshot code. If PULUMI_ACCEPT is true just
// write the new files instead.
func doSnapshot(
	disableSnapshotWriting bool,
	sourceDirectory, snapshotDirectory string,
) ([]string, error) {
	if !disableSnapshotWriting && cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT")) {
		// Write files
		err := os.RemoveAll(snapshotDirectory)
		if err != nil {
			return nil, fmt.Errorf("remove snapshot dir: %w", err)
		}
		err = os.MkdirAll(snapshotDirectory, 0o755)
		if err != nil {
			return nil, fmt.Errorf("create snapshot dir: %w", err)
		}
		err = copyDirectory(os.DirFS(sourceDirectory), ".", snapshotDirectory, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("copy snapshot dir: %w", err)
		}
		return nil, nil
	}
	validations, err := compareDirectories(sourceDirectory, snapshotDirectory, false /* allowNewFiles */)
	if err != nil {
		return nil, err
	}

	return validations, nil
}
