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

package logs

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	cmdBackend "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/backend"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/engine/encryptedlog"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

func newShareCmd(ws pkgWorkspace.Context) *cobra.Command {
	var includeSecrets bool

	cmd := &cobra.Command{
		Use:   "share [filename]",
		Short: "Re-encrypt a log file for sharing with Pulumi support",
		Long: "Create a copy of a log file that can be safely shared with\n" +
			"Pulumi support. The log content is re-encrypted with a key\n" +
			"that only Pulumi can read.\n" +
			"\n" +
			"If no filename is provided, the most recent log file is\n" +
			"used. When a current stack is selected (or --stack is\n" +
			"given), logs for that stack are preferred.\n" +
			"\n" +
			"By default, secret values in the log are redacted. Use\n" +
			"--include-secrets to include them in the shared log.",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
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
				fmt.Fprintf(os.Stderr, "Sharing %s\n", filename)
			}

			sessionID, sessionKey, err := createEncryptionSession(ctx, ws)
			if err != nil {
				return err
			}

			outPath := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".shared.log"
			redact := !includeSecrets

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

			if string(magic[:]) == encryptedlog.Magic {
				err = sharePLOG(ctx, ws, stackName, f, outPath, sessionID, sessionKey, redact)
			} else {
				err = shareGzip(f, outPath, sessionID, sessionKey, redact)
			}
			if err != nil {
				return err
			}

			fmt.Fprintf(os.Stderr, "Shared log written to %s\n\n", outPath)
			fmt.Fprintf(os.Stderr, "You can safely attach this file to a GitHub issue or\n")
			fmt.Fprintf(os.Stderr, "send it to Pulumi support. Only Pulumi can decrypt it.\n")
			return nil
		},
	}

	cmd.Flags().BoolVar(&includeSecrets, "include-secrets", false,
		"Include secret values in the shared log (by default secrets are redacted)")

	return cmd
}

// sharePLOG decrypts a PLOG file, optionally redacts secrets, and
// re-encrypts the content with the service-provided session key.
func sharePLOG(
	ctx context.Context, ws pkgWorkspace.Context, stackName string,
	f *os.File, outPath string, sessionID string, sessionKey []byte, redact bool,
) error {
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
		stackName, cmdStack.LoadOnly, opts, "",
	)
	if err != nil {
		return fmt.Errorf("loading stack %q: %w", stackName, err)
	}

	sm, err := secretsManagerFromStack(ctx, s)
	if err != nil {
		return fmt.Errorf("getting secrets manager for stack %q: %w", stackName, err)
	}

	// Decrypt the entire log body.
	reader, err := encryptedlog.NewReader(ctx, f, sm.Decrypter())
	if err != nil {
		return fmt.Errorf("decrypting log: %w", err)
	}
	plaintext, err := io.ReadAll(reader)
	if err != nil {
		return fmt.Errorf("reading decrypted log: %w", err)
	}

	if redact {
		plaintext = redactSecretsInLog(plaintext)
	}

	return writeEncryptedLog(outPath, sessionID, sessionKey, plaintext)
}

// shareGzip decompresses a gzip log file, optionally redacts secrets,
// and encrypts the content with the service-provided session key.
func shareGzip(
	f *os.File, outPath string, sessionID string, sessionKey []byte, redact bool,
) error {
	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("log file is neither encrypted nor gzip-compressed: %w", err)
	}
	defer gz.Close()

	plaintext, err := io.ReadAll(gz)
	if err != nil {
		return fmt.Errorf("decompressing log: %w", err)
	}

	if redact {
		plaintext = redactSecretsInLog(plaintext)
	}

	return writeEncryptedLog(outPath, sessionID, sessionKey, plaintext)
}

// writeEncryptedLog creates a PLOG file encrypted with the given session
// key, storing the session ID in the header.
func writeEncryptedLog(outPath string, sessionID string, sessionKey []byte, plaintext []byte) error {
	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer outFile.Close()

	w, err := encryptedlog.NewWriterWithKey(outFile, sessionKey, []byte(sessionID))
	if err != nil {
		os.Remove(outPath)
		return fmt.Errorf("creating encrypted writer: %w", err)
	}
	if _, err := w.Write(plaintext); err != nil {
		os.Remove(outPath)
		return fmt.Errorf("writing shared log: %w", err)
	}
	if err := w.Close(); err != nil {
		os.Remove(outPath)
		return fmt.Errorf("closing shared log: %w", err)
	}
	return nil
}

// createEncryptionSession is a variable so tests can replace it with a mock.
var createEncryptionSession = createEncryptionSessionFromAPI

func createEncryptionSessionFromAPI(ctx context.Context, ws pkgWorkspace.Context) (sessionID string, sessionKey []byte, err error) {
	// Resolve the cloud URL without requiring login — this endpoint needs no auth.
	cloudURL := httpstate.ValueOrDefaultURL(ws, "")
	if cloudURL == "" {
		return "", nil, fmt.Errorf("could not determine Pulumi Cloud URL; set PULUMI_API or run `pulumi login`")
	}
	insecure := pkgWorkspace.GetCloudInsecure(ws, cloudURL)

	apiClient := client.NewClient(cloudURL, "" /*apiToken*/, insecure, cmdutil.Diag())
	resp, err := apiClient.CreateLogEncryptionSession(ctx, apitype.LogEncryptionSessionInitRequest{
		SessionKeyType: apitype.SessionKeyTypePlogV1,
	})
	if err != nil {
		return "", nil, fmt.Errorf("creating encryption session: %w", err)
	}

	keyBytes, err := base64.StdEncoding.DecodeString(resp.SessionKey)
	if err != nil {
		return "", nil, fmt.Errorf("decoding session key: %w", err)
	}

	return resp.SessionID, keyBytes, nil
}

// redactSecretsInLog processes each JSON line in the log and replaces
// secret property values with "[secret]".
func redactSecretsInLog(data []byte) []byte {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	var result bytes.Buffer
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			result.WriteByte('\n')
			continue
		}

		var rec map[string]any
		if err := json.Unmarshal(line, &rec); err != nil {
			// Not JSON — write through as-is.
			result.Write(line)
			result.WriteByte('\n')
			continue
		}

		redactSecretsInValue(rec)

		redacted, err := json.Marshal(rec)
		if err != nil {
			result.Write(line)
		} else {
			result.Write(redacted)
		}
		result.WriteByte('\n')
	}
	return result.Bytes()
}
