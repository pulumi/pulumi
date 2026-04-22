// Copyright 2016, Pulumi Corporation.
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

package logs

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	backend_secrets "github.com/pulumi/pulumi/pkg/v3/backend/secrets"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/engine/encryptedlog"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

func newDecryptCmd(ws pkgWorkspace.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decrypt [filename]",
		Short: "Decrypt and display automatic logs",
		Long: "Decrypt and display the contents of an automatic log file.\n" +
			"\n" +
			"If no filename is provided, the most recent log file is\n" +
			"decrypted. When a current stack is selected (or --stack is\n" +
			"given), logs for that stack are preferred.\n" +
			"\n" +
			"For encrypted logs, the stack's secrets provider is used\n" +
			"for decryption (parsed from the filename, or overridden\n" +
			"with --stack). Gzip-compressed logs are decompressed\n" +
			"without needing a stack.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			stackName, _ := cmd.Flags().GetString("stack")

			var filename string
			if len(args) > 0 {
				filename = args[0]
			} else {
				filterName := stackName
				if filterName == "" {
					filterName = currentStackName(ws)
				}

				var err error
				filename, err = findLatestLog(filterName)
				if err != nil {
					return err
				}
				fmt.Fprintf(os.Stderr, "Decrypting %s\n", filename)
			}

			f, err := os.Open(filename)
			if err != nil {
				return fmt.Errorf("opening log file: %w", err)
			}
			defer f.Close()

			var magic [4]byte
			if _, err := io.ReadFull(f, magic[:]); err != nil {
				return fmt.Errorf("reading log file: %w", err)
			}
			if _, err := f.Seek(0, io.SeekStart); err != nil {
				return fmt.Errorf("seeking log file: %w", err)
			}

			out := bufio.NewWriter(os.Stdout)
			defer out.Flush()

			if string(magic[:]) == encryptedlog.Magic {
				return decryptPLOG(cmd, ws, stackName, f, out)
			}

			gz, err := gzip.NewReader(f)
			if err != nil {
				return fmt.Errorf("log file is neither encrypted nor gzip-compressed: %w", err)
			}
			defer gz.Close()

			return formatLogRecords(gz, out)
		},
	}

	return cmd
}

// decryptPLOG decrypts an encrypted PLOG log file. The stack name is
// parsed from the filename, or overridden with --stack. The secrets
// provider is read from the stack's deployment, so no project directory
// is needed.
func decryptPLOG(
	cmd *cobra.Command, ws pkgWorkspace.Context,
	stackName string, f *os.File, out io.Writer,
) error {
	ctx := cmd.Context()

	if stackName == "" {
		stackName = stackNameFromFilename(filepath.Base(f.Name()))
	}
	if stackName == "" {
		return fmt.Errorf("cannot determine stack from filename %q; use --stack to specify", filepath.Base(f.Name()))
	}

	opts := display.Options{Color: cmdutil.GetGlobalColorization()}
	s, err := cmdStack.RequireStack(
		ctx, cmdutil.Diag(), ws,
		cmdBackend.DefaultLoginManager,
		stackName, cmdStack.LoadOnly, opts,
	)
	if err != nil {
		return fmt.Errorf("loading stack %q for decryption: %w", stackName, err)
	}

	sm, err := secretsManagerFromStack(ctx, s)
	if err != nil {
		return fmt.Errorf("getting secrets manager for stack %q: %w", stackName, err)
	}

	reader, err := encryptedlog.NewReader(ctx, f, sm.Decrypter())
	if err != nil {
		return fmt.Errorf("decrypting log: %w", err)
	}

	return formatLogRecords(reader, out)
}

// stackNameFromFilename extracts the stack name from a log filename.
// Files are named "<stack>-<timestamp>[-<updateid>].log" where "/" in
// stack names is replaced with "+".
func stackNameFromFilename(name string) string {
	loc := logTimestampRe.FindStringIndex(name)
	if loc == nil {
		return ""
	}
	prefix := strings.TrimSuffix(name[:loc[0]], "-")
	if prefix == "" {
		return ""
	}
	return strings.ReplaceAll(prefix, "+", "/")
}

