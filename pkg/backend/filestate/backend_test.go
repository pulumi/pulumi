package filestate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	user "github.com/tweekmonster/luser"
	"gocloud.dev/blob/fileblob"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/operations"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets/b64"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/encoding"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/testing/diagtest"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
)

func TestMassageBlobPath(t *testing.T) {
	t.Parallel()

	testMassagePath := func(t *testing.T, s string, want string) {
		massaged, err := massageBlobPath(s)
		assert.NoError(t, err)
		assert.Equal(t, want, massaged,
			"massageBlobPath(%s) didn't return expected result.\nWant: %q\nGot:  %q", s, want, massaged)
	}

	// URLs not prefixed with "file://" are kept as-is. Also why we add FilePathPrefix as a prefix for other tests.
	t.Run("NonFilePrefixed", func(t *testing.T) {
		t.Parallel()

		testMassagePath(t, "asdf-123", "asdf-123")
	})

	// The home directory is converted into the user's actual home directory.
	// Which requires even more tweaks to work on Windows.
	t.Run("PrefixedWithTilde", func(t *testing.T) {
		t.Parallel()

		usr, err := user.Current()
		if err != nil {
			t.Fatalf("Unable to get current user: %v", err)
		}

		homeDir := usr.HomeDir

		// When running on Windows, the "home directory" takes on a different meaning.
		if runtime.GOOS == "windows" {
			t.Logf("Running on %v", runtime.GOOS)

			t.Run("NormalizeDirSeparator", func(t *testing.T) {
				t.Parallel()

				testMassagePath(t, FilePathPrefix+`C:\Users\steve\`, FilePathPrefix+"/C:/Users/steve")
			})

			newHomeDir := "/" + filepath.ToSlash(homeDir)
			t.Logf("Changed homeDir to expect from %q to %q", homeDir, newHomeDir)
			homeDir = newHomeDir
		}

		testMassagePath(t, FilePathPrefix+"~", FilePathPrefix+homeDir)
		testMassagePath(t, FilePathPrefix+"~/alpha/beta", FilePathPrefix+homeDir+"/alpha/beta")
	})

	t.Run("MakeAbsolute", func(t *testing.T) {
		t.Parallel()

		// Run the expected result through filepath.Abs, since on Windows we expect "C:\1\2".
		expected := "/1/2"
		abs, err := filepath.Abs(expected)
		assert.NoError(t, err)

		expected = filepath.ToSlash(abs)
		if expected[0] != '/' {
			expected = "/" + expected // A leading slash is added on Windows.
		}

		testMassagePath(t, FilePathPrefix+"/1/2/3/../4/..", FilePathPrefix+expected)
	})
}

func TestGetLogsForTargetWithNoSnapshot(t *testing.T) {
	t.Parallel()

	target := &deploy.Target{
		Name:      "test",
		Config:    config.Map{},
		Decrypter: config.NopDecrypter,
		Snapshot:  nil,
	}
	query := operations.LogQuery{}
	res, err := GetLogsForTarget(target, query)
	assert.NoError(t, err)
	assert.Nil(t, res)
}

func makeUntypedDeployment(name tokens.QName, phrase, state string) (*apitype.UntypedDeployment, error) {
	return makeUntypedDeploymentTimestamp(name, phrase, state, nil, nil)
}

func makeUntypedDeploymentTimestamp(
	name tokens.QName,
	phrase, state string,
	created, modified *time.Time,
) (*apitype.UntypedDeployment, error) {
	sm, err := passphrase.NewPassphraseSecretsManager(phrase, state)
	if err != nil {
		return nil, err
	}

	resources := []*resource.State{
		{
			URN:  resource.NewURN("a", "proj", "d:e:f", "a:b:c", name),
			Type: "a:b:c",
			Inputs: resource.PropertyMap{
				resource.PropertyKey("secret"): resource.MakeSecret(resource.NewStringProperty("s3cr3t")),
			},
			Created:  created,
			Modified: modified,
		},
	}

	snap := deploy.NewSnapshot(deploy.Manifest{}, sm, resources, nil)

	sdep, err := stack.SerializeDeployment(snap, snap.SecretsManager, false /* showSecrsts */)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(sdep)
	if err != nil {
		return nil, err
	}

	return &apitype.UntypedDeployment{
		Version:    3,
		Deployment: json.RawMessage(data),
	}, nil
}

//nolint:paralleltest // mutates environment variables
func TestListStacksWithMultiplePassphrases(t *testing.T) {
	// Login to a temp dir filestate backend
	tmpDir := t.TempDir()
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	assert.NoError(t, err)

	// Create stack "a" and import a checkpoint with a secret
	aStackRef, err := b.ParseStackReference("a")
	assert.NoError(t, err)
	aStack, err := b.CreateStack(ctx, aStackRef, "", nil)
	assert.NoError(t, err)
	assert.NotNil(t, aStack)
	defer func() {
		t.Setenv("PULUMI_CONFIG_PASSPHRASE", "abc123")
		_, err := b.RemoveStack(ctx, aStack, true)
		assert.NoError(t, err)
	}()
	deployment, err := makeUntypedDeployment("a", "abc123",
		"v1:4iF78gb0nF0=:v1:Co6IbTWYs/UdrjgY:FSrAWOFZnj9ealCUDdJL7LrUKXX9BA==")
	assert.NoError(t, err)
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "abc123")
	err = b.ImportDeployment(ctx, aStack, deployment)
	assert.NoError(t, err)

	// Create stack "b" and import a checkpoint with a secret
	bStackRef, err := b.ParseStackReference("b")
	assert.NoError(t, err)
	bStack, err := b.CreateStack(ctx, bStackRef, "", nil)
	assert.NoError(t, err)
	assert.NotNil(t, bStack)
	defer func() {
		t.Setenv("PULUMI_CONFIG_PASSPHRASE", "123abc")
		_, err := b.RemoveStack(ctx, bStack, true)
		assert.NoError(t, err)
	}()
	deployment, err = makeUntypedDeployment("b", "123abc",
		"v1:C7H2a7/Ietk=:v1:yfAd1zOi6iY9DRIB:dumdsr+H89VpHIQWdB01XEFqYaYjAg==")
	assert.NoError(t, err)
	t.Setenv("PULUMI_CONFIG_PASSPHRASE", "123abc")
	err = b.ImportDeployment(ctx, bStack, deployment)
	assert.NoError(t, err)

	// Remove the config passphrase so that we can no longer deserialize the checkpoints
	err = os.Unsetenv("PULUMI_CONFIG_PASSPHRASE")
	assert.NoError(t, err)

	// Ensure that we can list the stacks we created even without a passphrase
	stacks, outContToken, err := b.ListStacks(ctx, backend.ListStacksFilter{}, nil /* inContToken */)
	assert.NoError(t, err)
	assert.Nil(t, outContToken)
	assert.Len(t, stacks, 2)
	for _, stack := range stacks {
		assert.NotNil(t, stack.ResourceCount())
		assert.Equal(t, 1, *stack.ResourceCount())
	}
}

func TestDrillError(t *testing.T) {
	t.Parallel()

	// Login to a temp dir filestate backend
	tmpDir := t.TempDir()
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	assert.NoError(t, err)

	// Get a non-existent stack and expect a nil error because it won't be found.
	stackRef, err := b.ParseStackReference("dev")
	if err != nil {
		t.Fatalf("unexpected error %v when parsing stack reference", err)
	}
	_, err = b.GetStack(ctx, stackRef)
	assert.Nil(t, err)
}

func TestCancel(t *testing.T) {
	t.Parallel()

	// Login to a temp dir filestate backend
	tmpDir := t.TempDir()
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	assert.NoError(t, err)

	// Check that trying to cancel a stack that isn't created yet doesn't error
	aStackRef, err := b.ParseStackReference("a")
	assert.NoError(t, err)
	err = b.CancelCurrentUpdate(ctx, aStackRef)
	assert.NoError(t, err)

	// Check that trying to cancel a stack that isn't locked doesn't error
	aStack, err := b.CreateStack(ctx, aStackRef, "", nil)
	assert.NoError(t, err)
	assert.NotNil(t, aStack)
	err = b.CancelCurrentUpdate(ctx, aStackRef)
	assert.NoError(t, err)

	// Locking and lock checks are only part of the internal interface
	lb, ok := b.(*localBackend)
	assert.True(t, ok)
	assert.NotNil(t, lb)

	// Lock the stack and check CancelCurrentUpdate deletes the lock file
	err = lb.Lock(ctx, aStackRef)
	assert.NoError(t, err)
	// check the lock file exists
	lockExists, err := lb.bucket.Exists(ctx, lb.lockPath(aStackRef))
	assert.NoError(t, err)
	assert.True(t, lockExists)
	// Call CancelCurrentUpdate
	err = lb.CancelCurrentUpdate(ctx, aStackRef)
	assert.NoError(t, err)
	// Now check the lock file no longer exists
	lockExists, err = lb.bucket.Exists(ctx, lb.lockPath(aStackRef))
	assert.NoError(t, err)
	assert.False(t, lockExists)

	// Make another filestate backend which will have a different lockId
	ob, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	assert.NoError(t, err)
	otherBackend, ok := ob.(*localBackend)
	assert.True(t, ok)
	assert.NotNil(t, lb)

	// Lock the stack with this new backend, then check that checkForLocks on the first backend now errors
	err = otherBackend.Lock(ctx, aStackRef)
	assert.NoError(t, err)
	err = lb.checkForLock(ctx, aStackRef)
	assert.Error(t, err)
	// Now call CancelCurrentUpdate and check that checkForLocks no longer errors
	err = lb.CancelCurrentUpdate(ctx, aStackRef)
	assert.NoError(t, err)
	err = lb.checkForLock(ctx, aStackRef)
	assert.NoError(t, err)
}

func TestRemoveMakesBackups(t *testing.T) {
	t.Parallel()

	// Login to a temp dir filestate backend
	tmpDir := t.TempDir()
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	assert.NoError(t, err)

	// Grab the bucket interface to test with
	lb, ok := b.(*localBackend)
	assert.True(t, ok)
	assert.NotNil(t, lb)

	// Check that creating a new stack doesn't make a backup file
	aStackRef, err := lb.parseStackReference("a")
	assert.NoError(t, err)
	aStack, err := b.CreateStack(ctx, aStackRef, "", nil)
	assert.NoError(t, err)
	assert.NotNil(t, aStack)

	// Check the stack file now exists, but the backup file doesn't
	stackFileExists, err := lb.bucket.Exists(ctx, lb.stackPath(aStackRef))
	assert.NoError(t, err)
	assert.True(t, stackFileExists)
	backupFileExists, err := lb.bucket.Exists(ctx, lb.stackPath(aStackRef)+".bak")
	assert.NoError(t, err)
	assert.False(t, backupFileExists)

	// Now remove the stack
	removed, err := b.RemoveStack(ctx, aStack, false)
	assert.NoError(t, err)
	assert.False(t, removed)

	// Check the stack file is now gone, but the backup file exists
	stackFileExists, err = lb.bucket.Exists(ctx, lb.stackPath(aStackRef))
	assert.NoError(t, err)
	assert.False(t, stackFileExists)
	backupFileExists, err = lb.bucket.Exists(ctx, lb.stackPath(aStackRef)+".bak")
	assert.NoError(t, err)
	assert.True(t, backupFileExists)
}

func TestRenameWorks(t *testing.T) {
	t.Parallel()

	// Login to a temp dir filestate backend
	tmpDir := t.TempDir()
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	assert.NoError(t, err)

	// Grab the bucket interface to test with
	lb, ok := b.(*localBackend)
	assert.True(t, ok)
	assert.NotNil(t, lb)

	// Create a new stack
	aStackRef, err := lb.parseStackReference("a")
	assert.NoError(t, err)
	aStack, err := b.CreateStack(ctx, aStackRef, "", nil)
	assert.NoError(t, err)
	assert.NotNil(t, aStack)

	// Check the stack file now exists
	stackFileExists, err := lb.bucket.Exists(ctx, lb.stackPath(aStackRef))
	assert.NoError(t, err)
	assert.True(t, stackFileExists)

	// Fake up some history
	err = lb.addToHistory(aStackRef, backend.UpdateInfo{Kind: apitype.DestroyUpdate})
	assert.NoError(t, err)
	// And pollute the history folder
	err = lb.bucket.WriteAll(ctx, path.Join(aStackRef.HistoryDir(), "randomfile.txt"), []byte{0, 13}, nil)
	assert.NoError(t, err)

	// Rename the stack
	bStackRefI, err := b.RenameStack(ctx, aStack, "b")
	assert.NoError(t, err)
	assert.Equal(t, "b", bStackRefI.String())
	bStackRef := bStackRefI.(*localBackendReference)

	// Check the new stack file now exists and the old one is gone
	stackFileExists, err = lb.bucket.Exists(ctx, lb.stackPath(bStackRef))
	assert.NoError(t, err)
	assert.True(t, stackFileExists)
	stackFileExists, err = lb.bucket.Exists(ctx, lb.stackPath(aStackRef))
	assert.NoError(t, err)
	assert.False(t, stackFileExists)

	// Rename again
	bStack, err := b.GetStack(ctx, bStackRef)
	assert.NoError(t, err)
	cStackRefI, err := b.RenameStack(ctx, bStack, "c")
	assert.NoError(t, err)
	assert.Equal(t, "c", cStackRefI.String())
	cStackRef := cStackRefI.(*localBackendReference)

	// Check the new stack file now exists and the old one is gone
	stackFileExists, err = lb.bucket.Exists(ctx, lb.stackPath(cStackRef))
	assert.NoError(t, err)
	assert.True(t, stackFileExists)
	stackFileExists, err = lb.bucket.Exists(ctx, lb.stackPath(bStackRef))
	assert.NoError(t, err)
	assert.False(t, stackFileExists)

	// Check we can still get the history
	history, err := b.GetHistory(ctx, cStackRef, 10, 0)
	assert.NoError(t, err)
	assert.Len(t, history, 1)
	assert.Equal(t, apitype.DestroyUpdate, history[0].Kind)
}

func TestLoginToNonExistingFolderFails(t *testing.T) {
	t.Parallel()

	fakeDir := "file://" + filepath.ToSlash(os.TempDir()) + "/non-existing"
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), fakeDir, nil)
	assert.Error(t, err)
	assert.Nil(t, b)
}

// TestParseEmptyStackFails demonstrates that ParseStackReference returns
// an error when the stack name is the empty string.TestParseEmptyStackFails
func TestParseEmptyStackFails(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	assert.NoError(t, err)

	_, err = b.ParseStackReference("")
	assert.Error(t, err)
}

// Regression test for https://github.com/pulumi/pulumi/issues/10439
func TestHtmlEscaping(t *testing.T) {
	t.Parallel()

	sm := b64.NewBase64SecretsManager()
	resources := []*resource.State{
		{
			URN:  resource.NewURN("a", "proj", "d:e:f", "a:b:c", "name"),
			Type: "a:b:c",
			Inputs: resource.PropertyMap{
				resource.PropertyKey("html"): resource.NewStringProperty("<html@tags>"),
			},
		},
	}

	snap := deploy.NewSnapshot(deploy.Manifest{}, sm, resources, nil)

	sdep, err := stack.SerializeDeployment(snap, snap.SecretsManager, false /* showSecrsts */)
	assert.NoError(t, err)

	data, err := encoding.JSON.Marshal(sdep)
	assert.NoError(t, err)

	// Ensure data has the string contents "<html@tags>"", not "\u003chtml\u0026tags\u003e"
	// ImportDeployment below should not modify the data
	assert.Contains(t, string(data), "<html@tags>")

	udep := &apitype.UntypedDeployment{
		Version:    3,
		Deployment: json.RawMessage(data),
	}

	// Login to a temp dir filestate backend
	tmpDir := t.TempDir()
	ctx := context.Background()
	b, err := New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(tmpDir), nil)
	assert.NoError(t, err)

	// Create stack "a" and import a checkpoint with a secret
	aStackRef, err := b.ParseStackReference("a")
	assert.NoError(t, err)
	aStack, err := b.CreateStack(ctx, aStackRef, "", nil)
	assert.NoError(t, err)
	assert.NotNil(t, aStack)
	err = b.ImportDeployment(ctx, aStack, udep)
	assert.NoError(t, err)

	// Ensure the file has the string contents "<html@tags>"", not "\u003chtml\u0026tags\u003e"

	// Grab the bucket interface to read the file with
	lb, ok := b.(*localBackend)
	assert.True(t, ok)
	assert.NotNil(t, lb)

	chkpath := lb.stackPath(aStackRef.(*localBackendReference))
	bytes, err := lb.bucket.ReadAll(context.Background(), chkpath)
	assert.NoError(t, err)
	state := string(bytes)
	assert.Contains(t, state, "<html@tags>")
}

func TestLocalBackendRejectsStackInitOptions(t *testing.T) {
	t.Parallel()
	// Here, we provide options that illegally specify a team on a
	// backend that does not support teams. We expect this to create
	// an error later when we call CreateStack.
	illegalOptions := &backend.CreateStackOptions{Teams: []string{"red-team"}}

	// • Create a mock local backend
	tmpDir := t.TempDir()
	dirURI := fmt.Sprintf("file://%s", filepath.ToSlash(tmpDir))
	local, err := New(context.Background(), diagtest.LogSink(t), dirURI, nil)
	assert.NoError(t, err)
	ctx := context.Background()

	// • Simulate `pulumi stack init`, passing non-nil init options
	fakeStackRef, err := local.ParseStackReference("foobar")
	assert.NoError(t, err)
	assert.Panics(t, func() {
		// • Expect a panic because the options provided illegally
		//   include a team.
		_, err := local.CreateStack(ctx, fakeStackRef, "", illegalOptions)
		assert.Fail(t, "This statement should be unreachable.")
		// The linter complains if we don't check this error, even though
		// the code should be unreachable.
		assert.NoError(t, err)
	})
}

func TestNew_unsupportedStoreVersion(t *testing.T) {
	t.Parallel()

	// Verifies that we fail to initialize a backend if the store version is
	// newer than the CLI version.

	stateDir := t.TempDir()
	bucket, err := fileblob.OpenBucket(stateDir, nil)
	require.NoError(t, err)

	// Set up a meta.yaml "from the future".
	ctx := context.Background()
	require.NoError(t,
		bucket.WriteAll(ctx, ".pulumi/meta.yaml", []byte("version: 999999999"), nil))

	_, err = New(ctx, diagtest.LogSink(t), "file://"+filepath.ToSlash(stateDir), nil)
	assert.ErrorContains(t, err, "state store unsupported")
	assert.ErrorContains(t, err, "'meta.yaml' version (999999999) is not supported")
}

// TestSerializeTimestampRFC3339 captures our expectations that Created and Modified will be serialized to
// RFC3339.
func TestSerializeTimestampRFC3339(t *testing.T) {
	t.Parallel()

	created := time.Now().UTC()
	modified := created.Add(time.Hour)

	deployment, err := makeUntypedDeploymentTimestamp("b", "123abc",
		"v1:C7H2a7/Ietk=:v1:yfAd1zOi6iY9DRIB:dumdsr+H89VpHIQWdB01XEFqYaYjAg==", &created, &modified)
	assert.NoError(t, err)

	createdStr := created.Format(time.RFC3339Nano)
	modifiedStr := modified.Format(time.RFC3339Nano)
	assert.Contains(t, string(deployment.Deployment), createdStr)
	assert.Contains(t, string(deployment.Deployment), modifiedStr)
}
