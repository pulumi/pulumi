package filestate

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	user "github.com/tweekmonster/luser"

	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/operations"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/resource/stack"
	"github.com/pulumi/pulumi/pkg/v3/secrets/passphrase"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
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
	sm, err := passphrase.NewPassphaseSecretsManager(phrase, state)
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
	tmpDir, err := ioutil.TempDir("", "filestatebackend")
	assert.NoError(t, err)
	b, err := New(cmdutil.Diag(), "file://"+filepath.ToSlash(tmpDir))
	assert.NoError(t, err)
	ctx := context.Background()

	// Create stack "a" and import a checkpoint with a secret
	aStackRef, err := b.ParseStackReference("a")
	assert.NoError(t, err)
	aStack, err := b.CreateStack(ctx, aStackRef, nil)
	assert.NoError(t, err)
	assert.NotNil(t, aStack)
	defer func() {
		err = os.Setenv("PULUMI_CONFIG_PASSPHRASE", "abc123")
		_, err := b.RemoveStack(ctx, aStack, true)
		assert.NoError(t, err)
	}()
	deployment, err := makeUntypedDeployment("a", "abc123",
		"v1:4iF78gb0nF0=:v1:Co6IbTWYs/UdrjgY:FSrAWOFZnj9ealCUDdJL7LrUKXX9BA==")
	assert.NoError(t, err)
	err = os.Setenv("PULUMI_CONFIG_PASSPHRASE", "abc123")
	assert.NoError(t, err)
	err = b.ImportDeployment(ctx, aStack, deployment)
	assert.NoError(t, err)

	// Create stack "b" and import a checkpoint with a secret
	bStackRef, err := b.ParseStackReference("b")
	assert.NoError(t, err)
	bStack, err := b.CreateStack(ctx, bStackRef, nil)
	assert.NoError(t, err)
	assert.NotNil(t, bStack)
	defer func() {
		err = os.Setenv("PULUMI_CONFIG_PASSPHRASE", "123abc")
		_, err := b.RemoveStack(ctx, bStack, true)
		assert.NoError(t, err)
	}()
	deployment, err = makeUntypedDeployment("b", "123abc",
		"v1:C7H2a7/Ietk=:v1:yfAd1zOi6iY9DRIB:dumdsr+H89VpHIQWdB01XEFqYaYjAg==")
	assert.NoError(t, err)
	err = os.Setenv("PULUMI_CONFIG_PASSPHRASE", "123abc")
	assert.NoError(t, err)
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
	tmpDir, err := ioutil.TempDir("", "filestatebackend")
	assert.NoError(t, err)
	b, err := New(cmdutil.Diag(), "file://"+filepath.ToSlash(tmpDir))
	assert.NoError(t, err)
	ctx := context.Background()

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
	tmpDir, err := ioutil.TempDir("", "filestatebackend")
	assert.NoError(t, err)
	b, err := New(cmdutil.Diag(), "file://"+filepath.ToSlash(tmpDir))
	assert.NoError(t, err)
	ctx := context.Background()

	// Check that trying to cancel a stack that isn't created yet doesn't error
	aStackRef, err := b.ParseStackReference("a")
	assert.NoError(t, err)
	err = b.CancelCurrentUpdate(ctx, aStackRef)
	assert.NoError(t, err)

	// Check that trying to cancel a stack that isn't locked doesn't error
	aStack, err := b.CreateStack(ctx, aStackRef, nil)
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
	lockExists, err := lb.bucket.Exists(ctx, lb.lockPath(aStackRef.Name()))
	assert.NoError(t, err)
	assert.True(t, lockExists)
	// Call CancelCurrentUpdate
	err = lb.CancelCurrentUpdate(ctx, aStackRef)
	assert.NoError(t, err)
	// Now check the lock file no longer exists
	lockExists, err = lb.bucket.Exists(ctx, lb.lockPath(aStackRef.Name()))
	assert.NoError(t, err)
	assert.False(t, lockExists)

	// Make another filestate backend which will have a different lockId
	ob, err := New(cmdutil.Diag(), "file://"+filepath.ToSlash(tmpDir))
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
