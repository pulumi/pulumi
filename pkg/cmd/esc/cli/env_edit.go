// Copyright 2023, Pulumi Corporation.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os/exec"
	"strings"
	"unicode"

	"github.com/spf13/cobra"

	"github.com/pulumi/esc"
	"github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

type envEditCommand struct {
	env *envCommand

	editorFlag string
}

func newEnvEditCmd(env *envCommand) *cobra.Command {
	var file string
	var showSecrets bool

	edit := &envEditCommand{env: env}

	cmd := &cobra.Command{
		Use:   "edit [<org-name>/][<project-name>/]<environment-name>",
		Args:  cobra.MaximumNArgs(1),
		Short: "Edit an environment definition",
		Long: "Edit an environment definition\n" +
			"\n" +
			"This command fetches the current definition for the named environment and opens it\n" +
			"for editing in an editor. The editor defaults to the value of the VISUAL environment\n" +
			"variable. If VISUAL is not set, EDITOR is used. These values are interpreted as\n" +
			"commands to which the name of the temporary file used for the environment is appended.\n" +
			"If no editor is specified via the --editor flag or environment variables, edit\n" +
			"defaults to `vi`.\n",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if err := env.esc.getCachedClient(ctx); err != nil {
				return err
			}

			ref, args, err := edit.env.getExistingEnvRef(ctx, args)
			if err != nil {
				return err
			}
			if ref.version != "" {
				return fmt.Errorf("the edit command does not accept versions")
			}
			_ = args

			if file != "" {
				var yaml []byte
				switch file {
				case "-":
					yaml, err = io.ReadAll(env.esc.stdin)
				default:
					yaml, err = fs.ReadFile(env.esc.fs, file)
				}
				if err != nil {
					return fmt.Errorf("reading environment definition: %w", err)
				}

				diags, err := edit.env.esc.client.UpdateEnvironmentWithProject(ctx, ref.orgName, ref.projectName, ref.envName, yaml, "")
				if err != nil {
					return fmt.Errorf("updating environment definition: %w", err)
				}
				if len(diags) == 0 {
					fmt.Fprintln(edit.env.esc.stdout, "Environment updated.")
					return nil
				}

				return edit.env.writeYAMLEnvironmentDiagnostics(edit.env.esc.stderr, ref.envName, yaml, diags)
			}

			editor, err := edit.getEditor()
			if err != nil {
				return err
			}

			yaml, tag, _, err := edit.env.esc.client.GetEnvironment(ctx, ref.orgName, ref.projectName, ref.envName, "", showSecrets)
			if err != nil {
				return fmt.Errorf("getting environment definition: %w", err)
			}

			var env *esc.Environment
			var diags []client.EnvironmentDiagnostic
			if len(yaml) != 0 {
				env, diags, _ = edit.env.esc.client.CheckYAMLEnvironment(ctx, ref.orgName, yaml)
			}

			for {
				newYAML, err := edit.editWithYAMLEditor(editor, ref.envName, yaml, env, diags)
				if err != nil {
					return err
				}
				if len(bytes.TrimSpace(newYAML)) == 0 {
					fmt.Fprintln(edit.env.esc.stderr, "Aborting edit due to empty definition.")
					return nil
				}

				diags, err = edit.env.esc.client.UpdateEnvironmentWithProject(ctx, ref.orgName, ref.projectName, ref.envName, newYAML, tag)
				if err != nil {
					return fmt.Errorf("updating environment definition: %w", err)
				}
				if len(diags) == 0 {
					fmt.Fprintln(edit.env.esc.stdout, "Environment updated.")
					return nil
				}

				err = edit.env.writeYAMLEnvironmentDiagnostics(edit.env.esc.stderr, ref.envName, newYAML, diags)
				contract.IgnoreError(err)

				fmt.Fprintln(edit.env.esc.stderr, "Press ENTER to continue editing or ^D to exit")

				var b [1]byte
				if _, err := edit.env.esc.stdin.Read(b[:]); err != nil {
					if errors.Is(err, io.EOF) {
						fmt.Fprintln(edit.env.esc.stderr, "Aborting edit.")
						return nil
					}
					return err
				}

				yaml = newYAML
			}
		},
	}

	cmd.Flags().StringVar(&edit.editorFlag, "editor", "", "the command to use to edit the environment definition")

	cmd.Flags().StringVarP(&file,
		"file", "f", "",
		"the file that contains the updated environment, if any. Pass `-` to read from standard input.")

	cmd.Flags().BoolVar(
		&showSecrets, "show-secrets", false,
		"Show static secrets in plaintext rather than ciphertext")

	return cmd
}

