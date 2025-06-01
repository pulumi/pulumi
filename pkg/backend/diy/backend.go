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

package diy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofrs/uuid"

	"gocloud.dev/blob"
	_ "gocloud.dev/blob/azureblob" // driver for azblob://
	_ "gocloud.dev/blob/fileblob"  // driver for file://
	"gocloud.dev/blob/gcsblob"     // driver for gs://
	_ "gocloud.dev/blob/s3blob"    // driver for s3://
	_ "github.com/pulumi/pulumi/pkg/v3/backend/diy/postgres" // driver for postgres://
	"gocloud.dev/gcerrors"

	"github.com/pulumi/pulumi/pkg/v3/authhelpers"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	sdkDisplay "github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/operations"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/edit"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	"github.com/pulumi/pulumi/pkg/v3/util/nosleep"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

// UpgradeOptions customizes the behavior of the upgrade operation.
type UpgradeOptions struct {
	// ProjectsForDetachedStacks is an optional function that is able to
	// backfill project names for stacks that have no project specified otherwise.
	//
	// It is called with a list of stack names that have no project specified.
	// It should return a list of project names to use for each stack name
	// in the same order.
	// If a returned name is blank, the stack at that position will be skipped
	// in the upgrade process.
	//
	// The length of 'projects' MUST match the length of 'stacks'.
	// If it does not, the upgrade will panic.
	//
	// If this function is not specified,
	// stacks without projects will be skipped during the upgrade.
	ProjectsForDetachedStacks func(stacks []tokens.StackName) (projects []tokens.Name, err error)
}

// Backend extends the base backend interface with specific information about diy backends.
type Backend interface {
	backend.Backend
	diy() // at the moment, no diy specific info, so just use a marker function.

	// Upgrade to the latest state store version.
	Upgrade(ctx context.Context, opts *UpgradeOptions) error

	// Lock the specified stack reference in this backend.
	Lock(ctx context.Context, stackRef backend.StackReference) error
}

type diyBackend struct {
	d diag.Sink

	// originalURL is the URL provided when the diyBackend was initialized, for example
	// "file://~". url is a canonicalized version that should be used when persisting data.
	// (For example, replacing ~ with the home directory, making an absolute path, etc.)
	originalURL string
	url         string

	bucket Bucket
	mutex  sync.Mutex

	lockID string

	gzip bool

	Env env.Env

	// The current project, if any.
	currentProject atomic.Pointer[workspace.Project]

	// The store controls the layout of stacks in the backend.
	// We use different layouts based on the version of the backend
	// specified in the metadata file.
	// If the metadata file is missing, we use the legacy layout.
	store referenceStore
}

type diyBackendReference struct {
	name    tokens.StackName
	project tokens.Name

	// A thread-safe way to get the current project.
	// The function reference or the pointer returned by the function may be nil.
	currentProject func() *workspace.Project

	// referenceStore that created this reference.
	//
	// This is necessary because
	// the referenceStore for a backend may change over time,
	// but the store for this reference should not.
	store referenceStore
}

func (r *diyBackendReference) String() string {
	// If project is blank this is a legacy non-project scoped stack reference, just return the name.
	if r.project == "" {
		return r.name.String()
	}

	// If the user has asked us to fully qualify names, we won't elide any
	// information.
	if cmdutil.FullyQualifyStackNames {
		return fmt.Sprintf("organization/%s/%s", r.project, r.name)
	}

	if r.currentProject != nil {
		proj := r.currentProject()
		// For project scoped references when stringifying backend references,
		// we take the current project (if present) into account.
		// If the project names match, we can elide them.
		if proj != nil && string(r.project) == string(proj.Name) {
			return r.name.String()
		}
	}

	// Else return a new style fully qualified reference.
	return fmt.Sprintf("organization/%s/%s", r.project, r.name)
}

func (r *diyBackendReference) Name() tokens.StackName {
	return r.name
}

func (r *diyBackendReference) Project() (tokens.Name, bool) {
	return r.project, r.project != ""
}

func (r *diyBackendReference) Organization() (string, bool) {
	// DIY backend always uses "organization" as the fixed organization name
	return "organization", r.project != ""
}

func (r *diyBackendReference) FullyQualifiedName() tokens.QName {
	if r.project == "" {
		return r.name.Q()
	}
	return tokens.QName(fmt.Sprintf("organization/%s/%s", r.project, r.name))
}

// Helper methods that delegate to the underlying referenceStore.
func (r *diyBackendReference) Validate() error       { return r.store.ValidateReference(r) }
func (r *diyBackendReference) StackBasePath() string { return r.store.StackBasePath(r) }
func (r *diyBackendReference) HistoryDir() string    { return r.store.HistoryDir(r) }
func (r *diyBackendReference) BackupDir() string     { return r.store.BackupDir(r) }

func IsDIYBackendURL(urlstr string) bool {
	u, err := url.Parse(urlstr)
	if err != nil {
		return false
	}

	return blob.DefaultURLMux().ValidBucketScheme(u.Scheme)
}

const FilePathPrefix = "file://"

// New constructs a new diy backend,
// using the given URL as the root for storage.
// The URL must use one of the schemes supported by the go-cloud blob package.
// Thes inclue: file, s3, gs, azblob.
func New(ctx context.Context, d diag.Sink, originalURL string, project *workspace.Project) (Backend, error) {
	return newDIYBackend(ctx, d, originalURL, project, nil)
}

type diyBackendOptions struct {
	// Env specifies how to get environment variables.
	//
	// Defaults to env.Global
	Env env.Env
}