// secretsManagerFromStack reconstructs the secrets manager from the
// stack's stored deployment.
func secretsManagerFromStack(ctx context.Context, s backend.Stack) (secrets.Manager, error) {
	dep, err := s.Backend().ExportDeployment(ctx, s)
	if err != nil {
		return nil, fmt.Errorf("exporting deployment: %w", err)
	}
	if dep == nil || len(dep.Deployment) == 0 {
		return nil, errors.New("stack has no deployment")
	}

	var v3 apitype.DeploymentV3
	if err := json.Unmarshal(dep.Deployment, &v3); err != nil {
		return nil, fmt.Errorf("unmarshaling deployment: %w", err)
	}
	if v3.SecretsProviders == nil {
		return nil, errors.New("deployment has no secrets provider configured")
	}

	provider := backend_secrets.NamedStackProvider{StackName: s.Ref().Name().String()}
	return provider.OfType(ctx, v3.SecretsProviders.Type, v3.SecretsProviders.State)
}

func currentStackName(ws pkgWorkspace.Context) string {
	w, err := ws.New()
	if err != nil {
		return ""
	}
	return w.Settings().Stack
}

var logTimestampRe = regexp.MustCompile(`(\d{8}T\d{6})`)

// findLatestLog returns the path to the most recent .log file in
// ~/.pulumi/logs/. With a stack name, it prefers files for that stack.
// Without a stack name, it prefers "pulumi-" prefixed CLI-level logs.
func findLatestLog(stackName string) (string, error) {
	logsDir, err := workspace.GetPulumiPath("logs")
	if err != nil {
		return "", fmt.Errorf("getting log directory: %w", err)
	}

	entries, err := os.ReadDir(logsDir)
	if err != nil {
		return "", fmt.Errorf("reading log directory %s: %w", logsDir, err)
	}

	var all []logCandidate
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		ts, ok := parseLogTimestamp(e.Name())
		if !ok {
			continue
		}
		all = append(all, logCandidate{
			path: filepath.Join(logsDir, e.Name()),
			ts:   ts,
		})
	}

	if len(all) == 0 {
		return "", fmt.Errorf("no log files found in %s", logsDir)
	}

	if stackName != "" {
		safe := strings.ReplaceAll(stackName, "/", "+")
		if best := latestMatching(all, func(path string) bool {
			return strings.Contains(filepath.Base(path), safe)
		}); best != "" {
			return best, nil
		}
	} else {
		// Without a stack, prefer CLI-level logs (pulumi- prefix).
		if best := latestMatching(all, func(path string) bool {
			return strings.HasPrefix(filepath.Base(path), "pulumi-")
		}); best != "" {
			return best, nil
		}
	}

	return latestByTimestamp(all), nil
}

type logCandidate struct {
	path string
	ts   time.Time
}

func latestMatching(candidates []logCandidate, match func(path string) bool) string {
	var best *logCandidate
	for i := range candidates {
		c := &candidates[i]
		if match(c.path) && (best == nil || c.ts.After(best.ts)) {
			best = c
		}
	}
	if best != nil {
		return best.path
	}
	return ""
}

func latestByTimestamp(candidates []logCandidate) string {
	var best *logCandidate
	for i := range candidates {
		c := &candidates[i]
		if best == nil || c.ts.After(best.ts) {
			best = c
		}
	}
	return best.path
}

func parseLogTimestamp(name string) (time.Time, bool) {
	m := logTimestampRe.FindString(name)
	if m == "" {
		return time.Time{}, false
	}
	t, err := time.Parse("20060102T150405", m)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// formatLogRecords reads JSON log lines from r, reconstructs formatted
// messages from pulumi.log.arg* fields, removes those fields, and
// writes the resulting JSON to w.
func formatLogRecords(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	enc := json.NewEncoder(w)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec map[string]any
		if err := json.Unmarshal(line, &rec); err != nil {
			// Not JSON — write through as-is (e.g. old plain-text logs).
			fmt.Fprintf(w, "%s\n", line)
			continue
		}

		// Collect pulumi.log.argN keys in order, reconstruct the
		// formatted message, and delete the arg keys.
		var argKeys []string
		for k := range rec {
			if strings.HasPrefix(k, "pulumi.log.arg") {
				argKeys = append(argKeys, k)
			}
		}
		if len(argKeys) > 0 {
			sort.Strings(argKeys)
			args := make([]any, len(argKeys))
			for i, k := range argKeys {
				args[i] = rec[k]
				delete(rec, k)
			}
			if msg, ok := rec["msg"].(string); ok {
				rec["msg"] = fmt.Sprintf(msg, args...)
			}
		}

		if err := enc.Encode(rec); err != nil {
			return err
		}
	}
	return scanner.Err()
}