func parseEditorCommand(editor string) []string {
	var command []string
	for {
		editor = strings.TrimLeftFunc(editor, unicode.IsSpace)
		if len(editor) == 0 {
			return command
		}

		var arg strings.Builder
		i, quoted := 0, false
		for ; i < len(editor); i++ {
			c := editor[i]
			if c == '"' {
				quoted = !quoted
				continue
			}

			if !quoted && unicode.IsSpace(rune(c)) {
				break
			} else if i+1 < len(editor) && c == '\\' && editor[i+1] == '"' {
				arg.WriteByte('"')
				i++
			} else {
				arg.WriteByte(c)
			}
		}

		command = append(command, arg.String())
		editor = editor[i:]
	}
}

func (edit *envEditCommand) getEditor() ([]string, error) {
	editor := edit.editorFlag

	if editor == "" {
		editor = edit.env.esc.environ.Get("VISUAL")
		if editor == "" {
			editor = edit.env.esc.environ.Get("EDITOR")
		}
	}

	args := parseEditorCommand(editor)
	if len(args) == 0 {
		path, err := edit.env.esc.exec.LookPath("vi")
		if err != nil {
			return nil, errors.New("No available editor. Please use the --editor flag or set one of the " +
				"VISUAL or EDITOR environment variables.")
		}
		args = []string{path}
		return args, nil
	}

	// Automatically add -w to 'code' if it is not present.
	if args[0] == "code" && len(args) == 1 {
		args = append(args, "-w")
	}

	path, err := edit.env.esc.exec.LookPath(args[0])
	if err != nil {
		return nil, fmt.Errorf("finding %q on path: %w", args[0], err)
	}

	args[0] = path
	return args, nil
}

func (edit *envEditCommand) editWithYAMLEditor(
	editor []string,
	envName string,
	yaml []byte,
	checked *esc.Environment,
	diags []client.EnvironmentDiagnostic,
) ([]byte, error) {
	var details bytes.Buffer
	if len(diags) != 0 {
		var tmp bytes.Buffer
		fmt.Fprintln(&tmp, "# Diagnostics")
		fmt.Fprintln(&tmp, "")
		err := edit.env.writeYAMLEnvironmentDiagnostics(&tmp, envName, yaml, diags)
		contract.IgnoreError(err)

		fmt.Fprintln(&details, "---")
		fmt.Fprint(&details, strings.ReplaceAll(tmp.String(), "\n", "\n# "))
	}
	if checked != nil {
		fmt.Fprintln(&details, "---")
		fmt.Fprintln(&details, "# Please edit the environment definition above.")
		fmt.Fprintln(&details, "# The object below is the current result of")
		fmt.Fprintln(&details, "# evaluating the environment and will not be")
		fmt.Fprintln(&details, "# saved. An empty definition aborts the edit.")
		fmt.Fprintln(&details, "")

		enc := json.NewEncoder(&details)
		enc.SetIndent("", "  ")
		err := enc.Encode(esc.NewValue(checked.Properties).ToJSON(false))
		contract.IgnoreError(err)
	}

	filename, err := func() (string, error) {
		filename, f, err := edit.env.esc.fs.CreateTemp("", "*.yaml")
		if err != nil {
			return "", err
		}
		defer contract.IgnoreClose(f)

		if _, err = f.Write(yaml); err != nil {
			rmErr := edit.env.esc.fs.Remove(filename)
			contract.IgnoreError(rmErr)
			return "", err
		}

		if details.Len() != 0 {
			if len(yaml) != 0 && yaml[len(yaml)-1] != '\n' {
				fmt.Fprintln(f, "")
			}

			if _, err = f.Write(details.Bytes()); err != nil {
				rmErr := edit.env.esc.fs.Remove(filename)
				contract.IgnoreError(rmErr)
				return "", err
			}
		}

		return filename, nil
	}()
	if err != nil {
		return nil, fmt.Errorf("writing temporary file: %w", err)
	}
	defer func() {
		err := edit.env.esc.fs.Remove(filename)
		contract.IgnoreError(err)
	}()

	//nolint:gosec
	cmd := exec.Command(editor[0], append(editor[1:], filename)...)
	cmd.Stdin = edit.env.esc.stdin
	cmd.Stdout = edit.env.esc.stdout
	cmd.Stderr = edit.env.esc.stderr
	if err := edit.env.esc.exec.Run(cmd); err != nil {
		return nil, fmt.Errorf("editor: %w", err)
	}

	new, err := fs.ReadFile(edit.env.esc.fs, filename)
	if err != nil {
		return nil, fmt.Errorf("reading temporary file: %w", err)
	}

	sep := bytes.Index(new, []byte("---"))
	if sep != -1 {
		isDocSep := true
		if sep+len("---") < len(new) && new[sep+len("---")] != '\n' {
			isDocSep = false
		}
		if sep != 0 && new[sep-1] != '\n' {
			isDocSep = false
		}
		if isDocSep {
			new = new[:sep]
		}
	}

	return new, nil
}
