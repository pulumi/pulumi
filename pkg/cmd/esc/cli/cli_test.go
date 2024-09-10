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
	"net/http"
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
	"github.com/pgavlin/fx"
	"github.com/pulumi/esc"
	"github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/pulumi/esc/eval"
	"github.com/pulumi/esc/schema"
	"github.com/pulumi/esc/syntax"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
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
	if err := tfs.MkdirAll(path.Dir(name), perm); err != nil {
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

	for i := 0; ; i++ {
		name := path.Join(dir, strings.ReplaceAll(pattern, "*", fmt.Sprintf("temp-%v", i)))
		if _, ok := tfs.MapFS[name]; !ok {
			f := &fstest.MapFile{Mode: 0o600}
			tfs.MapFS[name] = f
			return name, &testFile{f: f}, nil
		}
	}
}

func (tfs testFS) Remove(name string) error {
	_, err := tfs.Stat(name)
	if err != nil {
		return err
	}
	delete(tfs.MapFS, name)
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

func (w *testPulumiWorkspace) DeleteAccount(backendURL string) error {
	delete(w.credentials.Accounts, backendURL)
	if w.credentials.Current == backendURL {
		w.credentials.Current = ""
	}
	return nil
}

func (w *testPulumiWorkspace) DeleteAllAccounts() error {
	w.credentials.Accounts = map[string]workspace.Account{}
	w.credentials.Current = ""
	return nil
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

func (testProvider) Open(ctx context.Context, inputs map[string]esc.Value, context esc.EnvExecContext) (esc.Value, error) {
	return esc.NewValue(inputs), nil
}

type testProviders struct{}

func (testProviders) LoadProvider(ctx context.Context, name string) (esc.Provider, error) {
	if name == "test" {
		return testProvider{}, nil
	}
	return nil, fmt.Errorf("unknown provider %q", name)
}

type rot128 struct{}

func (rot128) Encrypt(_ context.Context, plaintext []byte) ([]byte, error) {
	for i, b := range plaintext {
		plaintext[i] = b + 128
	}
	return plaintext, nil
}

func (rot128) Decrypt(_ context.Context, plaintext []byte) ([]byte, error) {
	for i, b := range plaintext {
		plaintext[i] = b + 128
	}
	return plaintext, nil
}

type testEnvironments struct {
	orgName      string
	environments map[string]*testEnvironment
}

func (e *testEnvironments) LoadEnvironment(ctx context.Context, ref string) ([]byte, eval.Decrypter, error) {
	var name string

	// This "emulates" the backend behavior of resolving refs
	if strings.Contains(ref, "/") {
		name = path.Join(e.orgName, ref)
	} else {
		// If ref is a single identifier, assume the default project
		name = path.Join(e.orgName, client.DefaultProject, ref)
	}

	env, ok := e.environments[name]
	if !ok {
		return nil, nil, errors.New("not found")
	}
	return env.latest().yaml, rot128{}, nil
}

type testEnvironmentRetract struct {
	replacement int
	reason      string
}

type testEnvironmentRevision struct {
	number    int
	yaml      []byte
	tags      map[string]bool
	retracted *testEnvironmentRetract
}

type testEnvironment struct {
	revisions    []*testEnvironmentRevision
	revisionTags map[string]int
	tags         map[string]string
}

func (env *testEnvironment) latest() *testEnvironmentRevision {
	return env.revisions[len(env.revisions)-1]
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
	if lm.creds.Current == "" {
		return nil, nil
	}

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
		if cloudURL != "https://api.pulumi.com" {
			return nil, errors.New("unauthorized")
		}
		acct := workspace.Account{
			Username:    "test-user",
			AccessToken: "access-token",
		}
		lm.creds.Accounts[cloudURL] = acct
		return &acct, nil
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

func (c *testPulumiClient) getEnvironment(orgName, projectName, envName, version string) (*testEnvironment, *testEnvironmentRevision, error) {
	name := path.Join(orgName, projectName, envName)

	env, ok := c.environments[name]
	if !ok {
		return nil, nil, errors.New("not found")
	}

	var revision int
	if version == "" || version == "latest" {
		revision = len(env.revisions)
	} else if version[0] >= '0' && version[0] <= '9' {
		rev, err := strconv.ParseInt(version, 10, 0)
		if err != nil || rev < 1 || rev > int64(len(env.revisions)) {
			return nil, nil, errors.New("not found")
		}
		revision = int(rev)
	} else {
		rev, ok := env.revisionTags[version]
		if !ok {
			return nil, nil, errors.New("not found")
		}
		revision = rev
	}

	return env, env.revisions[revision-1], nil
}

func (c *testPulumiClient) checkEnvironment(ctx context.Context, orgName, envName string, yaml []byte, opts []client.CheckYAMLOption) (*esc.Environment, []client.EnvironmentDiagnostic, error) {
	environment, diags, err := eval.LoadYAMLBytes(envName, yaml)
	if err != nil {
		return nil, nil, fmt.Errorf("loading environment: %w", err)
	}
	if diags.HasErrors() {
		return nil, mapDiags(diags), nil
	}

	providers := &testProviders{}
	envLoader := &testEnvironments{orgName: orgName, environments: c.environments}

	execContext, err := esc.NewExecContext(make(map[string]esc.Value))
	if err != nil {
		return nil, nil, fmt.Errorf("initializing the ESC exec context: %w", err)
	}

	showSecrets := false
	if len(opts) > 0 {
		showSecrets = opts[0].ShowSecrets
	}

	checked, checkDiags := eval.CheckEnvironment(ctx, envName, environment, rot128{}, providers, envLoader, execContext, showSecrets)
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

	execContext, err := esc.NewExecContext(make(map[string]esc.Value))
	if err != nil {
		return "", nil, fmt.Errorf("initializing the ESC exec context: %w", err)
	}

	openEnv, evalDiags := eval.EvalEnvironment(ctx, name, decl, rot128{}, providers, envLoader, execContext)
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

func (c *testPulumiClient) GetRevisionNumber(ctx context.Context, orgName, projectName, envName, version string) (int, error) {
	_, rev, err := c.getEnvironment(orgName, projectName, envName, version)
	if err != nil {
		return 0, err
	}
	return rev.number, nil

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
		parts := strings.Split(k, "/")
		org, projectName, envName := parts[0], parts[1], parts[2]

		if orgName == "" || org == orgName {
			envs = append(envs, client.OrgEnvironment{
				Organization: org,
				Project:      projectName,
				Name:         envName,
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
	return c.CreateEnvironmentWithProject(ctx, orgName, client.DefaultProject, envName)
}

func (c *testPulumiClient) CreateEnvironmentWithProject(ctx context.Context, orgName, projectName, envName string) error {
	name := path.Join(orgName, projectName, envName)
	if _, ok := c.environments[name]; ok {
		return errors.New("already exists")
	}
	c.environments[name] = &testEnvironment{
		revisions:    []*testEnvironmentRevision{{}},
		revisionTags: map[string]int{"latest": 0},
		tags:         map[string]string{},
	}
	return nil
}

func (c *testPulumiClient) CloneEnvironment(
	ctx context.Context,
	orgName, projectName, envName string,
	destEnv client.CloneEnvironmentRequest,
) error {
	srcEnvName := path.Join(orgName, projectName, envName)
	srcEnv, ok := c.environments[srcEnvName]
	if !ok {
		return errors.New("source env not found")
	}
	if destEnv.Project == "" {
		destEnv.Project = projectName
	}
	destEnvName := path.Join(orgName, destEnv.Project, destEnv.Name)
	if _, ok := c.environments[destEnvName]; ok {
		return errors.New("already exists")
	}
	testDestEnv := &testEnvironment{
		revisions: []*testEnvironmentRevision{srcEnv.revisions[len(srcEnv.revisions)-1]},
	}
	if destEnv.PreserveHistory {
		testDestEnv.revisions = srcEnv.revisions
	}
	if destEnv.PreserveEnvironmentTags {
		testDestEnv.tags = srcEnv.tags
	}
	if destEnv.PreserveRevisionTags {
		testDestEnv.revisionTags = srcEnv.revisionTags
	}
	c.environments[destEnvName] = testDestEnv
	return nil
}

func (c *testPulumiClient) GetEnvironment(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	version string,
	showSecrets bool,
) ([]byte, string, int, error) {
	_, env, err := c.getEnvironment(orgName, projectName, envName, version)
	if err != nil {
		return nil, "", 0, err
	}

	yaml := env.yaml
	if showSecrets {
		plaintext, err := eval.DecryptSecrets(ctx, envName, yaml, rot128{})
		if err != nil {
			return nil, "", 0, err
		}
		yaml = plaintext
	}

	eTag := ""
	if len(env.tags) > 0 {
		eTag = maps.Keys(env.tags)[0]
	}

	return yaml, eTag, env.number, nil
}

func (c *testPulumiClient) UpdateEnvironment(
	ctx context.Context,
	orgName string,
	envName string,
	yaml []byte,
	tag string,
) ([]client.EnvironmentDiagnostic, error) {
	return c.UpdateEnvironmentWithProject(ctx, orgName, client.DefaultProject, envName, yaml, tag)
}

func (c *testPulumiClient) UpdateEnvironmentWithProject(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	yaml []byte,
	tag string,
) ([]client.EnvironmentDiagnostic, error) {
	diags, _, err := c.UpdateEnvironmentWithRevision(ctx, orgName, projectName, envName, yaml, tag)
	return diags, err
}

func (c *testPulumiClient) UpdateEnvironmentWithRevision(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	yaml []byte,
	tag string,
) ([]client.EnvironmentDiagnostic, int, error) {
	env, latest, err := c.getEnvironment(orgName, projectName, envName, "")
	if err != nil {
		return nil, 0, err
	}

	latestRevHasTag := latest.tags[tag]
	if tag != "" && !latestRevHasTag {
		return nil, 0, errors.New("tag mismatch")
	}

	_, diags, err := c.checkEnvironment(ctx, orgName, envName, yaml, nil)
	if err == nil && len(diags) == 0 {
		h := fnv.New32()
		h.Write(yaml)

		yaml, err = eval.EncryptSecrets(ctx, envName, yaml, rot128{})
		if err != nil {
			return nil, 0, err
		}

		revisionNumber := len(env.revisions) + 1
		env.revisions = append(env.revisions, &testEnvironmentRevision{
			number: revisionNumber,
			yaml:   yaml,
			tags:   map[string]bool{base64.StdEncoding.EncodeToString(h.Sum(nil)): true},
		})
		env.revisionTags["latest"] = revisionNumber
	}

	return diags, env.revisionTags["latest"], err
}

func (c *testPulumiClient) DeleteEnvironment(ctx context.Context, orgName, projectName, envName string) error {
	name := path.Join(orgName, projectName, envName)
	if _, ok := c.environments[name]; !ok {
		return errors.New("not found")
	}
	delete(c.environments, name)
	return nil
}

func (c *testPulumiClient) OpenEnvironment(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	version string,
	duration time.Duration,
) (string, []client.EnvironmentDiagnostic, error) {
	_, env, err := c.getEnvironment(orgName, projectName, envName, version)
	if err != nil {
		return "", nil, err
	}

	return c.openEnvironment(ctx, orgName, envName, env.yaml)
}

func (c *testPulumiClient) CheckYAMLEnvironment(
	ctx context.Context,
	orgName string,
	yaml []byte,
	opts ...client.CheckYAMLOption,
) (*esc.Environment, []client.EnvironmentDiagnostic, error) {
	return c.checkEnvironment(ctx, orgName, "<yaml>", yaml, opts)
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
	return c.GetOpenEnvironmentWithProject(ctx, orgName, client.DefaultProject, envName, openEnvID)
}

func (c *testPulumiClient) GetOpenEnvironmentWithProject(ctx context.Context, orgName, projectName, envName, openEnvID string) (*esc.Environment, error) {
	env, ok := c.openEnvs[openEnvID]
	if !ok {
		return nil, errors.New("not found")
	}
	return env, nil
}

func (c *testPulumiClient) GetAnonymousOpenEnvironment(ctx context.Context, orgName, openEnvID string) (*esc.Environment, error) {
	return c.GetOpenEnvironmentWithProject(ctx, orgName, "project", "yaml", openEnvID)
}

func (c *testPulumiClient) GetOpenProperty(ctx context.Context, orgName, projectName, envName, openEnvID, property string) (*esc.Value, error) {
	return nil, errors.New("NYI")
}

func (c *testPulumiClient) GetAnonymousOpenProperty(ctx context.Context, orgName, openEnvID, property string) (*esc.Value, error) {
	return nil, errors.New("NYI")
}

func (c *testPulumiClient) GetEnvironmentTag(
	ctx context.Context,
	orgName, projectName, envName, key string,
) (*client.EnvironmentTag, error) {
	env, ok := c.environments[fmt.Sprintf("%s/%s/%s", orgName, projectName, envName)]
	if !ok {
		return nil, errors.New("environment not found")
	}

	if v, ok := env.tags[key]; ok {
		ts, _ := time.Parse(time.RFC1123, "Mon, 29 Jul 2024 12:30:00 UTC")
		return &client.EnvironmentTag{
			ID:          key,
			Name:        key,
			Value:       v,
			Created:     ts,
			Modified:    ts,
			EditorLogin: "pulumipus",
			EditorName:  "pulumipus",
		}, nil
	}
	return nil, &apitype.ErrorResponse{Code: http.StatusNotFound}
}

func (c *testPulumiClient) ListEnvironmentTags(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	options client.ListEnvironmentTagsOptions,
) ([]*client.EnvironmentTag, string, error) {
	env, ok := c.environments[fmt.Sprintf("%s/%s/%s", orgName, projectName, envName)]
	if !ok {
		return nil, "", errors.New("environment not found")
	}

	ts, _ := time.Parse(time.RFC1123, "Mon, 29 Jul 2024 12:30:00 UTC")
	tags := []*client.EnvironmentTag{}
	for k, v := range env.tags {
		tags = append(tags, &client.EnvironmentTag{
			ID:          k,
			Name:        k,
			Value:       v,
			Created:     ts,
			Modified:    ts,
			EditorLogin: "pulumipus",
			EditorName:  "pulumipus",
		})
	}
	return tags, "0", nil
}

func (c *testPulumiClient) CreateEnvironmentTag(
	ctx context.Context,
	orgName, projectName, envName, key, value string,
) (*client.EnvironmentTag, error) {
	ts, _ := time.Parse(time.RFC1123, "Mon, 29 Jul 2024 12:30:00 UTC")
	tag := &client.EnvironmentTag{
		ID:          key,
		Name:        key,
		Value:       value,
		Created:     ts,
		Modified:    ts,
		EditorLogin: "pulumipus",
		EditorName:  "pulumipus",
	}
	env, ok := c.environments[fmt.Sprintf("%s/%s/%s", orgName, projectName, envName)]
	if !ok {
		return nil, errors.New("environment not found")
	}
	if _, ok := env.tags[key]; ok {
		return nil, errors.New("tag already exists")
	}
	env.tags[key] = value
	return tag, nil
}

func (c *testPulumiClient) UpdateEnvironmentTag(
	ctx context.Context,
	orgName, projectName, envName, currentKey, currentValue, newKey, newValue string,
) (*client.EnvironmentTag, error) {
	name := newKey
	if name == "" {
		name = currentKey
	}
	value := newValue
	if value == "" {
		value = currentValue
	}
	ts, _ := time.Parse(time.RFC1123, "Mon, 29 Jul 2024 12:30:00 UTC")
	env, ok := c.environments[fmt.Sprintf("%s/%s/%s", orgName, projectName, envName)]
	if !ok {
		return nil, errors.New("environment not found")
	}

	if _, ok := env.tags[currentKey]; !ok {
		return nil, &apitype.ErrorResponse{Code: http.StatusNotFound}
	}
	if newKey != "" {
		delete(env.tags, currentKey)
	}
	env.tags[name] = value
	return &client.EnvironmentTag{
		ID:          name,
		Name:        name,
		Value:       value,
		Created:     ts,
		Modified:    ts,
		EditorLogin: "pulumipus",
		EditorName:  "pulumipus",
	}, nil
}

func (c *testPulumiClient) DeleteEnvironmentTag(ctx context.Context, orgName, projectName, envName, tagName string) error {
	env, ok := c.environments[fmt.Sprintf("%s/%s/%s", orgName, projectName, envName)]
	if !ok {
		return errors.New("environment not found")
	}
	if _, ok := env.tags[tagName]; !ok {
		return errors.New("tag not found")
	}
	delete(env.tags, tagName)
	return nil
}

func (c *testPulumiClient) GetEnvironmentRevision(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	revision int,
) (*client.EnvironmentRevision, error) {
	before, count := revision+1, 1

	opts := client.ListEnvironmentRevisionsOptions{Before: &before, Count: &count}
	revs, err := c.ListEnvironmentRevisions(ctx, orgName, projectName, envName, opts)
	if err != nil || len(revs) == 0 {
		return nil, err
	}
	return &revs[0], nil
}

func (c *testPulumiClient) ListEnvironmentRevisions(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	options client.ListEnvironmentRevisionsOptions,
) ([]client.EnvironmentRevision, error) {
	env, _, err := c.getEnvironment(orgName, projectName, envName, "")
	if err != nil {
		return nil, err
	}

	before := len(env.revisions) + 1
	if options.Before != nil && *options.Before != 0 {
		before = *options.Before
	}

	var resp []client.EnvironmentRevision
	for i := before - 1; i > 0; i-- {
		tags := []string{}
		for k := range env.revisions[i-1].tags {
			if k != "" {
				tags = append(tags, k)
			}
		}

		var retracted *client.EnvironmentRevisionRetracted
		if r := env.revisions[i-1].retracted; r != nil {
			retracted = &client.EnvironmentRevisionRetracted{
				Replacement: r.replacement,
				At:          time.Unix(0, 0).Add(time.Duration(i) * time.Hour),
				ByLogin:     "test-tester",
				ByName:      "Test Tester",
				Reason:      r.reason,
			}
		}

		resp = append(resp, client.EnvironmentRevision{
			Number:       i,
			Created:      time.Unix(0, 0).Add(time.Duration(i) * time.Hour),
			CreatorLogin: "test-tester",
			CreatorName:  "Test Tester",
			Tags:         tags,
			Retracted:    retracted,
		})
	}

	return resp, nil
}

// RetractEnvironmentRevision retracts a specific revision of an environment.
func (c *testPulumiClient) RetractEnvironmentRevision(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	version string,
	replacement *int,
	reason string,
) error {
	_, rev, err := c.getEnvironment(orgName, projectName, envName, version)
	if err != nil {
		return err
	}
	if replacement == nil {
		// It's okay to fake up a replacement here. We don't actually interpret this value.
		rev.retracted = &testEnvironmentRetract{replacement: 42, reason: reason}
	} else {
		rev.retracted = &testEnvironmentRetract{replacement: *replacement, reason: reason}
	}
	return nil
}

// CreateEnvironmentRevisionTag creates a new revision tag with the given name.
func (c *testPulumiClient) CreateEnvironmentRevisionTag(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	tagName string,
	revision *int,
) error {
	env, _, err := c.getEnvironment(orgName, projectName, envName, "")
	if err != nil {
		return err
	}

	rev := len(env.revisions)
	if revision != nil {
		rev = *revision
	}
	if rev < 1 || rev > len(env.revisions) {
		return errors.New("not found")
	}

	if _, ok := env.revisionTags[tagName]; ok {
		return errors.New("already exists")
	}

	env.revisionTags[tagName] = rev
	return nil
}

// GetEnvironmentRevisionTag returns a description of the given revision tag.
func (c *testPulumiClient) GetEnvironmentRevisionTag(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	tagName string,
) (*client.EnvironmentRevisionTag, error) {
	env, _, err := c.getEnvironment(orgName, projectName, envName, "")
	if err != nil {
		return nil, err
	}

	rev, ok := env.revisionTags[tagName]
	if !ok {
		return nil, &apitype.ErrorResponse{Code: http.StatusNotFound}
	}
	return &client.EnvironmentRevisionTag{Name: tagName, Revision: rev}, nil
}

// UpdateEnvironmentRevisionTag updates the revision tag with the given name.
func (c *testPulumiClient) UpdateEnvironmentRevisionTag(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	tagName string,
	revision *int,
) error {
	env, _, err := c.getEnvironment(orgName, projectName, envName, "")
	if err != nil {
		return err
	}

	rev := len(env.revisions)
	if revision != nil {
		rev = *revision
	}
	if rev < 1 || rev > len(env.revisions) {
		return errors.New("not found")
	}

	if _, ok := env.revisionTags[tagName]; !ok {
		return &apitype.ErrorResponse{Code: http.StatusNotFound}
	}

	env.revisionTags[tagName] = rev
	return nil
}

// DeleteEnvironmentRevisionTag deletes the revision tag with the given name.
func (c *testPulumiClient) DeleteEnvironmentRevisionTag(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	tagName string,
) error {
	env, _, err := c.getEnvironment(orgName, projectName, envName, "")
	if err != nil {
		return err
	}

	if _, ok := env.revisionTags[tagName]; !ok {
		return errors.New("not found")
	}

	delete(env.revisionTags, tagName)
	return nil
}

// ListEnvironmentRevisionTags lists the revision tags for the given environment.
func (c *testPulumiClient) ListEnvironmentRevisionTags(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	options client.ListEnvironmentRevisionTagsOptions,
) ([]client.EnvironmentRevisionTag, error) {
	env, _, err := c.getEnvironment(orgName, projectName, envName, "")
	if err != nil {
		return nil, err
	}

	names := maps.Keys(env.revisionTags)
	slices.Sort(names)
	return fx.ToSlice(fx.FMap(fx.IterSlice(names), func(name string) (client.EnvironmentRevisionTag, bool) {
		return client.EnvironmentRevisionTag{Name: name, Revision: env.revisionTags[name]}, name > options.After
	})), nil
}

// unused
func (c *testPulumiClient) EnvironmentExists(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
) (bool, error) {
	return false, nil
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
					pager:           testPager(0),
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
				fmt.Fprintf(hc.Stderr, "> %v\n", strings.Join(args, " "))

				esc.SetArgs(args[1:])
				esc.SetIn(hc.Stdin)
				esc.SetOut(hc.Stdout)
				esc.SetErr(hc.Stderr)

				ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
				esc.SetContext(ctx)
				defer cancel()

				ch := make(chan error, 1)
				go func() {
					ch <- esc.Execute()
				}()
				select {
				case <-ctx.Done():
					return ctx.Err()
				case result := <-ch:
					if result != nil {
						return interp.NewExitStatus(1)
					}
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
				c.fs.MapFS[path] = f
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

type testPager int

func (testPager) Run(pager string, stdout, stderr io.Writer, f func(context.Context, io.Writer) error) error {
	return f(context.Background(), stdout)
}

type cliTestcaseProcess struct {
	FS       map[string]string `yaml:"fs,omitempty"`
	Environ  map[string]string `yaml:"environ,omitempty"`
	Commands map[string]string `yaml:"commands,omitempty"`
}

type cliTestcaseRetract struct {
	Replacement int    `yaml:"replacement"`
	Reason      string `yaml:"reason,omitempty"`
}

type cliTestcaseRevision struct {
	YAML      yaml.Node           `yaml:"yaml"`
	Tags      []string            `yaml:"tags,omitempty"`
	Retracted *cliTestcaseRetract `yaml:"retracted,omitempty"`
}

type cliTestcaseRevisions struct {
	Revisions []cliTestcaseRevision `yaml:"revisions,omitempty"`
}

type cliTestcaseEnvironmentTags struct {
	Tags map[string]string `yaml:"tags,omitempty"`
}

type cliTestcaseYAML struct {
	Parent string `yaml:"parent,omitempty"`

	Run   string `yaml:"run,omitempty"`
	Error string `yaml:"error,omitempty"`

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
		var revisions cliTestcaseRevisions
		if err := env.Decode(&revisions); err != nil || len(revisions.Revisions) == 0 {
			revisions = cliTestcaseRevisions{Revisions: []cliTestcaseRevision{{YAML: env}}}
		}

		var tags cliTestcaseEnvironmentTags
		envTags := map[string]string{}
		if err := env.Decode(&tags); tags.Tags != nil && err == nil {
			envTags = tags.Tags
		}

		envRevisions := []*testEnvironmentRevision{{number: 1}}
		revisionTags := map[string]int{}
		for _, rev := range revisions.Revisions {
			bytes, err := yaml.Marshal(rev.YAML)
			if err != nil {
				return nil, nil, err
			}

			var retract *testEnvironmentRetract
			if rev.Retracted != nil {
				retract = &testEnvironmentRetract{
					replacement: rev.Retracted.Replacement,
					reason:      rev.Retracted.Reason,
				}
			}

			revisionNumber := len(envRevisions) + 1
			envRevisions = append(envRevisions, &testEnvironmentRevision{
				number:    revisionNumber,
				yaml:      bytes,
				retracted: retract,
				tags:      map[string]bool{},
			})

			for _, rt := range rev.Tags {
				if _, ok := revisionTags[rt]; ok || rt == "latest" {
					return nil, nil, fmt.Errorf("duplicate tag %q", rt)
				}
				revisionTags[rt] = revisionNumber
				envRevisions[revisionNumber-1].tags[rt] = true
			}
		}
		revisionTags["latest"] = len(envRevisions)

		environments[k] = &testEnvironment{
			revisions:    envRevisions,
			revisionTags: revisionTags,
			tags:         envTags,
		}
	}

	creds := workspace.Credentials{
		Current: "http://fake.pulumi.api",
		Accounts: map[string]workspace.Account{
			"https://api.pulumi.com": {
				Username:    "test-user",
				AccessToken: "access-token",
			},
			"http://fake.pulumi.api": {
				Username:    "test-user",
				AccessToken: "access-token",
			},
		},
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
