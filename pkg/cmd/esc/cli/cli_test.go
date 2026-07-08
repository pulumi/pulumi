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
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gofrs/uuid"
	"github.com/hashicorp/hcl/v2"
	"github.com/pgavlin/fx/v2"
	"github.com/pgavlin/fx/v2/maps"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/cmd/esc/cli/client"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc/eval"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/esc/syntax"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestMain(m *testing.M) {
	os.Unsetenv("PULUMI_API")
	os.Exit(m.Run())
}

func (env testEnviron) Get(key string) string {
	return env[key]
}

func (env testEnviron) Vars() []string {
	vars := make([]string, 0, len(env))
	for k, v := range env {
		vars = append(vars, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(vars)
	return vars
}

// mockWorkspace returns a pulumi workspace Context that serves the given stored credentials,
// used to drive ESC's credential reads in tests.
func mockWorkspace(creds workspace.Credentials) pkgWorkspace.Context {
	return &pkgWorkspace.MockContext{
		GetStoredCredentialsF: func() (workspace.Credentials, error) {
			return creds, nil
		},
	}
}

type testProvider struct{}

func (testProvider) Schema() (*schema.Schema, *schema.Schema) {
	return schema.Always(), schema.Always()
}

func (testProvider) Open(
	ctx context.Context,
	inputs map[string]esc.Value,
	context esc.EnvExecContext,
) (esc.Value, error) {
	return esc.NewValue(inputs), nil
}

type testProviders struct{}

func (testProviders) LoadProvider(ctx context.Context, name string) (esc.Provider, error) {
	switch name {
	case "test", "aws-login", "azure-login", "gcp-login":
		return testProvider{}, nil
	}
	return nil, fmt.Errorf("unknown provider %q", name)
}

func (testProviders) LoadRotator(ctx context.Context, name string) (esc.Rotator, error) {
	return nil, fmt.Errorf("unknown rotator %q", name)
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
	etag      string
	tags      map[string]bool
	retracted *testEnvironmentRetract
}

type testEnvironment struct {
	revisions         []*testEnvironmentRevision
	revisionTags      map[string]int
	tags              map[string]string
	deletionProtected bool
	webhooks          []client.EnvironmentWebhook
	webhookDeliveries map[string][]client.EnvironmentWebhookDelivery
	schedules         []client.ScheduledAction
	referrers         map[string][]client.EnvironmentReferrer
}

func (env *testEnvironment) latest() *testEnvironmentRevision {
	return env.revisions[len(env.revisions)-1]
}

type testPulumiClient struct {
	user         string
	defaultOrg   string
	environments map[string]*testEnvironment
	openEnvs     map[string]*esc.Environment
}

type testLoginManager struct {
	creds workspace.Credentials
}

// Current returns the current cloud backend if one is already logged in.
func (lm *testLoginManager) Current(
	ctx context.Context,
	cloudURL string,
	insecure, setCurrent bool,
) (*workspace.Account, error) {
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

func (lm *testLoginManager) LoginWithOIDCToken(
	ctx context.Context,
	sink diag.Sink,
	cloudURL string,
	insecure bool,
	oidcTokenSource string,
	organization string,
	scope string,
	expiration time.Duration,
	setCurrent bool,
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

		severity := client.DiagError
		if d.Severity == hcl.DiagWarning {
			severity = client.DiagWarning
		}

		out[i] = client.EnvironmentDiagnostic{
			Range:    rng,
			Summary:  d.Summary,
			Detail:   d.Detail,
			Severity: severity,
		}
	}
	return out
}

func (c *testPulumiClient) getEnvironment(
	orgName, projectName, envName, version string,
) (*testEnvironment, *testEnvironmentRevision, error) {
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

func (c *testPulumiClient) checkEnvironment(
	ctx context.Context,
	orgName, envName string,
	yaml []byte,
	opts []client.CheckYAMLOption,
) (*esc.Environment, []client.EnvironmentDiagnostic, error) {
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

	checked, checkDiags := eval.CheckEnvironment(
		ctx,
		envName,
		environment,
		rot128{},
		providers,
		envLoader,
		execContext,
		showSecrets,
	)
	diags.Extend(checkDiags...)
	return checked, mapDiags(diags), nil
}

func (c *testPulumiClient) openEnvironment(
	ctx context.Context,
	orgName, name string,
	yaml []byte,
) (string, []client.EnvironmentDiagnostic, error) {
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
	return "https://api.fake.pulumi.com"
}

// GetPulumiAccountDetails returns the user implied by the API token associated with this client.
func (c *testPulumiClient) GetPulumiAccountDetails(
	ctx context.Context,
) (string, []string, *workspace.TokenInformation, error) {
	return c.user, []string{"test-org"}, nil, nil
}

func (c *testPulumiClient) GetRevisionNumber(
	ctx context.Context,
	orgName, projectName, envName, version string,
) (int, error) {
	_, rev, err := c.getEnvironment(orgName, projectName, envName, version)
	if err != nil {
		return 0, err
	}
	return rev.number, nil
}

func (c *testPulumiClient) GetDefaultOrg(ctx context.Context) (string, error) {
	return c.defaultOrg, nil
}

func (c *testPulumiClient) ListEnvironments(
	ctx context.Context,
	continuationToken string,
) ([]client.OrgEnvironment, string, error) {
	return c.ListOrganizationEnvironments(ctx, "", continuationToken)
}

func (c *testPulumiClient) ListOrganizationEnvironments(
	ctx context.Context,
	orgName string,
	continuationToken string,
) ([]client.OrgEnvironment, string, error) {
	var envs []client.OrgEnvironment
	for k := range maps.Sorted(c.environments) {
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

func (c *testPulumiClient) CreateEnvironment(
	ctx context.Context,
	orgName, projectName, envName string,
) error {
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

	return yaml, env.etag, env.number, nil
}

func (c *testPulumiClient) UpdateEnvironment(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	yaml []byte,
	etag string,
) ([]client.EnvironmentDiagnostic, int, error) {
	env, latest, err := c.getEnvironment(orgName, projectName, envName, "")
	if err != nil {
		return nil, 0, err
	}

	if etag != "" && etag != latest.etag {
		return nil, 0, errors.New("etag mismatch")
	}

	envId := projectName + "/" + envName
	_, diags, err := c.checkEnvironment(ctx, orgName, envId, yaml, nil)
	if err == nil && !client.DiagnosticsHaveErrors(diags) {
		h := fnv.New32()
		h.Write(yaml)

		yaml, err = eval.EncryptSecrets(ctx, envId, yaml, rot128{})
		if err != nil {
			return nil, 0, err
		}

		revisionNumber := len(env.revisions) + 1
		env.revisions = append(env.revisions, &testEnvironmentRevision{
			number: revisionNumber,
			yaml:   yaml,
			etag:   base64.StdEncoding.EncodeToString(h.Sum(nil)),
			tags:   map[string]bool{"latest": true},
		})

		if n, ok := env.revisionTags["latest"]; ok && n > 0 {
			delete(env.revisions[n-1].tags, "latest")
		}
		env.revisionTags["latest"] = revisionNumber
	}

	return diags, env.revisionTags["latest"], err
}

func (c *testPulumiClient) CreateEnvironmentDraft(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	yaml []byte,
	etag string,
) (string, []client.EnvironmentDiagnostic, error) {
	_, latest, err := c.getEnvironment(orgName, projectName, envName, "")
	if err != nil {
		return "", nil, err
	}

	if etag != "" && etag != latest.etag {
		return "", nil, errors.New("etag mismatch")
	}

	envId := projectName + "/" + envName
	_, diags, err := c.checkEnvironment(ctx, orgName, envId, yaml, nil)
	if err != nil || len(diags) != 0 {
		return "", diags, nil
	}
	// store drafts in dummy environments
	envName = envName + "_DRAFT"
	err = c.CreateEnvironment(ctx, orgName, projectName, envName)
	if err != nil {
		return "", nil, err
	}
	diags, _, err = c.UpdateEnvironment(ctx, orgName, projectName, envName, yaml, "")
	if err == nil && len(diags) == 0 {
		return "00000000-0000-0000-0000-000000000000", []client.EnvironmentDiagnostic{}, nil
	}
	return "", diags, err
}

func (c *testPulumiClient) GetEnvironmentDraft(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	changeRequestID string,
) ([]byte, string, error) {
	envName = envName + "_DRAFT"
	_, env, err := c.getEnvironment(orgName, projectName, envName, "")
	if err != nil {
		return nil, "", err
	}

	return env.yaml, env.etag, nil
}

func (c *testPulumiClient) UpdateEnvironmentDraft(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	changeRequestID string,
	yaml []byte,
	etag string,
) ([]client.EnvironmentDiagnostic, error) {
	envName = envName + "_DRAFT"
	diags, _, err := c.UpdateEnvironment(ctx, orgName, projectName, envName, yaml, etag)
	return diags, err
}

func (c *testPulumiClient) SubmitChangeRequest(
	ctx context.Context,
	orgName string,
	changeRequestID string,
	description *string,
) error {
	return nil
}

func (c *testPulumiClient) DeleteEnvironment(ctx context.Context, orgName, projectName, envName string) error {
	name := path.Join(orgName, projectName, envName)
	env, ok := c.environments[name]
	if !ok {
		return errors.New("not found")
	}
	if env.deletionProtected {
		return &apitype.ErrorResponse{Code: http.StatusConflict, Message: "environment is deletion protected"}
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

func (c *testPulumiClient) OpenEnvironmentDraft(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	changeRequestID string,
	duration time.Duration,
) (string, []client.EnvironmentDiagnostic, error) {
	envName = envName + "_DRAFT"
	return c.OpenEnvironment(ctx, orgName, projectName, envName, "", duration)
}

func (c *testPulumiClient) RotateEnvironment(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	rotationPaths []string,
) (*client.RotateEnvironmentResponse, []client.EnvironmentDiagnostic, error) {
	return &client.RotateEnvironmentResponse{}, []client.EnvironmentDiagnostic{}, nil
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

func (c *testPulumiClient) GetOpenEnvironment(
	ctx context.Context,
	orgName, projectName, envName, openEnvID string,
) (*esc.Environment, error) {
	env, ok := c.openEnvs[openEnvID]
	if !ok {
		return nil, errors.New("not found")
	}
	return env, nil
}

func (c *testPulumiClient) GetAnonymousOpenEnvironment(
	ctx context.Context,
	orgName, openEnvID string,
) (*esc.Environment, error) {
	return c.GetOpenEnvironment(ctx, orgName, "project", "yaml", openEnvID)
}

func (c *testPulumiClient) GetOpenProperty(
	ctx context.Context,
	orgName, projectName, envName, openEnvID, property string,
) (*esc.Value, error) {
	return nil, errors.New("NYI")
}

func (c *testPulumiClient) GetAnonymousOpenProperty(
	ctx context.Context,
	orgName, openEnvID, property string,
) (*esc.Value, error) {
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

func (c *testPulumiClient) ListEnvironmentReferrers(
	ctx context.Context,
	orgName, projectName, envName string,
	options client.ListEnvironmentReferrersOptions,
) (*client.ListEnvironmentReferrersResponse, error) {
	env, ok := c.environments[fmt.Sprintf("%s/%s/%s", orgName, projectName, envName)]
	if !ok {
		return nil, errors.New("environment not found")
	}
	resp := &client.ListEnvironmentReferrersResponse{
		Referrers: map[string][]client.EnvironmentReferrer{},
	}
	for k, v := range env.referrers {
		resp.Referrers[k] = v
	}
	return resp, nil
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

func (c *testPulumiClient) DeleteEnvironmentTag(
	ctx context.Context,
	orgName, projectName, envName, tagName string,
) error {
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

func (c *testPulumiClient) ListEnvironmentSchedules(
	ctx context.Context,
	orgName, projectName, envName string,
) (*client.ListScheduledActionsResponse, error) {
	env, ok := c.environments[fmt.Sprintf("%s/%s/%s", orgName, projectName, envName)]
	if !ok {
		return nil, errors.New("environment not found")
	}
	resp := &client.ListScheduledActionsResponse{
		Schedules: append([]client.ScheduledAction(nil), env.schedules...),
	}
	return resp, nil
}

func (c *testPulumiClient) CreateEnvironmentSchedule(
	ctx context.Context,
	orgName, projectName, envName string,
	req client.CreateEnvironmentScheduleRequest,
) (*client.ScheduledAction, error) {
	env, ok := c.environments[fmt.Sprintf("%s/%s/%s", orgName, projectName, envName)]
	if !ok {
		return nil, errors.New("environment not found")
	}
	id := fmt.Sprintf("sched-%d", len(env.schedules)+1)
	var def json.RawMessage
	if req.SecretRotationRequest != nil {
		def, _ = json.Marshal(map[string]any{
			"environmentPath": req.SecretRotationRequest.EnvironmentPath,
		})
	}
	s := client.ScheduledAction{
		ID:            id,
		OrgID:         orgName,
		Kind:          "environment_rotation",
		Paused:        false,
		Created:       "2026-05-13T12:00:00Z",
		Modified:      "2026-05-13T12:00:00Z",
		LastExecuted:  "",
		NextExecution: "2026-05-14T00:00:00Z",
		Definition:    def,
		ScheduleCron:  req.ScheduleCron,
		ScheduleOnce:  req.ScheduleOnce,
	}
	env.schedules = append(env.schedules, s)
	return &s, nil
}

func (c *testPulumiClient) findSchedule(
	orgName, projectName, envName, scheduleID string,
) (*testEnvironment, int, error) {
	env, ok := c.environments[fmt.Sprintf("%s/%s/%s", orgName, projectName, envName)]
	if !ok {
		return nil, 0, errors.New("environment not found")
	}
	for i, s := range env.schedules {
		if s.ID == scheduleID {
			return env, i, nil
		}
	}
	return nil, 0, errors.New("schedule not found")
}

func (c *testPulumiClient) GetEnvironmentSchedule(
	ctx context.Context,
	orgName, projectName, envName, scheduleID string,
) (*client.ScheduledAction, error) {
	env, i, err := c.findSchedule(orgName, projectName, envName, scheduleID)
	if err != nil {
		return nil, err
	}
	s := env.schedules[i]
	return &s, nil
}

func (c *testPulumiClient) UpdateEnvironmentSchedule(
	ctx context.Context,
	orgName, projectName, envName, scheduleID string,
	req client.UpdateEnvironmentScheduleRequest,
) (*client.ScheduledAction, error) {
	env, i, err := c.findSchedule(orgName, projectName, envName, scheduleID)
	if err != nil {
		return nil, err
	}
	if req.ScheduleCron != "" {
		env.schedules[i].ScheduleCron = req.ScheduleCron
		env.schedules[i].ScheduleOnce = ""
	}
	if req.ScheduleOnce != "" {
		env.schedules[i].ScheduleOnce = req.ScheduleOnce
		env.schedules[i].ScheduleCron = ""
	}
	if req.SecretRotationRequest != nil {
		def, _ := json.Marshal(map[string]any{
			"environmentPath": req.SecretRotationRequest.EnvironmentPath,
		})
		env.schedules[i].Definition = def
	}
	env.schedules[i].Modified = "2026-05-13T13:00:00Z"
	s := env.schedules[i]
	return &s, nil
}

func (c *testPulumiClient) ListEnvironmentScheduleHistory(
	ctx context.Context,
	orgName, projectName, envName, scheduleID string,
) (*client.ListScheduleHistoryResponse, error) {
	if _, _, err := c.findSchedule(orgName, projectName, envName, scheduleID); err != nil {
		return nil, err
	}
	return &client.ListScheduleHistoryResponse{
		ScheduleHistoryEvents: []client.ScheduleHistoryEvent{
			{
				ID:                "evt-1",
				ScheduledActionID: scheduleID,
				Executed:          "2026-05-13T12:30:00Z",
				Version:           1,
				Result:            "succeeded",
			},
		},
	}, nil
}

func (c *testPulumiClient) DeleteEnvironmentSchedule(
	ctx context.Context,
	orgName, projectName, envName, scheduleID string,
) error {
	env, i, err := c.findSchedule(orgName, projectName, envName, scheduleID)
	if err != nil {
		return err
	}
	env.schedules = append(env.schedules[:i], env.schedules[i+1:]...)
	return nil
}

func (c *testPulumiClient) findWebhook(
	orgName, projectName, envName, webhookName string,
) (*testEnvironment, int, error) {
	env, ok := c.environments[fmt.Sprintf("%s/%s/%s", orgName, projectName, envName)]
	if !ok {
		return nil, 0, errors.New("environment not found")
	}
	for i, w := range env.webhooks {
		if w.Name == webhookName {
			return env, i, nil
		}
	}
	return nil, 0, errors.New("webhook not found")
}

func (c *testPulumiClient) ListEnvironmentWebhooks(
	ctx context.Context,
	orgName, projectName, envName string,
) ([]client.EnvironmentWebhook, error) {
	env, ok := c.environments[fmt.Sprintf("%s/%s/%s", orgName, projectName, envName)]
	if !ok {
		return nil, errors.New("environment not found")
	}
	return append([]client.EnvironmentWebhook(nil), env.webhooks...), nil
}

func (c *testPulumiClient) GetEnvironmentWebhook(
	ctx context.Context,
	orgName, projectName, envName, webhookName string,
) (*client.EnvironmentWebhook, error) {
	env, i, err := c.findWebhook(orgName, projectName, envName, webhookName)
	if err != nil {
		return nil, err
	}
	w := env.webhooks[i]
	return &w, nil
}

func (c *testPulumiClient) CreateEnvironmentWebhook(
	ctx context.Context,
	orgName, projectName, envName string,
	req client.CreateEnvironmentWebhookRequest,
) (*client.EnvironmentWebhook, error) {
	env, ok := c.environments[fmt.Sprintf("%s/%s/%s", orgName, projectName, envName)]
	if !ok {
		return nil, errors.New("environment not found")
	}
	name := req.Name
	if name == "" {
		name = fmt.Sprintf("hook-%d", len(env.webhooks)+1)
	}
	for _, w := range env.webhooks {
		if w.Name == name {
			return nil, errors.New("webhook already exists")
		}
	}
	w := client.EnvironmentWebhook{
		Active:           req.Active,
		DisplayName:      req.DisplayName,
		Name:             name,
		OrganizationName: orgName,
		PayloadURL:       req.PayloadURL,
		EnvName:          envName,
		Filters:          append([]string(nil), req.Filters...),
		Groups:           append([]string(nil), req.Groups...),
		Format:           req.Format,
		ProjectName:      projectName,
		HasSecret:        req.Secret != "",
	}
	env.webhooks = append(env.webhooks, w)
	return &w, nil
}

func (c *testPulumiClient) UpdateEnvironmentWebhook(
	ctx context.Context,
	orgName, projectName, envName, webhookName string,
	req client.UpdateEnvironmentWebhookRequest,
) (*client.EnvironmentWebhook, error) {
	env, i, err := c.findWebhook(orgName, projectName, envName, webhookName)
	if err != nil {
		return nil, err
	}
	w := env.webhooks[i]
	w.Active = req.Active
	w.DisplayName = req.DisplayName
	w.PayloadURL = req.PayloadURL
	w.Filters = append([]string(nil), req.Filters...)
	w.Groups = append([]string(nil), req.Groups...)
	if req.Format != nil {
		w.Format = *req.Format
	}
	switch req.Secret {
	case "":
		// leave HasSecret unchanged
	case "__remove-secret":
		w.HasSecret = false
	default:
		w.HasSecret = true
	}
	env.webhooks[i] = w
	return &w, nil
}

func (c *testPulumiClient) DeleteEnvironmentWebhook(
	ctx context.Context,
	orgName, projectName, envName, webhookName string,
) error {
	env, i, err := c.findWebhook(orgName, projectName, envName, webhookName)
	if err != nil {
		return err
	}
	env.webhooks = append(env.webhooks[:i], env.webhooks[i+1:]...)
	return nil
}

func (c *testPulumiClient) PingEnvironmentWebhook(
	ctx context.Context,
	orgName, projectName, envName, webhookName string,
) (*client.EnvironmentWebhookDelivery, error) {
	env, _, err := c.findWebhook(orgName, projectName, envName, webhookName)
	if err != nil {
		return nil, err
	}
	if env.webhookDeliveries == nil {
		env.webhookDeliveries = map[string][]client.EnvironmentWebhookDelivery{}
	}
	id := fmt.Sprintf("dlv-%s-%d", webhookName, len(env.webhookDeliveries[webhookName])+1)
	d := client.EnvironmentWebhookDelivery{
		ID:              id,
		Kind:            "ping",
		Timestamp:       1747094400,
		Duration:        42,
		Payload:         "{}",
		RequestURL:      "https://example.invalid/hook",
		RequestHeaders:  "Content-Type: application/json",
		ResponseCode:    200,
		ResponseHeaders: "Content-Type: text/plain",
		ResponseBody:    "ok",
	}
	env.webhookDeliveries[webhookName] = append(env.webhookDeliveries[webhookName], d)
	return &d, nil
}

func (c *testPulumiClient) ListEnvironmentWebhookDeliveries(
	ctx context.Context,
	orgName, projectName, envName, webhookName string,
) ([]client.EnvironmentWebhookDelivery, error) {
	env, _, err := c.findWebhook(orgName, projectName, envName, webhookName)
	if err != nil {
		return nil, err
	}
	return append([]client.EnvironmentWebhookDelivery(nil), env.webhookDeliveries[webhookName]...), nil
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

	return slices.Collect(
		fx.FMap(
			maps.SortedPairs(env.revisionTags),
			func(kvp fx.Pair[string, int]) (client.EnvironmentRevisionTag, bool) {
				return client.EnvironmentRevisionTag{Name: kvp.Fst, Revision: kvp.Snd}, kvp.Fst > options.After
			},
		),
	), nil
}

// unused
func (c *testPulumiClient) EnvironmentExists(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
) (bool, error) {
	_, ok := c.environments[path.Join(orgName, projectName, envName)]
	return ok, nil
}

func (c *testPulumiClient) CreateEnvironmentOpenRequest(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	grantExpirationSeconds int,
	accessDurationSeconds int,
) (*client.CreateEnvironmentOpenRequestResponse, error) {
	// Check if environment exists
	key := fmt.Sprintf("%s/%s/%s", orgName, projectName, envName)
	if _, exists := c.environments[key]; !exists {
		return nil, errors.New("environment not found")
	}

	// Mock implementation for testing
	return &client.CreateEnvironmentOpenRequestResponse{
		ChangeRequests: []client.EnvironmentOpenRequestChangeRequest{
			{
				ProjectName:          projectName,
				EnvironmentName:      envName,
				ChangeRequestID:      "test-request-id-12345",
				LatestRevisionNumber: 0,
				ETag:                 "test-etag/0",
			},
		},
	}, nil
}

func (c *testPulumiClient) GetEnvironmentSettings(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
) (*client.EnvironmentSettings, error) {
	name := path.Join(orgName, projectName, envName)
	env, ok := c.environments[name]
	if !ok {
		return nil, errors.New("not found")
	}
	return &client.EnvironmentSettings{
		DeletionProtected: env.deletionProtected,
	}, nil
}

func (c *testPulumiClient) PatchEnvironmentSettings(
	ctx context.Context,
	orgName string,
	projectName string,
	envName string,
	req client.PatchEnvironmentSettingsRequest,
) error {
	name := path.Join(orgName, projectName, envName)
	env, ok := c.environments[name]
	if !ok {
		return errors.New("not found")
	}
	if req.DeletionProtected != nil {
		env.deletionProtected = *req.DeletionProtected
	}
	return nil
}

type testExec struct {
	fs       testFS
	environ  map[string]string
	commands map[string]string

	parentPath string
	login      *testLoginManager
	creds      workspace.Credentials
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
					ParentPath: c.parentPath,
					Stdin:      hc.Stdin,
					Stdout:     hc.Stdout,
					Stderr:     hc.Stderr,
					Colors:     colors.Never,
					Login:      c.login,
					ws:         mockWorkspace(c.creds),
					fs:         c.fs,
					environ:    environ,
					exec:       c,
					pager:      testPager(0),
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
						fmt.Fprintf(hc.Stderr, "Error: %s\n", result)
						return error(interp.ExitStatus(uint8(1)))
					}
				}
				return nil
			}
		}),
		interp.OpenHandler(
			func(ctx context.Context, path string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
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
			},
		),
		//nolint:staticcheck // ReadDirHandler2 has a different signature; this stub is sufficient for tests
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

type cliTestcaseWebhook struct {
	Name        string   `yaml:"name,omitempty"`
	DisplayName string   `yaml:"display-name,omitempty"`
	PayloadURL  string   `yaml:"payload-url,omitempty"`
	Active      bool     `yaml:"active,omitempty"`
	Format      string   `yaml:"format,omitempty"`
	Filters     []string `yaml:"filters,omitempty"`
	HasSecret   bool     `yaml:"has-secret,omitempty"`
}

type cliTestcaseEnvironmentWebhooks struct {
	Webhooks []cliTestcaseWebhook `yaml:"webhooks,omitempty"`
}

func (w cliTestcaseWebhook) toClient(orgName, projectName, envName string) client.EnvironmentWebhook {
	return client.EnvironmentWebhook{
		Active:           w.Active,
		DisplayName:      w.DisplayName,
		Name:             w.Name,
		OrganizationName: orgName,
		PayloadURL:       w.PayloadURL,
		EnvName:          envName,
		Filters:          append([]string(nil), w.Filters...),
		Format:           w.Format,
		ProjectName:      projectName,
		HasSecret:        w.HasSecret,
	}
}

type cliTestcaseSchedule struct {
	ID            string `yaml:"id,omitempty"`
	Kind          string `yaml:"kind,omitempty"`
	Paused        bool   `yaml:"paused,omitempty"`
	Created       string `yaml:"created,omitempty"`
	Modified      string `yaml:"modified,omitempty"`
	LastExecuted  string `yaml:"last-executed,omitempty"`
	NextExecution string `yaml:"next-execution,omitempty"`
	ScheduleCron  string `yaml:"schedule-cron,omitempty"`
	ScheduleOnce  string `yaml:"schedule-once,omitempty"`
	Path          string `yaml:"path,omitempty"`
}

type cliTestcaseEnvironmentSchedules struct {
	Schedules []cliTestcaseSchedule `yaml:"schedules,omitempty"`
}

func (s cliTestcaseSchedule) toClient() client.ScheduledAction {
	var def json.RawMessage
	if s.Path != "" {
		def, _ = json.Marshal(map[string]any{"environmentPath": s.Path})
	}
	kind := s.Kind
	if kind == "" {
		kind = "environment_rotation"
	}
	return client.ScheduledAction{
		ID:            s.ID,
		Kind:          kind,
		Paused:        s.Paused,
		Created:       s.Created,
		Modified:      s.Modified,
		LastExecuted:  s.LastExecuted,
		NextExecution: s.NextExecution,
		Definition:    def,
		ScheduleCron:  s.ScheduleCron,
		ScheduleOnce:  s.ScheduleOnce,
	}
}

type cliTestcaseReferrer struct {
	Environment     *cliTestcaseEnvironmentImportReferrer `yaml:"environment,omitempty"`
	Stack           *cliTestcaseStackReferrer             `yaml:"stack,omitempty"`
	InsightsAccount *cliTestcaseInsightsAccountReferrer   `yaml:"insights-account,omitempty"`
}

type cliTestcaseEnvironmentImportReferrer struct {
	Project  string `yaml:"project"`
	Name     string `yaml:"name"`
	Revision int64  `yaml:"revision"`
}

type cliTestcaseStackReferrer struct {
	Project string `yaml:"project"`
	Stack   string `yaml:"stack"`
	Version int64  `yaml:"version"`
}

type cliTestcaseInsightsAccountReferrer struct {
	AccountName string `yaml:"account-name"`
}

type cliTestcaseEnvironmentReferrers struct {
	Referrers map[string][]cliTestcaseReferrer `yaml:"referrers,omitempty"`
}

func (r cliTestcaseReferrer) toClient() client.EnvironmentReferrer {
	out := client.EnvironmentReferrer{}
	if r.Environment != nil {
		out.Environment = &client.EnvironmentImportReferrer{
			Project:  r.Environment.Project,
			Name:     r.Environment.Name,
			Revision: r.Environment.Revision,
		}
	}
	if r.Stack != nil {
		out.Stack = &client.EnvironmentStackReferrer{
			Project: r.Stack.Project,
			Stack:   r.Stack.Stack,
			Version: r.Stack.Version,
		}
	}
	if r.InsightsAccount != nil {
		out.InsightsAccount = &client.EnvironmentInsightsAccountReferrer{
			AccountName: r.InsightsAccount.AccountName,
		}
	}
	return out
}

type cliTestcaseYAML struct {
	Parent string `yaml:"parent,omitempty"`

	Run   string `yaml:"run,omitempty"`
	Error string `yaml:"error,omitempty"`

	Process *cliTestcaseProcess `yaml:"process,omitempty"`

	Environments map[string]yaml.Node `yaml:"environments,omitempty"`
}

type cliTestcase struct {
	exec *testExec

	script         string
	expectedStdout string
	expectedStderr string
}

func loadTestcase(path string) (*cliTestcaseYAML, *cliTestcase, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	header, stdout, stderr := "", "", ""
	switch components := strings.Split(string(contents), "\n---\n"); len(components) {
	case 1:
		header = components[0]
	case 2:
		header, stdout = components[0], components[1]
	case 3:
		header, stdout, stderr = components[0], components[1], components[2]
	}

	var testcase cliTestcaseYAML
	if err := yaml.NewDecoder(strings.NewReader(header)).Decode(&testcase); err != nil {
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

		var webhooks cliTestcaseEnvironmentWebhooks
		var envWebhooks []client.EnvironmentWebhook
		if err := env.Decode(&webhooks); webhooks.Webhooks != nil && err == nil {
			parts := strings.SplitN(k, "/", 3)
			var orgName, projectName, envName string
			if len(parts) == 3 {
				orgName, projectName, envName = parts[0], parts[1], parts[2]
			}
			envWebhooks = make([]client.EnvironmentWebhook, len(webhooks.Webhooks))
			for i, w := range webhooks.Webhooks {
				envWebhooks[i] = w.toClient(orgName, projectName, envName)
			}
		}

		var schedules cliTestcaseEnvironmentSchedules
		var envSchedules []client.ScheduledAction
		if err := env.Decode(&schedules); schedules.Schedules != nil && err == nil {
			envSchedules = make([]client.ScheduledAction, len(schedules.Schedules))
			for i, s := range schedules.Schedules {
				envSchedules[i] = s.toClient()
			}
		}

		var referrers cliTestcaseEnvironmentReferrers
		envReferrers := map[string][]client.EnvironmentReferrer{}
		if err := env.Decode(&referrers); referrers.Referrers != nil && err == nil {
			for k, rs := range referrers.Referrers {
				out := make([]client.EnvironmentReferrer, len(rs))
				for i, r := range rs {
					out[i] = r.toClient()
				}
				envReferrers[k] = out
			}
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

			h := fnv.New32()
			h.Write(bytes)

			revisionNumber := len(envRevisions) + 1
			r := &testEnvironmentRevision{
				number:    revisionNumber,
				yaml:      bytes,
				retracted: retract,
				etag:      base64.StdEncoding.EncodeToString(h.Sum(nil)),
				tags:      map[string]bool{},
			}
			envRevisions = append(envRevisions, r)

			for _, rt := range rev.Tags {
				if _, ok := revisionTags[rt]; ok || rt == "latest" {
					return nil, nil, fmt.Errorf("duplicate tag %q", rt)
				}
				revisionTags[rt] = revisionNumber
				r.tags[rt] = true
			}
		}
		revisionTags["latest"] = len(envRevisions)

		environments[k] = &testEnvironment{
			revisions:    envRevisions,
			revisionTags: revisionTags,
			tags:         envTags,
			webhooks:     envWebhooks,
			schedules:    envSchedules,
			referrers:    envReferrers,
		}
	}

	creds := workspace.Credentials{
		Current: "https://api.fake.pulumi.com",
		Accounts: map[string]workspace.Account{
			"https://api.pulumi.com": {
				Username:    "test-user",
				AccessToken: "access-token",
			},
			"https://api.fake.pulumi.com": {
				Username:    "test-user",
				AccessToken: "access-token",
			},
		},
	}

	exec.parentPath = testcase.Parent
	exec.creds = creds
	exec.login = &testLoginManager{creds: creds}
	exec.client = &testPulumiClient{
		user:         "test-user",
		environments: environments,
		openEnvs:     map[string]*esc.Environment{},
	}

	return &testcase, &cliTestcase{
		exec:           &exec,
		script:         testcase.Run,
		expectedStdout: stdout,
		expectedStderr: stderr,
	}, nil
}

func TestCLI(t *testing.T) {
	t.Setenv("PULUMI_API", "")
	// Point PULUMI_HOME at an empty temp dir so the default-org lookup's workspace.GetPulumiConfig()
	// read is hermetic (no local default org) rather than reading the developer's ~/.pulumi.
	t.Setenv("PULUMI_HOME", t.TempDir())
	path := filepath.Join("testdata")
	entries, err := os.ReadDir(path)
	require.NoError(t, err)
	for _, e := range entries { //nolint:paralleltest,lll // non-thread-safe shared state
		t.Run(e.Name(), func(t *testing.T) {
			if runtime.GOOS == "windows" && e.Name() == "run.yaml" {
				// run.yaml exercises Unix shell semantics (source, echo -n, and file
				// projections) that the pure-Go shell interpreter handles differently on
				// Windows. The esc run command itself is covered on Linux/macOS.
				t.Skip("skipped on Windows: relies on Unix shell semantics")
			}

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

				var b bytes.Buffer
				enc := yaml.NewEncoder(&b)
				enc.SetIndent(2)
				err := enc.Encode(def)
				require.NoError(t, err)

				fmt.Fprintf(&b, "\n---\n")
				b.Write(stdout.Bytes())
				fmt.Fprintf(&b, "\n---\n")
				b.Write(stderr.Bytes())

				err = os.WriteFile(path, b.Bytes(), 0o600)
				require.NoError(t, err)

				return
			}

			if def.Error == "" {
				require.NoError(t, err)
			} else {
				assert.Error(t, err)
			}

			assert.Equal(t, testcase.expectedStdout, stdout.String())
			assert.Equal(t, testcase.expectedStderr, stderr.String())
		})
	}
}
