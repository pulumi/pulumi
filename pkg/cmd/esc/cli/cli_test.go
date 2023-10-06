// Copyright 2023, Pulumi Corporation.

package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gofrs/uuid"
	"github.com/pulumi/esc"
	"github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/pulumi/esc/eval"
	"github.com/pulumi/esc/schema"
	"github.com/pulumi/esc/syntax"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
	"gopkg.in/yaml.v3"
	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	shsyntax "mvdan.cc/sh/v3/syntax"
)

func accept() bool {
	return cmdutil.IsTruthy(os.Getenv("PULUMI_ACCEPT"))
}

type testFile struct {
	f      *fstest.MapFile
	offset int64
}

func (f *testFile) Close() error {
	return nil
}

func (f *testFile) Read(p []byte) (int, error) {
	if f.offset >= int64(len(f.f.Data)) {
		return 0, io.EOF
	}
	n := copy(p, f.f.Data[f.offset:])
	f.offset += int64(n)
	return n, nil
}

func (f *testFile) Write(p []byte) (int, error) {
	if delta := f.offset + int64(len(p)) - int64(len(f.f.Data)); delta > 0 {
		f.f.Data = append(f.f.Data, make([]byte, delta)...)
	}
	n := copy(f.f.Data[f.offset:], p)
	f.offset += int64(n)
	return n, nil
}

type testFS struct {
	fstest.MapFS
}

func (tfs testFS) MkdirAll(name string, perm fs.FileMode) error {
	if path.Dir(name) == "/" {
		return nil
	}
	if err := tfs.MkdirAll(path.Base(name), perm); err != nil {
		return err
	}
	tfs.MapFS[name] = &fstest.MapFile{
		Mode: perm | fs.ModeDir,
	}
	return nil
}

func (tfs testFS) LockedRead(name string) ([]byte, error) {
	return tfs.ReadFile(name)
}

func (tfs testFS) LockedWrite(name string, content io.Reader, perm os.FileMode) error {
	data, err := io.ReadAll(content)
	if err != nil {
		return err
	}
	tfs.MapFS[name] = &fstest.MapFile{
		Data: data,
		Mode: perm,
	}
	return nil
}

func (tfs testFS) CreateTemp(dir, pattern string) (string, io.ReadWriteCloser, error) {
	if dir == "" {
		dir = "temp"
	}
	name := path.Join(dir, strings.ReplaceAll(pattern, "*", "temp"))

	f := &fstest.MapFile{Mode: 0o600}
	tfs.MapFS[name] = f
	return name, &testFile{f: f}, nil
}

func (tfs testFS) Remove(name string) error {
	f, err := tfs.Stat(name)
	if err != nil {
		return err
	}
	delete(tfs.MapFS, f.Name())
	return nil
}

type testEnviron map[string]string

func (env testEnviron) Get(key string) string {
	return env[key]
}

