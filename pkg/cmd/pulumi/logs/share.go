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
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"errors"
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
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	cmdStack "github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/stack"
	"github.com/pulumi/pulumi/pkg/v3/engine/encryptedlog"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

func newShareCmd(
	ws pkgWorkspace.Context,
	stack *string,
	createEncryptionSession func(ctx context.Context, ws pkgWorkspace.Context) (string, []byte, error),
) *cobra.Command {
	var includeSecrets bool
	var latest bool

	cmd := &cobra.Command{
		Use:   "share",
		Short: "Re-encrypt a log file for sharing with Pulumi support",
		Long: "Create a copy of a log file that can be safely shared with\n" +
			"Pulumi support. The log content is re-encrypted with a key\n" +
			"that only Pulumi can read.\n" +
			"\n" +
			"If no filename is provided, a list of available log files is\n" +
			"displayed and the user is prompted to choose one. Pass\n" +
			"--latest to skip the prompt and share the most recent log\n" +
			"file instead.\n" +
			"\n" +
			"By default, secret values in the log are redacted. Use\n" +
			"--include-secrets to include them in the shared log.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			stackName := *stack

			var filename string
			if len(args) > 0 {
				filename = args[0]
			} else {
				var err error
				if latest {
					filterName := stackName
					if filterName == "" {
						filterName = currentStackName(ws)
					}
					filename, err = findLatestLog(filterName)
					if err != nil {
						return err
					}
					fmt.Fprintf(cmd.ErrOrStderr(), "Sharing %s\n", filename)
				} else {
					filename, err = chooseLog(stackName, "Select a log file to share:")
					if err != nil {
						return err
					}
				}
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

			fmt.Fprintf(cmd.ErrOrStderr(), "Shared log written to %s\n\n", outPath)
			fmt.Fprintf(cmd.ErrOrStderr(), "You can safely attach this file to a GitHub issue or\n")
			fmt.Fprintf(cmd.ErrOrStderr(), "send it to Pulumi support. Only Pulumi can decrypt it.\n")
			return nil
		},
	}

	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "filename"}},
		Required:  0,
	})

	cmd.Flags().BoolVar(&latest, "latest", false,
		"Share the most recent log file without prompting")
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

	var processed bytes.Buffer
	if err := formatLogRecords(bytes.NewReader(plaintext), &processed, redact); err != nil {
		return fmt.Errorf("processing log: %w", err)
	}

	return writeEncryptedLog(outPath, sessionID, sessionKey, processed.Bytes())
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

	var processed bytes.Buffer
	if err := formatLogRecords(bytes.NewReader(plaintext), &processed, redact); err != nil {
		return fmt.Errorf("processing log: %w", err)
	}

	return writeEncryptedLog(outPath, sessionID, sessionKey, processed.Bytes())
}

// writeEncryptedLog creates a PLOG file encrypted with the given session
// key, storing the session ID in the header.
func writeEncryptedLog(outPath string, sessionID string, sessionKey []byte, plaintext []byte) error {
	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer outFile.Close()

	key, err := encryptedlog.NewPreparedKey(sessionKey, []byte(sessionID))
	if err != nil {
		contract.IgnoreError(os.Remove(outPath))
		return fmt.Errorf("preparing encryption key: %w", err)
	}
	w, err := encryptedlog.NewWriterFromKey(outFile, key)
	if err != nil {
		contract.IgnoreError(os.Remove(outPath))
		return fmt.Errorf("creating encrypted writer: %w", err)
	}
	if _, err := w.Write(plaintext); err != nil {
		contract.IgnoreError(os.Remove(outPath))
		return fmt.Errorf("writing shared log: %w", err)
	}
	if err := w.Close(); err != nil {
		contract.IgnoreError(os.Remove(outPath))
		return fmt.Errorf("closing shared log: %w", err)
	}
	return nil
}

func createEncryptionSessionFromAPI(
	ctx context.Context, ws pkgWorkspace.Context,
) (sessionID string, sessionKey []byte, err error) {
	// Resolve the cloud URL without requiring login — this endpoint needs no auth.
	cloudURL := httpstate.ValueOrDefaultURL(ws, "")
	if cloudURL == "" {
		return "", nil, errors.New("could not determine Pulumi Cloud URL; set PULUMI_API or run `pulumi login`")
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