// newDIYBackend builds a diy backend implementation
// with the given options.
func newDIYBackend(
	ctx context.Context, d diag.Sink, originalURL string, project *workspace.Project,
	opts *diyBackendOptions,
) (*diyBackend, error) {
	if opts == nil {
		opts = &diyBackendOptions{}
	}
	if opts.Env == nil {
		opts.Env = env.Global()
	}

	if !IsDIYBackendURL(originalURL) {
		return nil, fmt.Errorf("diy URL %s has an illegal prefix; expected one of: %s",
			originalURL, strings.Join(blob.DefaultURLMux().BucketSchemes(), ", "))
	}

	u, err := massageBlobPath(originalURL)
	if err != nil {
		return nil, err
	}

	p, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	blobmux := blob.DefaultURLMux()

	// for gcp we want to support additional credentials
	// schemes on top of go-cloud's default credentials mux.
	if p.Scheme == gcsblob.Scheme {
		blobmux, err = authhelpers.GoogleCredentialsMux(ctx)
		if err != nil {
			return nil, err
		}
	}

	bucket, err := blobmux.OpenBucket(ctx, u)
	if err != nil {
		return nil, fmt.Errorf("unable to open bucket %s: %w", u, err)
	}

	if !strings.HasPrefix(u, FilePathPrefix) {
		bucketSubDir := strings.TrimLeft(p.Path, "/")
		if bucketSubDir != "" {
			if !strings.HasSuffix(bucketSubDir, "/") {
				bucketSubDir += "/"
			}

			bucket = blob.PrefixedBucket(bucket, bucketSubDir)
		}
	}

	// Allocate a unique lock ID for this backend instance.
	lockID, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	gzipCompression := opts.Env.GetBool(env.DIYBackendGzip)

	wbucket := &wrappedBucket{bucket: bucket}

	// Prevent accidental use of the unwrapped bucket.
	//
	//nolint:wastedassign
	bucket = nil

	backend := &diyBackend{
		d:           d,
		originalURL: originalURL,
		url:         u,
		bucket:      wbucket,
		lockID:      lockID.String(),
		gzip:        gzipCompression,
		Env:         opts.Env,
	}
	backend.currentProject.Store(project)

	// Read the Pulumi state metadata
	// and ensure that it is compatible with this version of the CLI.
	// The version in the metadata file informs which store we use.
	meta, err := ensurePulumiMeta(ctx, wbucket, opts.Env)
	if err != nil {
		return nil, err
	}

	// projectMode tracks whether the current state supports project-scoped stacks.
	// Historically, the diy backend did not support this.
	// To avoid breaking old stacks, we use legacy mode for existing states.
	// We use project mode only if one of the following is true:
	//
	//  - The state has a single .pulumi/meta.yaml file
	//    and the version is 1 or greater.
	//  - The state is entirely new
	//    so there's no risk of breaking old stacks.
	//
	// All actual logic of project mode vs legacy mode is handled by the referenceStore.
	// This boolean just helps us warn users about unmigrated stacks.
	var projectMode bool
	switch meta.Version {
	case 0:
		backend.store = newLegacyReferenceStore(wbucket)
	case 1:
		backend.store = newProjectReferenceStore(wbucket, backend.currentProject.Load)
		projectMode = true
	default:
		return nil, fmt.Errorf(
			"state store unsupported: 'meta.yaml' version (%d) is not supported "+
				"by this version of the Pulumi CLI", meta.Version)
	}

	// If we're not in project mode, or we've disabled the warning, we're done.
	if !projectMode || opts.Env.GetBool(env.DIYBackendNoLegacyWarning) {
		return backend, nil
	}
	// Otherwise, warn about any old stack files.
	// This is possible if a user creates a new stack with a new CLI,
	// or migrates it to project mode with `pulumi state upgrade`,
	// but someone else interacts with the same state with an old CLI.

	refs, err := newLegacyReferenceStore(wbucket).ListReferences(ctx)
	if err != nil {
		// If there's an error listing don't fail, just don't print the warnings
		return backend, nil
	}
	if len(refs) == 0 {
		return backend, nil
	}

	var msg strings.Builder
	msg.WriteString("Found legacy stack files in state store:\n")
	for _, ref := range refs {
		fmt.Fprintf(&msg, "  - %s\n", ref.Name())
	}
	msg.WriteString("Please run 'pulumi state upgrade' to migrate them to the new format.\n")
	msg.WriteString("Set PULUMI_DIY_BACKEND_NO_LEGACY_WARNING=1 to disable this warning.")
	d.Warningf(diag.Message("", msg.String()))
	return backend, nil
}

func (b *diyBackend) Upgrade(ctx context.Context, opts *UpgradeOptions) error {
	if opts == nil {
		opts = &UpgradeOptions{}
	}

	// We don't use the existing b.store because
	// this may already be a projectReferenceStore
	// with new legacy files introduced to it accidentally.
	olds, err := newLegacyReferenceStore(b.bucket).ListReferences(ctx)
	if err != nil {
		return fmt.Errorf("read old references: %w", err)
	}
	sort.Slice(olds, func(i, j int) bool {
		return olds[i].Name().String() < olds[j].Name().String()
	})

	// Get the parallel value from environment variable or use a default value
	parallel := b.getParallel()

	// We don't want to overload the system with too many concurrent upgrades.
	// // We'll run a fixed pool of goroutines to upgrade stacks.
	pool := newWorkerPool(parallel, len(olds))
	defer pool.Close()

	// Projects for each stack in `olds` in the same order.
	// projects[i] is the project name for olds[i].
	projects := make([]tokens.Name, len(olds))
	for idx, old := range olds {
		pool.Enqueue(func() error {
			project, err := b.guessProject(ctx, old)
			if err != nil {
				return fmt.Errorf("guess stack %s project: %w", old.Name(), err)
			}

			// No lock necessary;
			// projects is pre-allocated.
			projects[idx] = project
			return nil
		})
	}

	if err := pool.Wait(); err != nil {
		return err
	}

	// If there are any stacks without projects
	// and the user provided a callback to fill them,
	// use it to fill in the missing projects.
	if opts.ProjectsForDetachedStacks != nil {
		var (
			// Names of stacks in 'olds' that don't have a project
			detached []tokens.StackName

			// reverseIdx[i] is the index of detached[i]
			// in olds and projects.
			//
			// In other words:
			//
			//   detached[i] == olds[reverseIdx[i]].Name()
			//   projects[reverseIdx[i]] == ""
			reverseIdx []int
		)
		for i, ref := range olds {
			if projects[i] == "" {
				detached = append(detached, ref.Name())
				reverseIdx = append(reverseIdx, i)
			}
		}

		if len(detached) != 0 {
			detachedProjects, err := opts.ProjectsForDetachedStacks(detached)
			if err != nil {
				return err
			}
			contract.Assertf(len(detached) == len(detachedProjects),
				"ProjectsForDetachedStacks returned the wrong number of projects: "+
					"expected %d, got %d", len(detached), len(detachedProjects))

			for i, project := range detachedProjects {
				projects[reverseIdx[i]] = project
			}
		}
	}

	// It's important that we attempt to write the new metadata file
	// before we attempt the upgrade.
	// This ensures that if permissions are borked for any reason,
	// (e.g., we can write to .pulumi/*/*" but not ".pulumi/*.")
	// we don't leave the bucket in a completely inaccessible state.
	meta := pulumiMeta{Version: 1}
	if err := meta.WriteTo(ctx, b.bucket); err != nil {
		var s strings.Builder
		fmt.Fprintf(&s, "Could not write new state metadata file: %v\n", err)
		fmt.Fprintf(&s, "Please verify that the storage is writable, and try again.")
		b.d.Errorf(diag.RawMessage("", s.String()))
		return errors.New("state upgrade failed")
	}

	newStore := newProjectReferenceStore(b.bucket, b.currentProject.Load)

	var upgraded atomic.Int64 // number of stacks successfully upgraded
	for idx, old := range olds {
		idx, old := idx, old
		pool.Enqueue(func() error {
			project := projects[idx]
			if project == "" {
				b.d.Warningf(diag.Message("", "Skipping stack %q: no project name found"), old)
				return nil
			}

			if err := b.upgradeStack(ctx, newStore, project, old); err != nil {
				b.d.Warningf(diag.Message("", "Skipping stack %q: %v"), old, err)
			} else {
				upgraded.Add(1)
			}
			return nil
		})
	}

	// We log all errors above. This should never fail.
	err = pool.Wait()
	contract.AssertNoErrorf(err, "pool.Wait should never return an error")

	b.store = newStore
	b.d.Infoerrf(diag.Message("", "Upgraded %d stack(s) to project mode"), upgraded.Load())
	return nil
}

