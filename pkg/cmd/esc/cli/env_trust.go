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

package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/ui"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

type trustDecision string

const (
	trustAccept trustDecision = "accept"
	trustDeny   trustDecision = "deny"
)

// dotESCTrustRecord records the user's decision about a particular .esc.yaml. A record is stale
// once the file's path or contents change; stale records count as no decision.
type dotESCTrustRecord struct {
	Path      string    `json:"path"`
	SHA256    string    `json:"sha256"`
	Decision  string    `json:"decision"`
	DecidedAt time.Time `json:"decidedAt"`
}

// trustDir returns the directory that holds .esc.yaml trust records.
func (esc *escCommand) trustDir() (string, error) {
	home := esc.pulumiHome
	if home == "" {
		h, err := workspace.GetPulumiHomeDir()
		if err != nil {
			return "", fmt.Errorf("getting Pulumi home directory: %w", err)
		}
		home = h
	}
	return filepath.Join(home, "esc", "trust"), nil
}

func trustRecordPath(dir, dotESCPath string) string {
	sum := sha256.Sum256([]byte(dotESCPath))
	return filepath.Join(dir, hex.EncodeToString(sum[:])+".json")
}

func contentsHash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// getTrust returns the recorded decision for the .esc.yaml at path with the given contents, if
// any. Records that are missing, malformed, or stale count as no decision.
func (cmd *envCommand) getTrust(path string, contents []byte) trustDecision {
	dir, err := cmd.esc.trustDir()
	if err != nil {
		return ""
	}
	b, err := cmd.esc.fs.ReadFile(trustRecordPath(dir, path))
	if err != nil {
		return ""
	}
	var record dotESCTrustRecord
	if err := json.Unmarshal(b, &record); err != nil {
		return ""
	}
	if record.Path != path || record.SHA256 != contentsHash(contents) {
		return ""
	}
	switch d := trustDecision(record.Decision); d {
	case trustAccept, trustDeny:
		return d
	default:
		return ""
	}
}

func (cmd *envCommand) setTrust(path string, contents []byte, decision trustDecision) error {
	dir, err := cmd.esc.trustDir()
	if err != nil {
		return err
	}
	if err := cmd.esc.fs.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating trust directory: %w", err)
	}
	record := dotESCTrustRecord{
		Path:      path,
		SHA256:    contentsHash(contents),
		Decision:  string(decision),
		DecidedAt: time.Now().UTC(),
	}
	b, err := json.Marshal(record)
	if err != nil {
		return err
	}
	if err := cmd.esc.fs.WriteFile(trustRecordPath(dir, path), b, 0o600); err != nil {
		return fmt.Errorf("writing trust record: %w", err)
	}
	return nil
}

func (cmd *envCommand) revokeTrust(path string) error {
	dir, err := cmd.esc.trustDir()
	if err != nil {
		return err
	}
	if err := cmd.esc.fs.Remove(trustRecordPath(dir, path)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// checkDotESCTrust reports whether the .esc.yaml at path may be used as the source of the default
// environment, prompting the user to accept or deny an unrecognized file when the session is
// interactive. relPath is the path to display in messages.
func (cmd *envCommand) checkDotESCTrust(path, relPath string, contents []byte) (bool, error) {
	switch cmd.getTrust(path, contents) {
	case trustAccept:
		return true, nil
	case trustDeny:
		return false, nil
	}

	if !cmd.esc.interactive {
		msg := fmt.Sprintf(
			"%swarning: ignoring untrusted %v; run `esc env default --accept` to accept it%s\n",
			colors.SpecWarning, relPath, colors.Reset,
		)
		fmt.Fprint(cmd.esc.stderr, cmd.esc.colors.Colorize(msg))
		return false, nil
	}

	return cmd.promptDotESCTrust(path, relPath, contents)
}

// promptDotESCTrust shows the contents of an unrecognized .esc.yaml and asks the user whether to
// accept it as the source of the default environment. Explicit answers are recorded; anything
// else ignores the file for this invocation only.
func (cmd *envCommand) promptDotESCTrust(path, relPath string, contents []byte) (bool, error) {
	fmt.Fprintf(cmd.esc.stderr, "esc: found default environment configuration at %v:\n\n", relPath)
	for line := range strings.SplitSeq(strings.TrimRight(string(contents), "\n"), "\n") {
		fmt.Fprintf(cmd.esc.stderr, "    %v\n", line)
	}
	fmt.Fprintln(cmd.esc.stderr)

	yes, answered := ui.YesNoPrompt("Accept this default environment configuration?", false, display.Options{
		Color:  cmd.esc.colors,
		Stdin:  cmd.esc.stdin,
		Stdout: cmd.esc.stderr,
	})
	switch {
	case !answered:
		return false, nil
	case yes:
		return true, cmd.setTrust(path, contents, trustAccept)
	default:
		return false, cmd.setTrust(path, contents, trustDeny)
	}
}
