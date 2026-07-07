// Copyright 2023, Pulumi Corporation.
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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newEnvOpenCmd(envcmd *envCommand) *cobra.Command {
	var duration time.Duration
	var format string
	var draft string

	cmd := &cobra.Command{
		Use:   "open [<org-name>/][<project-name>/]<environment-name>[@<version>] [property path]",
		Args:  cobra.MaximumNArgs(2),
		Short: "Open the environment with the given name.",
		Long: "Open the environment with the given name and return the result\n" +
			"\n" +
			"This command opens the environment with the given name. The result is written to\n" +
			"stdout as JSON. If a property path is specified, only retrieves that property.\n",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := envcmd.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, args, err := envcmd.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}

			var path resource.PropertyPath
			if len(args) == 1 {
				p, err := resource.ParsePropertyPath(args[0])
				if err != nil {
					return fmt.Errorf("invalid property path %v: %w", args[0], err)
				}
				path = p
			}

			if _, err := validateFormat(format, path); err != nil {
				return err
			}

			env, diags, err := envcmd.openEnvironment(ctx, ref, duration, draft)
			if err != nil {
				return err
			}
			if len(diags) != 0 {
				return envcmd.writePropertyEnvironmentDiagnostics(envcmd.esc.stderr, diags)
			}

			return envcmd.renderValue(envcmd.esc.stdout, env, path, format, false, true)
		},
	}

	cmd.Flags().DurationVarP(
		&duration, "lifetime", "l", 2*time.Hour,
		"the lifetime of the opened environment in the form HhMm (e.g. 2h, 1h30m, 15m)")
	cmd.Flags().StringVarP(
		&format, "format", "f", "json",
		formatFlagHelp("the output format to use. May be one of "))
	cmd.Flags().StringVar(
		&draft, "draft", "",
		"open an environment draft with --draft=<change-request-id>")

	return cmd
}

func (env *envCommand) renderValue(
	out io.Writer,
	e *esc.Environment,
	path resource.PropertyPath,
	format string,
	pretend bool,
	showSecrets bool,
) error {
	if e == nil {
		return nil
	}

	f, err := parseFormat(format)
	if err != nil {
		return err
	}

	if f.isProcess() {
		return env.renderProcessEnvironment(out, e, f.encoding, pretend, showSecrets)
	}

	val := esc.NewValue(e.Properties)
	if len(path) != 0 {
		if vv, ok := getEnvValue(val, path); ok {
			val = *vv
		} else {
			val = esc.Value{}
		}
	}

	switch f.encoding {
	case encodingJSON:
		body := val.ToJSON(!showSecrets)
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(body)
	case encodingYAML:
		body := val.ToJSON(!showSecrets)
		enc := yaml.NewEncoder(out)
		enc.SetIndent(3)
		return enc.Encode(body)
	case encodingJSONDetailed:
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(val)
	case encodingString:
		s := val.ToString(!showSecrets)
		if strings.HasSuffix(s, "\n") {
			fmt.Fprintf(out, "%v", s)
		} else {
			fmt.Fprintf(out, "%v\n", s)
		}
		return nil
	case encodingDotenv, encodingShell:
		return fmt.Errorf("unreachable: value object cannot be encoded as %q", format)
	default:
		return fmt.Errorf("unknown output format %q", format)
	}
}

// processEnvironmentEntry is one variable of the structured process-environment (process:json-detailed),
// pairing the value a process would see with the secret flag the flat dotenv/shell encodings discard.
type processEnvironmentEntry struct {
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
}

func (env *envCommand) renderProcessEnvironment(
	out io.Writer,
	e *esc.Environment,
	encoding outputEncoding,
	pretend bool,
	showSecrets bool,
) error {
	switch encoding {
	case encodingDotenv, encodingShell:
		_, environ, _, err := env.prepareEnvironment(
			e,
			PrepareOptions{Pretend: pretend, Quote: true, Redact: !showSecrets},
		)
		if err != nil {
			return err
		}
		prefix := ""
		if encoding == encodingShell {
			prefix = "export "
		}
		for _, kvp := range environ {
			fmt.Fprintf(out, "%s%v\n", prefix, kvp)
		}
		return nil
	case encodingJSON, encodingYAML:
		proj, err := env.projectEnvironment(e, PrepareOptions{Pretend: pretend})
		if err != nil {
			return err
		}
		body := make(map[string]string, len(proj.Variables)+len(proj.Files))
		for _, v := range proj.Variables {
			s := v.Value
			if v.Secret && !showSecrets {
				s = "[secret]"
			}
			body[v.Name] = s
		}
		for _, f := range proj.Files {
			body[f.Name] = f.Path
		}
		return encodeStructured(out, encoding, body)
	case encodingJSONDetailed:
		proj, err := env.projectEnvironment(e, PrepareOptions{Pretend: pretend})
		if err != nil {
			return err
		}
		body := make(map[string]processEnvironmentEntry, len(proj.Variables)+len(proj.Files))
		for _, v := range proj.Variables {
			body[v.Name] = processEnvironmentEntry{Value: v.Value, Secret: v.Secret}
		}
		for _, f := range proj.Files {
			body[f.Name] = processEnvironmentEntry{Value: f.Path, Secret: f.Secret}
		}
		return encodeStructured(out, encodingJSON, body)
	case encodingString:
		return errors.New("unreachable: process object cannot be encoded as a bare string")
	default:
		return errors.New("unknown process-environment encoding")
	}
}

func encodeStructured(out io.Writer, encoding outputEncoding, body any) error {
	if encoding == encodingYAML {
		enc := yaml.NewEncoder(out)
		enc.SetIndent(3)
		return enc.Encode(body)
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(body)
}

// prepareEnvironment prepares the envvar and temporary file projections for an environment. Returns the paths to
// temporary files, environment variable pairs, and secret values.
func (env *envCommand) prepareEnvironment(
	e *esc.Environment,
	opts PrepareOptions,
) (files, environ, secrets []string, err error) {
	opts.fs = env.esc.fs
	return PrepareEnvironment(e, &opts)
}

func (env *envCommand) projectEnvironment(e *esc.Environment, opts PrepareOptions) (*EnvironmentProjection, error) {
	opts.fs = env.esc.fs
	proj, err := projectEnvironment(e, &opts)
	if err != nil {
		return nil, fmt.Errorf("creating temporary files: %v", err)
	}
	return proj, nil
}

func (env *envCommand) removeTemporaryFiles(paths []string) {
	removeTemporaryFiles(env.esc.fs, paths)
}

func (env *envCommand) openEnvironment(
	ctx context.Context,
	ref environmentRef,
	duration time.Duration,
	changeRequestID string,
) (*esc.Environment, []client.EnvironmentDiagnostic, error) {
	var envID string
	var diags []client.EnvironmentDiagnostic
	var err error
	if changeRequestID == "" {
		envID, diags, err = env.esc.client.OpenEnvironment(
			ctx,
			ref.orgName,
			ref.projectName,
			ref.envName,
			ref.version,
			duration,
		)
	} else {
		envID, diags, err = env.esc.client.OpenEnvironmentDraft(ctx, ref.orgName, ref.projectName, ref.envName, changeRequestID, duration) //nolint:lll
	}
	if err != nil {
		return nil, nil, err
	}
	if len(diags) != 0 {
		return nil, diags, err
	}
	open, err := env.esc.client.GetOpenEnvironment(ctx, ref.orgName, ref.projectName, ref.envName, envID)
	return open, nil, err
}