// guessProject inspects the checkpoint for the given stack and attempts to
// guess the project name for it.
// Returns an empty string if the project name cannot be determined.
func (b *diyBackend) guessProject(ctx context.Context, old *diyBackendReference) (tokens.Name, error) {
	contract.Requiref(old.project == "", "old.project", "must be empty")

	chk, err := b.getCheckpoint(ctx, old)
	if err != nil {
		return "", fmt.Errorf("read checkpoint: %w", err)
	}

	// Try and find the project name from _any_ resource URN
	if chk.Latest != nil {
		for _, res := range chk.Latest.Resources {
			return tokens.Name(res.URN.Project()), nil
		}
	}
	return "", nil
}

// upgradeStack upgrades a single stack to use the provided projectReferenceStore.
func (b *diyBackend) upgradeStack(
	ctx context.Context,
	newStore *projectReferenceStore,
	project tokens.Name,
	old *diyBackendReference,
) error {
	contract.Requiref(old.project == "", "old.project", "must be empty")
	contract.Requiref(project != "", "project", "must not be empty")

	new := newStore.newReference(project, old.Name())
	if err := b.renameStack(ctx, old, new); err != nil {
		return fmt.Errorf("rename to %v: %w", new, err)
	}

	return nil
}

// massageBlobPath takes the path the user provided and converts it to an appropriate form go-cloud
// can support.  Importantly, s3/azblob/gs paths should not be be touched. This will only affect
// file:// paths which have a few oddities around them that we want to ensure work properly.
func massageBlobPath(path string) (string, error) {
	if !strings.HasPrefix(path, FilePathPrefix) {
		// Not a file:// path.  Keep this untouched and pass directly to gocloud.
		return path, nil
	}

	// We need to set no_tmp_dir to a value to avoid using the system temp directory.
	// See also https://github.com/pulumi/pulumi/issues/15352
	url, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("parsing the provided URL: %w", err)
	}
	query := url.Query()
	if query.Get("no_tmp_dir") == "" {
		query.Set("no_tmp_dir", "true")
	} else if query.Get("no_tmp_dir") == "false" {
		// If no_tmp_dir is set to false, we strip it out.  The library will default to false if
		// the parameter is not present, but will consider any value being set as true.
		query.Del("no_tmp_dir")
	}
	queryString := ""
	if len(query) > 0 {
		queryString = "?" + query.Encode()
	}

	// Strip off the "file://" portion so we can examine and determine what to do with the rest.
	path = strings.TrimPrefix(path, FilePathPrefix)
	// Strip off the query parameter, since we're computing that separately.
	path = strings.Split(path, "?")[0]

	// We need to specially handle ~.  The shell doesn't take care of this for us, and later
	// functions we run into can't handle this either.
	//
	// From https://stackoverflow.com/questions/17609732/expand-tilde-to-home-directory
	if strings.HasPrefix(path, "~") {
		usr, err := user.Current()
		if err != nil {
			return "", fmt.Errorf("Could not determine current user to resolve `file://~` path.: %w", err)
		}

		if path == "~" {
			path = usr.HomeDir
		} else {
			path = filepath.Join(usr.HomeDir, path[2:])
		}
	}

	// For file:// backend, ensure a relative path is resolved. fileblob only supports absolute paths.
	path, err = filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("An IO error occurred while building the absolute path: %w", err)
	}

	// Using example from https://godoc.org/gocloud.dev/blob/fileblob#example-package--OpenBucket
	// On Windows, convert "\" to "/" and add a leading "/". (See https://gocloud.dev/howto/blob/#local)
	path = filepath.ToSlash(path)
	if os.PathSeparator != '/' && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	return FilePathPrefix + path + queryString, nil
}

func Login(ctx context.Context, d diag.Sink, url string, project *workspace.Project) (Backend, error) {
	be, err := New(ctx, d, url, project)
	if err != nil {
		return nil, err
	}
	return be, workspace.StoreAccount(be.URL(), workspace.Account{}, true)
}

func (b *diyBackend) getReference(ref backend.StackReference) (*diyBackendReference, error) {
	stackRef, ok := ref.(*diyBackendReference)
	if !ok {
		return nil, errors.New("bad stack reference type")
	}
	return stackRef, stackRef.Validate()
}

func (b *diyBackend) diy() {}