func (env testEnviron) Vars() []string {
	var vars []string
	for k, v := range env {
		vars = append(vars, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(vars)
	return vars
}

type testPulumiWorkspace struct {
	credentials workspace.Credentials
}

func (*testPulumiWorkspace) SetBackendConfigDefaultOrg(backendURL, defaultOrg string) error {
	return nil
}

func (*testPulumiWorkspace) GetPulumiConfig() (workspace.PulumiConfig, error) {
	return workspace.PulumiConfig{}, nil
}

func (*testPulumiWorkspace) GetPulumiPath(elem ...string) (string, error) {
	return path.Join(append([]string{"/pulumi"}, elem...)...), nil
}

func (w *testPulumiWorkspace) GetStoredCredentials() (workspace.Credentials, error) {
	return w.credentials, nil
}

func (w *testPulumiWorkspace) StoreAccount(key string, account workspace.Account, current bool) error {
	w.credentials.Accounts[key] = account
	if current {
		w.credentials.Current = key
	}
	return nil
}

func (w *testPulumiWorkspace) GetAccount(key string) (workspace.Account, error) {
	return w.credentials.Accounts[key], nil
}

type testProvider struct{}

func (testProvider) Schema() (*schema.Schema, *schema.Schema) {
	return schema.Always(), schema.Always()
}

func (testProvider) Open(ctx context.Context, inputs map[string]esc.Value) (esc.Value, error) {
	return esc.NewValue(inputs), nil
}

type testProviders struct{}

func (testProviders) LoadProvider(ctx context.Context, name string) (esc.Provider, error) {
	if name == "test" {
		return testProvider{}, nil
	}
	return nil, fmt.Errorf("unknown provider %q", name)
}

type testEnvironments struct {
	orgName      string
	environments map[string]*testEnvironment
}

func (e *testEnvironments) LoadEnvironment(ctx context.Context, envName string) ([]byte, error) {
	name := path.Join(e.orgName, envName)
	env, ok := e.environments[name]
	if !ok {
		return nil, errors.New("not found")
	}
	return env.yaml, nil
}

type testEnvironment struct {
	yaml []byte
	tag  string
}

type testPulumiClient struct {
	user         string
	environments map[string]*testEnvironment
	openEnvs     map[string]*esc.Environment
}

type testLoginManager struct {
	creds workspace.Credentials
}

// Current returns the current cloud backend if one is already logged in.
func (lm *testLoginManager) Current(ctx context.Context, cloudURL string, insecure, setCurrent bool) (*workspace.Account, error) {
	acct, ok := lm.creds.Accounts[lm.creds.Current]
	if !ok {
		return nil, errors.New("unauthorized")
	}
	return &acct, nil
}

// Login logs into the target cloud URL and returns the cloud backend for it.
func (lm *testLoginManager) Login(
	ctx context.Context,
	cloudURL string,
	insecure bool,
	command string,
	message string,
	welcome func(display.Options),
	current bool,
	opts display.Options,
) (*workspace.Account, error) {
	acct, ok := lm.creds.Accounts[cloudURL]
	if !ok {
		return nil, errors.New("unauthorized")
	}
	return &acct, nil
}

func mapDiags(diags syntax.Diagnostics) []client.EnvironmentDiagnostic {
	if len(diags) == 0 {
		return nil
	}
	out := make([]client.EnvironmentDiagnostic, len(diags))
	for i, d := range diags {
		var rng *esc.Range
		if d.Subject != nil {
			rng = &esc.Range{
				Environment: d.Subject.Filename,
				Begin: esc.Pos{
					Line:   d.Subject.Start.Line,
					Column: d.Subject.Start.Column,
					Byte:   d.Subject.Start.Byte,
				},
				End: esc.Pos{
					Line:   d.Subject.End.Line,
					Column: d.Subject.End.Column,
					Byte:   d.Subject.End.Byte,
				},
			}
		}

		out[i] = client.EnvironmentDiagnostic{
			Range:   rng,
			Summary: d.Summary,
			Detail:  d.Detail,
		}
	}
	return out
}

func (c *testPulumiClient) checkEnvironment(ctx context.Context, orgName, envName string, yaml []byte) (*esc.Environment, []client.EnvironmentDiagnostic, error) {
	environment, diags, err := eval.LoadYAMLBytes(envName, yaml)
	if err != nil {
		return nil, nil, fmt.Errorf("loading environment: %w", err)
	}
	if diags.HasErrors() {
		return nil, mapDiags(diags), nil
	}

	providers := &testProviders{}
	envLoader := &testEnvironments{orgName: orgName, environments: c.environments}

	checked, checkDiags := eval.CheckEnvironment(ctx, envName, environment, providers, envLoader)
	diags.Extend(checkDiags...)
	return checked, mapDiags(diags), nil
}

func (c *testPulumiClient) openEnvironment(ctx context.Context, orgName, name string, yaml []byte) (string, []client.EnvironmentDiagnostic, error) {
	id, err := uuid.NewV4()
	if err != nil {
		return "", nil, err
	}

	decl, diags, err := eval.LoadYAMLBytes(name, yaml)
	if err != nil {
		return "", nil, fmt.Errorf("loading environment: %w", err)
	}
	if diags.HasErrors() {
		return "", mapDiags(diags), nil
	}

	providers := &testProviders{}
	envLoader := &testEnvironments{orgName: orgName, environments: c.environments}

	openEnv, evalDiags := eval.EvalEnvironment(ctx, name, decl, providers, envLoader)
	diags.Extend(evalDiags...)

	if diags.HasErrors() {
		return "", mapDiags(diags), nil
	}

	c.openEnvs[id.String()] = openEnv
	return id.String(), mapDiags(diags), nil
}

// Returns true if this client is insecure (i.e. has TLS disabled).
func (c *testPulumiClient) Insecure() bool {
	return true
}

// URL returns the URL of the API endpoint this client interacts with
func (c *testPulumiClient) URL() string {
	return "http://fake.pulumi.api"
}

// GetPulumiAccountDetails returns the user implied by the API token associated with this client.
func (c *testPulumiClient) GetPulumiAccountDetails(ctx context.Context) (string, []string, *workspace.TokenInformation, error) {
	return c.user, nil, nil, nil
}

func (c *testPulumiClient) ListEnvironments(
	ctx context.Context,
	orgName string,
	continuationToken string,
) ([]client.OrgEnvironment, string, error) {
	names := maps.Keys(c.environments)
	sort.Strings(names)

	var envs []client.OrgEnvironment
	for _, k := range names {
		org, name, _ := strings.Cut(k, "/")

		if orgName == "" || org == orgName {
			envs = append(envs, client.OrgEnvironment{
				Organization: org,
				Name:         name,
			})
		}
	}

	offset := uint(0)
	if continuationToken != "" {
		o, err := strconv.ParseUint(continuationToken, 10, 0)
		if err != nil {
			return nil, "", errors.New("invalid continuation token")
		}
		offset = uint(o)
	}

	if offset >= uint(len(envs)) {
		return nil, "", nil
	}
	return envs[offset : offset+1], strconv.FormatUint(uint64(offset+1), 10), nil
}

func (c *testPulumiClient) CreateEnvironment(ctx context.Context, orgName, envName string) error {
	name := path.Join(orgName, envName)
	if _, ok := c.environments[name]; ok {
		return errors.New("already exists")
	}
	c.environments[name] = &testEnvironment{}
	return nil
}

func (c *testPulumiClient) GetEnvironment(ctx context.Context, orgName, envName string) ([]byte, string, error) {
	name := path.Join(orgName, envName)
	env, ok := c.environments[name]
	if !ok {
		return nil, "", errors.New("not found")
	}
	return env.yaml, env.tag, nil
}

func (c *testPulumiClient) UpdateEnvironment(
	ctx context.Context,
	orgName string,
	envName string,
	yaml []byte,
	tag string,
) ([]client.EnvironmentDiagnostic, error) {
	name := path.Join(orgName, envName)
	env, ok := c.environments[name]
	if !ok {
		return nil, errors.New("not found")
	}
	if tag != "" && tag != env.tag {
		return nil, errors.New("tag mismatch")
	}

	_, diags, err := c.checkEnvironment(ctx, orgName, envName, yaml)
	if err == nil && len(diags) == 0 {
		env.yaml = yaml
		env.tag = base64.StdEncoding.EncodeToString(fnv.New32().Sum(yaml))
	}

	return diags, err
}

func (c *testPulumiClient) DeleteEnvironment(ctx context.Context, orgName, envName string) error {
	name := path.Join(orgName, envName)
	if _, ok := c.environments[name]; !ok {
		return errors.New("not found")
	}
	delete(c.environments, name)
	return nil
}

func (c *testPulumiClient) OpenEnvironment(
	ctx context.Context,
	orgName string,
	envName string,
	duration time.Duration,
) (string, []client.EnvironmentDiagnostic, error) {
	name := path.Join(orgName, envName)
	env, ok := c.environments[name]
	if !ok {
		return "", nil, errors.New("not found")
	}

	return c.openEnvironment(ctx, orgName, envName, env.yaml)
}

func (c *testPulumiClient) CheckYAMLEnvironment(
	ctx context.Context,
	orgName string,
	yaml []byte,
) (*esc.Environment, []client.EnvironmentDiagnostic, error) {
	return c.checkEnvironment(ctx, orgName, "<yaml>", yaml)
}

func (c *testPulumiClient) OpenYAMLEnvironment(
	ctx context.Context,
	orgName string,
	yaml []byte,
	duration time.Duration,
) (string, []client.EnvironmentDiagnostic, error) {
	return c.openEnvironment(ctx, orgName, "<yaml>", yaml)
}

func (c *testPulumiClient) GetOpenEnvironment(ctx context.Context, orgName, envName, openEnvID string) (*esc.Environment, error) {
	env, ok := c.openEnvs[openEnvID]
	if !ok {
		return nil, errors.New("not found")
	}
	return env, nil
}

func (c *testPulumiClient) GetOpenProperty(ctx context.Context, orgName, envName, openEnvID, property string) (*esc.Value, error) {
	return nil, errors.New("NYI")
}

type testExec struct {
	fs       testFS
	environ  map[string]string
	commands map[string]string

	parentPath string
	login      *testLoginManager
	workspace  *testPulumiWorkspace
	client     *testPulumiClient
}

func (c *testExec) LookPath(cmd string) (string, error) {
	_, ok := c.commands[cmd]
	if !ok {
		return "", errors.New("command not found")
	}
	return cmd, nil
}

func (c *testExec) Run(cmd *exec.Cmd) error {
	script, ok := c.commands[filepath.Base(cmd.Path)]
	if !ok {
		return errors.New("command not found")
	}
	return c.runScript(script, cmd)
}

func (c *testExec) runScript(script string, cmd *exec.Cmd) error {
	file, err := shsyntax.NewParser().Parse(strings.NewReader(script), cmd.Path)
	if err != nil {
		return err
	}

	runner, err := interp.New(
		interp.ExecHandlers(func(_ interp.ExecHandlerFunc) interp.ExecHandlerFunc {
			return func(ctx context.Context, args []string) error {
				if args[0] != valueOrDefault(c.parentPath, "esc") {
					return errors.New("unknown command")
				}

				hc := interp.HandlerCtx(ctx)

				environ := testEnviron{}
				for k, v := range c.environ {
					environ[k] = v
				}
				hc.Env.Each(func(name string, vr expand.Variable) bool {
					environ[name] = vr.String()
					return true
				})

				esc := New(&Options{
					ParentPath:      c.parentPath,
					Stdin:           hc.Stdin,
					Stdout:          hc.Stdout,
					Stderr:          hc.Stderr,
					Colors:          colors.Never,
					Login:           c.login,
					PulumiWorkspace: c.workspace,
					fs:              c.fs,
					environ:         environ,
					exec:            c,
					newClient: func(_, backendURL, accessToken string, insecure bool) client.Client {
						return c.client
					},
				})
				if c.parentPath != "" {
					parent := &cobra.Command{
						Use: c.parentPath,
					}
					parent.AddCommand(esc)
					esc = parent
				}

				fmt.Fprintf(hc.Stdout, "> %v\n", strings.Join(args, " "))

				esc.SetArgs(args[1:])
				esc.SetIn(hc.Stdin)
				esc.SetOut(hc.Stdout)
				esc.SetErr(hc.Stderr)
				if err := esc.Execute(); err != nil {
					return interp.NewExitStatus(1)
				}
				return nil
			}
		}),
		interp.OpenHandler(func(ctx context.Context, path string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
			f, ok := c.fs.MapFS[path]
			if !ok {
				if flag&os.O_CREATE == 0 {
					return nil, os.ErrNotExist
				}
				f = &fstest.MapFile{Mode: perm}
				c.fs.MapFS[path] = &fstest.MapFile{Mode: perm}
			}
			if flag&os.O_TRUNC != 0 {
				f.Data = f.Data[:0]
			}
			return &testFile{f: f}, nil
		}),
		interp.ReadDirHandler(func(ctx context.Context, path string) ([]os.FileInfo, error) {
			return nil, errors.New("not supported")
		}),
		interp.StatHandler(func(ctx context.Context, name string, followSymlinks bool) (os.FileInfo, error) {
			return nil, errors.New("not supported")
		}),
		interp.StdIO(cmd.Stdin, cmd.Stdout, cmd.Stderr),
		interp.Env(expand.ListEnviron(cmd.Env...)),
		interp.Params(append([]string{"-e", "-u", "--"}, cmd.Args[1:]...)...),
	)
	if err != nil {
		return err
	}

	return runner.Run(context.Background(), file)
}

type cliTestcaseProcess struct {
	FS       map[string]string `yaml:"fs,omitempty"`
	Environ  map[string]string `yaml:"environ,omitempty"`
	Commands map[string]string `yaml:"commands,omitempty"`
}

type cliTestcaseYAML struct {
	Parent string `yaml:"parent,omitempty"`

	Run   string `yaml:"run,omitempty"`
	Error string `yaml:"error,omitempty"`

	Credentials *workspace.Credentials `yaml:"credentials,omitempty"`

	Process *cliTestcaseProcess `yaml:"process,omitempty"`

	Environments map[string]yaml.Node `yaml:"environments,omitempty"`

	Stdout string `yaml:"stdout,omitempty"`
	Stderr string `yaml:"stderr,omitempty"`
}

type cliTestcase struct {
	exec *testExec

	script         string
	expectedStdout string
	expectedStderr string
}

func loadTestcase(path string) (*cliTestcaseYAML, *cliTestcase, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer contract.IgnoreClose(f)

	var testcase cliTestcaseYAML
	if err := yaml.NewDecoder(f).Decode(&testcase); err != nil {
		return nil, nil, err
	}

	fs := testFS{MapFS: fstest.MapFS{}}
	exec := testExec{fs: fs}

	if testcase.Process != nil {
		for k, v := range testcase.Process.FS {
			fs.MapFS[k] = &fstest.MapFile{
				Data: []byte(v),
				Mode: 0o600,
			}
		}

		exec.environ = testcase.Process.Environ
		exec.commands = testcase.Process.Commands
	}

	environments := map[string]*testEnvironment{}
	for k, env := range testcase.Environments {
		bytes, err := yaml.Marshal(env)
		if err != nil {
			return nil, nil, err
		}

		environments[k] = &testEnvironment{yaml: bytes}
	}

	var creds workspace.Credentials
	if testcase.Credentials == nil {
		creds = workspace.Credentials{
			Current: "http://fake.pulumi.api",
			Accounts: map[string]workspace.Account{
				"http://fake.pulumi.api": {
					Username:    "test-user",
					AccessToken: "access-token",
				},
			},
		}
	}

	exec.parentPath = testcase.Parent
	exec.workspace = &testPulumiWorkspace{credentials: creds}
	exec.login = &testLoginManager{creds: creds}
	exec.client = &testPulumiClient{
		user:         "test-user",
		environments: environments,
		openEnvs:     map[string]*esc.Environment{},
	}

	return &testcase, &cliTestcase{
		exec:           &exec,
		script:         testcase.Run,
		expectedStdout: testcase.Stdout,
		expectedStderr: testcase.Stderr,
	}, nil
}

func TestCLI(t *testing.T) {
	path := filepath.Join("testdata")
	entries, err := os.ReadDir(path)
	require.NoError(t, err)
	for _, e := range entries {
		t.Run(e.Name(), func(t *testing.T) {
			path := filepath.Join(path, e.Name())
			def, testcase, err := loadTestcase(path)
			require.NoError(t, err)

			var stdout, stderr bytes.Buffer

			err = testcase.exec.runScript(testcase.script, &exec.Cmd{
				Path:   "<script>",
				Args:   []string{"<script>"},
				Stdin:  bytes.NewReader(nil),
				Stdout: &stdout,
				Stderr: &stderr,
			})

			if accept() {
				if err != nil {
					def.Error = err.Error()
				} else {
					def.Error = ""
				}

				def.Stdout = stdout.String()
				def.Stderr = stderr.String()

				var b bytes.Buffer
				enc := yaml.NewEncoder(&b)
				enc.SetIndent(2)
				err := enc.Encode(def)
				require.NoError(t, err)

				err = os.WriteFile(path, b.Bytes(), 0o600)
				require.NoError(t, err)

				return
			}

			if def.Error == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
			}

			assert.Equal(t, testcase.expectedStdout, stdout.String())
			assert.Equal(t, testcase.expectedStderr, stderr.String())
		})
	}
}