func (b *diyBackend) Name() string {
	name, err := os.Hostname()
	contract.IgnoreError(err)
	if name == "" {
		name = "diy"
	}
	return name
}

func (b *diyBackend) URL() string {
	return b.originalURL
}

func (b *diyBackend) SetCurrentProject(project *workspace.Project) {
	b.currentProject.Store(project)
}

func (b *diyBackend) GetPolicyPack(ctx context.Context, policyPack string,
	d diag.Sink,
) (backend.PolicyPack, error) {
	return nil, errors.New("DIY backend does not support resource policy")
}

func (b *diyBackend) ListPolicyGroups(ctx context.Context, orgName string, _ backend.ContinuationToken) (
	apitype.ListPolicyGroupsResponse, backend.ContinuationToken, error,
) {
	return apitype.ListPolicyGroupsResponse{}, nil, errors.New("DIY backend does not support resource policy")
}

func (b *diyBackend) ListPolicyPacks(ctx context.Context, orgName string, _ backend.ContinuationToken) (
	apitype.ListPolicyPacksResponse, backend.ContinuationToken, error,
) {
	return apitype.ListPolicyPacksResponse{}, nil, errors.New("DIY backend does not support resource policy")
}

func (b *diyBackend) SupportsTags() bool {
	return false
}

func (b *diyBackend) SupportsTemplates() bool {
	return false
}

func (b *diyBackend) SupportsOrganizations() bool {
	return false
}

func (b *diyBackend) SupportsProgress() bool {
	return false
}

func (b *diyBackend) SupportsDeployments() bool {
	return false
}

func (b *diyBackend) GetDefaultOrg(ctx context.Context) (string, error) {
	return "", nil
}

func (b *diyBackend) ParseStackReference(stackRef string) (backend.StackReference, error) {
	return b.parseStackReference(stackRef)
}

func (b *diyBackend) parseStackReference(stackRef string) (*diyBackendReference, error) {
	return b.store.ParseReference(stackRef)
}

// ValidateStackName verifies the stack name is valid for the diy backend.
func (b *diyBackend) ValidateStackName(stackRef string) error {
	_, err := b.ParseStackReference(stackRef)
	return err
}

func (b *diyBackend) DoesProjectExist(ctx context.Context, _ string, projectName string) (bool, error) {
	projStore, ok := b.store.(*projectReferenceStore)
	if !ok {
		// Legacy stores don't have projects
		// so the project does not exist.
		return false, nil
	}

	return projStore.ProjectExists(ctx, projectName)
}

// Confirm the specified stack's project doesn't contradict the meta.yaml of the current project.
// If the CWD is not in a Pulumi project, does not contradict.
// If the project name in Pulumi.yaml is "foo", a stack with a name of bar/foo should not work.
func currentProjectContradictsWorkspace(stack *diyBackendReference) bool {
	contract.Requiref(stack != nil, "stack", "is nil")

	if stack.project == "" {
		return false
	}

	projPath, err := workspace.DetectProjectPath()
	if err != nil {
		return false
	}

	if projPath == "" {
		return false
	}

	proj, err := workspace.LoadProject(projPath)
	if err != nil {
		return false
	}

	return proj.Name.String() != stack.project.String()
}

func (b *diyBackend) CreateStack(
	ctx context.Context,
	stackRef backend.StackReference,
	root string,
	initialState *apitype.UntypedDeployment,
	opts *backend.CreateStackOptions,
) (backend.Stack, error) {
	if opts != nil && len(opts.Teams) > 0 {
		return nil, backend.ErrTeamsNotSupported
	}
	if opts != nil && opts.Config != nil {
		return nil, backend.ErrConfigNotSupported
	}

	diyStackRef, err := b.getReference(stackRef)
	if err != nil {
		return nil, err
	}

	err = b.Lock(ctx, stackRef)
	if err != nil {
		return nil, err
	}
	defer b.Unlock(ctx, stackRef)

	if currentProjectContradictsWorkspace(diyStackRef) {
		return nil, fmt.Errorf("provided project name %q doesn't match Pulumi.yaml", diyStackRef.project)
	}

	stackName := diyStackRef.FullyQualifiedName()
	if stackName == "" {
		return nil, errors.New("invalid empty stack name")
	}

	if _, err := b.stackExists(ctx, diyStackRef); err == nil {
		return nil, &backenderr.StackAlreadyExistsError{StackName: string(stackName)}
	}

	_, err = b.saveStack(ctx, diyStackRef, nil)
	if err != nil {
		return nil, err
	}

	if initialState != nil {
		chk, err := stack.MarshalUntypedDeploymentToVersionedCheckpoint(stackName, initialState)
		if err != nil {
			return nil, err
		}

		_, _, err = b.saveCheckpoint(ctx, diyStackRef, chk)
		if err != nil {
			return nil, err
		}
	}

	stack := newStack(diyStackRef, b)
	b.d.Infof(diag.Message("", "Created stack '%s'"), stack.Ref())

	return stack, nil
}

func (b *diyBackend) GetStack(ctx context.Context, stackRef backend.StackReference) (backend.Stack, error) {
	diyStackRef, err := b.getReference(stackRef)
	if err != nil {
		return nil, err
	}

	_, err = b.stackExists(ctx, diyStackRef)
	if err != nil {
		if errors.Is(err, errCheckpointNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return newStack(diyStackRef, b), nil
}

func (b *diyBackend) ListStacks(
	ctx context.Context, filter backend.ListStacksFilter, _ backend.ContinuationToken) (
	[]backend.StackSummary, backend.ContinuationToken, error,
) {
	stacks, err := b.getStacks(ctx)
	if err != nil {
		return nil, nil, err
	}

	// Get the parallel value from environment variable or use a default value
	parallel := b.getParallel()

	// Note that the provided stack filter is only partially honored, since fields like organizations and tags
	// aren't persisted in the diy backend.
	filteredStacks := slice.Prealloc[*diyBackendReference](len(stacks))
	for _, stackRef := range stacks {
		// We can check for project name filter here, but be careful about legacy stores where project is always blank.
		stackProject, hasProject := stackRef.Project()
		if filter.Project != nil && hasProject && string(stackProject) != *filter.Project {
			continue
		}
		filteredStacks = append(filteredStacks, stackRef)
	}

	// Create a worker pool to process stacks in parallel
	pool := newWorkerPool(parallel, len(filteredStacks))
	defer pool.Close()

	// Create a slice to store results in the same order as filteredStacks
	type checkpointResult struct {
		ref *diyBackendReference
		chk *apitype.CheckpointV3
	}
	results := make([]checkpointResult, len(filteredStacks))

	// Enqueue work for each stack
	for i, stackRef := range filteredStacks {
		i, stackRef := i, stackRef // https://golang.org/doc/faq#closures_and_goroutines
		pool.Enqueue(func() error {
			// TODO: Improve getCheckpoint to return errCheckpointNotFound directly when the checkpoint doesn't exist,
			// instead of having to call stackExists separately.
			chk, err := b.getCheckpoint(ctx, stackRef)
			if err != nil {
				// First check if the checkpoint exists
				_, existsErr := b.stackExists(ctx, stackRef)
				if existsErr != nil {
					// If checkpoint not found, stack doesn't exist anymore, don't report an error
					if errors.Is(existsErr, errCheckpointNotFound) {
						return nil
					}
				}
				return err
			}
			results[i] = checkpointResult{ref: stackRef, chk: chk}
			return nil
		})
	}

	// Wait for all work to complete
	if err := pool.Wait(); err != nil {
		return nil, nil, err
	}

	// Collect valid results
	summaries := []backend.StackSummary{}
	for _, result := range results {
		if result.ref != nil { // Skip entries where processing failed or stack disappeared
			summaries = append(summaries, newDIYStackSummary(result.ref, result.chk))
		}
	}

	return summaries, nil, nil
}

func (b *diyBackend) RemoveStack(ctx context.Context, stack backend.Stack, force bool) (bool, error) {
	diyStackRef, err := b.getReference(stack.Ref())
	if err != nil {
		return false, err
	}

	err = b.Lock(ctx, diyStackRef)
	if err != nil {
		return false, err
	}
	defer b.Unlock(ctx, diyStackRef)

	checkpoint, err := b.getCheckpoint(ctx, diyStackRef)
	if err != nil {
		return false, err
	}

	// Don't remove stacks that still have resources.
	if !force && checkpoint != nil && checkpoint.Latest != nil && len(checkpoint.Latest.Resources) > 0 {
		return true, errors.New("refusing to remove stack because it still contains resources")
	}

	return false, b.removeStack(ctx, diyStackRef)
}

func (b *diyBackend) RenameStack(ctx context.Context, stack backend.Stack,
	newName tokens.QName,
) (backend.StackReference, error) {
	diyStackRef, err := b.getReference(stack.Ref())
	if err != nil {
		return nil, err
	}

	// Ensure the new stack name is valid.
	newRef, err := b.parseStackReference(string(newName))
	if err != nil {
		return nil, err
	}

	err = b.renameStack(ctx, diyStackRef, newRef)
	if err != nil {
		return nil, err
	}

	return newRef, nil
}

func (b *diyBackend) renameStack(ctx context.Context, oldRef *diyBackendReference,
	newRef *diyBackendReference,
) error {
	err := b.Lock(ctx, oldRef)
	if err != nil {
		return err
	}
	defer b.Unlock(ctx, oldRef)

	// Ensure the destination stack does not already exist.
	hasExisting, err := b.bucket.Exists(ctx, b.stackPath(ctx, newRef))
	if err != nil {
		return err
	}
	if hasExisting {
		return fmt.Errorf("a stack named %s already exists", newRef.String())
	}

	// Get the current state from the stack to be renamed.
	chk, err := b.getCheckpoint(ctx, oldRef)
	if err != nil {
		return fmt.Errorf("failed to load checkpoint: %w", err)
	}

	// If we have a checkpoint, we need to rename the URNs inside it to use the new stack name.
	if chk != nil && chk.Latest != nil {
		project, has := newRef.Project()
		contract.Assertf(has || project == "", "project should be blank for legacy stacks")
		if err = edit.RenameStack(chk.Latest, newRef.name, tokens.PackageName(project)); err != nil {
			return err
		}
	}

	chkJSON, err := encoding.JSON.Marshal(chk)
	if err != nil {
		return fmt.Errorf("marshalling checkpoint: %w", err)
	}

	versionedCheckpoint := &apitype.VersionedCheckpoint{
		Version:    apitype.DeploymentSchemaVersionCurrent,
		Checkpoint: json.RawMessage(chkJSON),
	}

	// Now save the snapshot with a new name (we pass nil to re-use the existing secrets manager from the snapshot).
	if _, _, err = b.saveCheckpoint(ctx, newRef, versionedCheckpoint); err != nil {
		return err
	}

	// To remove the old stack, just make a backup of the file and don't write out anything new.
	file := b.stackPath(ctx, oldRef)
	backupTarget(ctx, b.bucket, file, false)

	// And rename the history folder as well.
	if err = b.renameHistory(ctx, oldRef, newRef); err != nil {
		return err
	}
	return err
}

func (b *diyBackend) GetLatestConfiguration(ctx context.Context,
	stack backend.Stack,
) (config.Map, error) {
	hist, err := b.GetHistory(ctx, stack.Ref(), 1 /*pageSize*/, 1 /*page*/)
	if err != nil {
		return nil, err
	}
	if len(hist) == 0 {
		return nil, backenderr.ErrNoPreviousDeployment
	}

	return hist[0].Config, nil
}

func (b *diyBackend) PackPolicies(
	ctx context.Context, policyPackRef backend.PolicyPackReference,
	cancellationScopes backend.CancellationScopeSource,
	callerEventsOpt chan<- engine.Event,
) error {
	return errors.New("DIY backend does not support resource policy")
}

func (b *diyBackend) Preview(ctx context.Context, stack backend.Stack,
	op backend.UpdateOperation, events chan<- engine.Event,
) (*deploy.Plan, sdkDisplay.ResourceChanges, error) {
	// We can skip PreviewThenPromptThenExecute and just go straight to Execute.
	opts := backend.ApplierOptions{
		DryRun:   true,
		ShowLink: true,
	}
	return b.apply(ctx, apitype.PreviewUpdate, stack, op, opts, events)
}

func (b *diyBackend) Update(ctx context.Context, stack backend.Stack,
	op backend.UpdateOperation, events chan<- engine.Event,
) (sdkDisplay.ResourceChanges, error) {
	err := b.Lock(ctx, stack.Ref())
	if err != nil {
		return nil, err
	}
	defer b.Unlock(ctx, stack.Ref())

	return backend.PreviewThenPromptThenExecute(ctx, apitype.UpdateUpdate, stack, op, b.apply, nil, events)
}

func (b *diyBackend) Import(ctx context.Context, stack backend.Stack,
	op backend.UpdateOperation, imports []deploy.Import,
) (sdkDisplay.ResourceChanges, error) {
	err := b.Lock(ctx, stack.Ref())
	if err != nil {
		return nil, err
	}
	defer b.Unlock(ctx, stack.Ref())

	op.Imports = imports

	if op.Opts.PreviewOnly {
		// We can skip PreviewThenPromptThenExecute, and just go straight to Execute.
		opts := backend.ApplierOptions{
			DryRun:   true,
			ShowLink: true,
		}

		op.Opts.Engine.GeneratePlan = false
		_, changes, err := b.apply(
			ctx, apitype.ResourceImportUpdate, stack, op, opts, nil /*events*/)
		return changes, err
	}

	return backend.PreviewThenPromptThenExecute(ctx, apitype.ResourceImportUpdate, stack, op, b.apply, nil, nil)
}

func (b *diyBackend) Refresh(ctx context.Context, stack backend.Stack,
	op backend.UpdateOperation,
) (sdkDisplay.ResourceChanges, error) {
	err := b.Lock(ctx, stack.Ref())
	if err != nil {
		return nil, err
	}
	defer b.Unlock(ctx, stack.Ref())

	if op.Opts.PreviewOnly {
		// We can skip PreviewThenPromptThenExecute, and just go straight to Execute.
		opts := backend.ApplierOptions{
			DryRun:   true,
			ShowLink: true,
		}

		op.Opts.Engine.GeneratePlan = false
		_, changes, err := b.apply(
			ctx, apitype.RefreshUpdate, stack, op, opts, nil /*events*/)
		return changes, err
	}

	return backend.PreviewThenPromptThenExecute(ctx, apitype.RefreshUpdate, stack, op, b.apply, nil, nil)
}

func (b *diyBackend) Destroy(ctx context.Context, stack backend.Stack,
	op backend.UpdateOperation,
) (sdkDisplay.ResourceChanges, error) {
	err := b.Lock(ctx, stack.Ref())
	if err != nil {
		return nil, err
	}
	defer b.Unlock(ctx, stack.Ref())

	if op.Opts.PreviewOnly {
		// We can skip PreviewThenPromptThenExecute, and just go straight to Execute.
		opts := backend.ApplierOptions{
			DryRun:   true,
			ShowLink: true,
		}

		op.Opts.Engine.GeneratePlan = false
		_, changes, err := b.apply(
			ctx, apitype.DestroyUpdate, stack, op, opts, nil /*events*/)
		return changes, err
	}

	return backend.PreviewThenPromptThenExecute(ctx, apitype.DestroyUpdate, stack, op, b.apply, nil, nil)
}

func (b *diyBackend) Watch(ctx context.Context, stk backend.Stack,
	op backend.UpdateOperation, paths []string,
) error {
	return backend.Watch(ctx, b, stk, op, b.apply, paths)
}

// apply actually performs the provided type of update on a diy hosted stack.
func (b *diyBackend) apply(
	ctx context.Context, kind apitype.UpdateKind, stack backend.Stack,
	op backend.UpdateOperation, opts backend.ApplierOptions,
	events chan<- engine.Event,
) (*deploy.Plan, sdkDisplay.ResourceChanges, error) {
	resetKeepRunning := nosleep.KeepRunning()
	defer resetKeepRunning()
	stackRef := stack.Ref()
	diyStackRef, err := b.getReference(stackRef)
	if err != nil {
		return nil, nil, err
	}

	if currentProjectContradictsWorkspace(diyStackRef) {
		return nil, nil, fmt.Errorf("provided project name %q doesn't match Pulumi.yaml", diyStackRef.project)
	}

	actionLabel := backend.ActionLabel(kind, opts.DryRun)

	if !op.Opts.Display.JSONDisplay && op.Opts.Display.Type != display.DisplayWatch {
		// Print a banner so it's clear this is a diy deployment.
		fmt.Printf(op.Opts.Display.Color.Colorize(
			colors.SpecHeadline+"%s (%s):"+colors.Reset+"\n"), actionLabel, stackRef)
	}

	// Start the update.
	update, err := b.newUpdate(ctx, op.SecretsProvider, diyStackRef, op)
	if err != nil {
		return nil, nil, err
	}

	// Spawn a display loop to show events on the CLI.
	displayEvents := make(chan engine.Event)
	displayDone := make(chan bool)
	go display.ShowEvents(
		strings.ToLower(actionLabel), kind, stackRef.Name(), op.Proj.Name, "",
		displayEvents, displayDone, op.Opts.Display, opts.DryRun)

	// Create a separate event channel for engine events that we'll pipe to both listening streams.
	engineEvents := make(chan engine.Event)

	scope := op.Scopes.NewScope(engineEvents, opts.DryRun)
	eventsDone := make(chan bool)
	go func() {
		// Pull in all events from the engine and send them to the two listeners.
		for e := range engineEvents {
			displayEvents <- e

			// If the caller also wants to see the events, stream them there also.
			if events != nil {
				events <- e
			}
		}

		close(eventsDone)
	}()

	// Create the management machinery.
	// We only need a snapshot manager if we're doing an update.
	var manager *backend.SnapshotManager
	if kind != apitype.PreviewUpdate && !opts.DryRun {
		persister := b.newSnapshotPersister(ctx, diyStackRef)
		manager = backend.NewSnapshotManager(persister, op.SecretsManager, update.GetTarget().Snapshot)
	}
	engineCtx := &engine.Context{
		Cancel:          scope.Context(),
		Events:          engineEvents,
		SnapshotManager: manager,
		BackendClient:   backend.NewBackendClient(b, op.SecretsProvider),
	}

	// Perform the update
	start := time.Now().Unix()
	var plan *deploy.Plan
	var changes sdkDisplay.ResourceChanges
	var updateErr error
	switch kind {
	case apitype.PreviewUpdate:
		plan, changes, updateErr = engine.Update(update, engineCtx, op.Opts.Engine, true)
	case apitype.UpdateUpdate:
		_, changes, updateErr = engine.Update(update, engineCtx, op.Opts.Engine, opts.DryRun)
	case apitype.ResourceImportUpdate:
		_, changes, updateErr = engine.Import(update, engineCtx, op.Opts.Engine, op.Imports, opts.DryRun)
	case apitype.RefreshUpdate:
		if op.Opts.Engine.RefreshProgram {
			_, changes, updateErr = engine.RefreshV2(update, engineCtx, op.Opts.Engine, opts.DryRun)
		} else {
			_, changes, updateErr = engine.Refresh(update, engineCtx, op.Opts.Engine, opts.DryRun)
		}
	case apitype.DestroyUpdate:
		if op.Opts.Engine.DestroyProgram {
			_, changes, updateErr = engine.DestroyV2(update, engineCtx, op.Opts.Engine, opts.DryRun)
		} else {
			_, changes, updateErr = engine.Destroy(update, engineCtx, op.Opts.Engine, opts.DryRun)
		}
	case apitype.StackImportUpdate, apitype.RenameUpdate:
		contract.Failf("unexpected %s event", kind)
	default:
		contract.Failf("Unrecognized update kind: %s", kind)
	}
	end := time.Now().Unix()

	// Wait for the display to finish showing all the events.
	<-displayDone
	scope.Close() // Don't take any cancellations anymore, we're shutting down.
	close(engineEvents)
	if manager != nil {
		err = manager.Close()
		// If the snapshot manager failed to close, we should return that error.
		// Even though all the parts of the operation have potentially succeeded, a
		// snapshotting failure is likely to rear its head on the next
		// operation/invocation (e.g. an invalid snapshot that fails integrity
		// checks, or a failure to write that means the snapshot is incomplete).
		// Reporting now should make debugging and reporting easier.
		if err != nil {
			return plan, changes, fmt.Errorf("writing snapshot: %w", err)
		}
	}

	// Make sure the goroutine writing to displayEvents and events has exited before proceeding.
	<-eventsDone
	close(displayEvents)

	// Save update results.
	backendUpdateResult := backend.SucceededResult
	if updateErr != nil {
		backendUpdateResult = backend.FailedResult
	}
	info := backend.UpdateInfo{
		Kind:        kind,
		StartTime:   start,
		Message:     op.M.Message,
		Environment: op.M.Environment,
		Config:      update.GetTarget().Config,
		Result:      backendUpdateResult,
		EndTime:     end,
		// IDEA: it would be nice to populate the *Deployment, so that addToHistory below doesn't need to
		//     rudely assume it knows where the checkpoint file is on disk as it makes a copy of it.  This isn't
		//     trivial to achieve today given the event driven nature of plan-walking, however.
		ResourceChanges: changes,
	}

	var saveErr error
	var backupErr error
	if !opts.DryRun {
		saveErr = b.addToHistory(ctx, diyStackRef, info)
		backupErr = b.backupStack(ctx, diyStackRef)
	}

	if updateErr != nil {
		// We swallow saveErr and backupErr as they are less important than the updateErr.
		return plan, changes, updateErr
	}

	if saveErr != nil {
		// We swallow backupErr as it is less important than the saveErr.
		return plan, changes, fmt.Errorf("saving update info: %w", saveErr)
	}

	if backupErr != nil {
		return plan, changes, fmt.Errorf("saving backup: %w", backupErr)
	}

	// Make sure to print a link to the stack's checkpoint before exiting.
	if !op.Opts.Display.SuppressPermalink && opts.ShowLink && !op.Opts.Display.JSONDisplay {
		// Note we get a real signed link for aws/azure/gcp links.  But no such option exists for
		// file:// links so we manually create the link ourselves.
		var link string
		if strings.HasPrefix(b.url, FilePathPrefix) {
			u, _ := url.Parse(b.url)
			u.Path = filepath.ToSlash(path.Join(u.Path, b.stackPath(ctx, diyStackRef)))
			link = u.String()
		} else {
			link, err = b.bucket.SignedURL(ctx, b.stackPath(ctx, diyStackRef), nil)
			if err != nil {
				// set link to be empty to when there is an error to hide use of Permalinks
				link = ""

				// we log a warning here rather then returning an error to avoid exiting
				// pulumi with an error code.
				// printing a statefile perma link happens after all the providers have finished
				// deploying the infrastructure, failing the pulumi update because there was a
				// problem printing a statefile perma link can be missleading in automated CI environments.
				cmdutil.Diag().Warningf(diag.Message("", "Unable to create signed url for current backend to "+
					"create a Permalink. Please visit https://www.pulumi.com/docs/troubleshooting/ "+
					"for more information\n"))
			}
		}

		if link != "" {
			fmt.Printf(op.Opts.Display.Color.Colorize(
				colors.SpecHeadline+"Permalink: "+
					colors.Underline+colors.BrightBlue+"%s"+colors.Reset+"\n"), link)
		}
	}

	return plan, changes, nil
}

func (b *diyBackend) GetHistory(
	ctx context.Context,
	stackRef backend.StackReference,
	pageSize int,
	page int,
) ([]backend.UpdateInfo, error) {
	diyStackRef, err := b.getReference(stackRef)
	if err != nil {
		return nil, err
	}
	updates, err := b.getHistory(ctx, diyStackRef, pageSize, page)
	if err != nil {
		return nil, err
	}
	return updates, nil
}

func (b *diyBackend) GetLogs(ctx context.Context,
	secretsProvider secrets.Provider, stack backend.Stack, cfg backend.StackConfiguration,
	query operations.LogQuery,
) ([]operations.LogEntry, error) {
	diyStackRef, err := b.getReference(stack.Ref())
	if err != nil {
		return nil, err
	}

	target, err := b.getTarget(ctx, secretsProvider, diyStackRef, cfg.Config, cfg.Decrypter)
	if err != nil {
		return nil, err
	}

	return GetLogsForTarget(target, query)
}

// GetLogsForTarget fetches stack logs using the config, decrypter, and checkpoint in the given target.
func GetLogsForTarget(target *deploy.Target, query operations.LogQuery) ([]operations.LogEntry, error) {
	contract.Requiref(target != nil, "target", "must not be nil")

	if target.Snapshot == nil {
		// If the stack has not been deployed yet, return no logs.
		return nil, nil
	}

	config, err := target.Config.Decrypt(target.Decrypter)
	if err != nil {
		return nil, err
	}

	components := operations.NewResourceTree(target.Snapshot.Resources)
	ops := components.OperationsProvider(config)
	logs, err := ops.GetLogs(query)
	if logs == nil {
		return nil, err
	}
	return *logs, err
}

func (b *diyBackend) ExportDeployment(ctx context.Context,
	stk backend.Stack,
) (*apitype.UntypedDeployment, error) {
	diyStackRef, err := b.getReference(stk.Ref())
	if err != nil {
		return nil, err
	}

	chk, err := b.getCheckpoint(ctx, diyStackRef)
	if err != nil {
		return nil, fmt.Errorf("failed to load checkpoint: %w", err)
	}

	data, err := encoding.JSON.Marshal(chk.Latest)
	if err != nil {
		return nil, err
	}

	return &apitype.UntypedDeployment{
		Version:    3,
		Deployment: json.RawMessage(data),
	}, nil
}

func (b *diyBackend) ImportDeployment(ctx context.Context, stk backend.Stack,
	deployment *apitype.UntypedDeployment,
) error {
	diyStackRef, err := b.getReference(stk.Ref())
	if err != nil {
		return err
	}

	err = b.Lock(ctx, diyStackRef)
	if err != nil {
		return err
	}
	defer b.Unlock(ctx, diyStackRef)

	stackName := diyStackRef.FullyQualifiedName()
	chk, err := stack.MarshalUntypedDeploymentToVersionedCheckpoint(stackName, deployment)
	if err != nil {
		return err
	}

	_, _, err = b.saveCheckpoint(ctx, diyStackRef, chk)
	return err
}

func (b *diyBackend) CurrentUser() (string, []string, *workspace.TokenInformation, error) {
	user, err := user.Current()
	if err != nil {
		return "", nil, nil, err
	}
	return user.Username, nil, nil, nil
}

func (b *diyBackend) getStacks(ctx context.Context) ([]*diyBackendReference, error) {
	return b.store.ListReferences(ctx)
}

// UpdateStackTags updates the stacks's tags, replacing all existing tags.
func (b *diyBackend) UpdateStackTags(ctx context.Context,
	stack backend.Stack, tags map[apitype.StackTagName]string,
) error {
	// The diy backend does not currently persist tags.
	return errors.New("stack tags not supported in diy mode")
}

func (b *diyBackend) EncryptStackDeploymentSettingsSecret(ctx context.Context,
	stack backend.Stack, secret string,
) (*apitype.SecretValue, error) {
	// The local backend does not support managing deployments.
	return nil, errors.New("stack deployments not supported with diy backends")
}

func (b *diyBackend) UpdateStackDeploymentSettings(ctx context.Context, stack backend.Stack,
	deployment apitype.DeploymentSettings,
) error {
	// The local backend does not support managing deployments.
	return errors.New("stack deployments not supported with diy backends")
}

func (b *diyBackend) DestroyStackDeploymentSettings(ctx context.Context, stack backend.Stack) error {
	// The local backend does not support managing deployments.
	return errors.New("stack deployments not supported with diy backends")
}

func (b *diyBackend) GetGHAppIntegration(
	ctx context.Context, stack backend.Stack,
) (*apitype.GitHubAppIntegration, error) {
	// The local backend does not support github integration.
	return nil, errors.New("github integration not supported with diy backends")
}

func (b *diyBackend) GetStackDeploymentSettings(ctx context.Context,
	stack backend.Stack,
) (*apitype.DeploymentSettings, error) {
	// The local backend does not support managing deployments.
	return nil, errors.New("stack deployments not supported with diy backends")
}

func (b *diyBackend) ListTemplates(context.Context, string) (apitype.ListOrgTemplatesResponse, error) {
	return apitype.ListOrgTemplatesResponse{}, errors.New("list templates not supported with diy backends")
}

func (b *diyBackend) DownloadTemplate(context.Context, string, string) (backend.TarReaderCloser, error) {
	return nil, errors.New("download template not supported with diy backends")
}

func (b *diyBackend) CancelCurrentUpdate(ctx context.Context, stackRef backend.StackReference) error {
	// Try to delete ALL the lock files
	allFiles, err := listBucket(ctx, b.bucket, stackLockDir(stackRef.FullyQualifiedName()))
	if err != nil {
		// Don't error if it just wasn't found
		if gcerrors.Code(err) == gcerrors.NotFound {
			return nil
		}
		return err
	}

	for _, file := range allFiles {
		if file.IsDir {
			continue
		}

		err := b.bucket.Delete(ctx, file.Key)
		if err != nil {
			// Race condition, don't error if the file was delete between us calling list and now
			if gcerrors.Code(err) == gcerrors.NotFound {
				return nil
			}
			return err
		}
	}

	return nil
}

func (b *diyBackend) DefaultSecretManager(ps *workspace.ProjectStack) (secrets.Manager, error) {
	// The default secrets manager for stacks against a DIY backend is a
	// passphrase-based manager.
	return passphrase.NewPromptingPassphraseSecretsManager(ps, false /* rotateSecretsProvider */)
}

// getParallel returns the number of parallel operations to use from the environment
// or a default value if not specified.
func (b *diyBackend) getParallel() int {
	parallel := b.Env.GetInt(env.DIYBackendParallel)
	if parallel <= 0 {
		// Default to 10 parallel operations if not specified.
		// This is just a sensible guess and hasn't been tuned for optimal performance.
		parallel = 10
	}
	return parallel
}

func (b *diyBackend) GetPackageRegistry() (backend.PackageRegistry, error) {
	return nil, errors.New("package registry is not supported by diy backends")
}
