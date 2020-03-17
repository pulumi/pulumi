package filestate

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"gocloud.dev/blob"
)

func mustNotHaveError(t *testing.T, context string, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Error in testcase %q, aborting: %v", context, err)
	}
}

// The wrappedBucket type exists so that when we use the blob.Bucket type we can present a consistent
// view of file paths. Since it will assume that backslashes (file separators on Windows) are part of
// file names, and this causes "problems".
func TestWrappedBucket(t *testing.T) {
	// wrappedBucket will only massage file paths IFF it is needed, as filepath.ToSlash is a noop.
	if filepath.Separator == '/' {
		assert.Equal(t, `foo\bar\baz`, filepath.ToSlash(`foo\bar\baz`))
		t.Skip("Skipping wrappedBucket tests because file paths won't be modified.")
	}

	// Initialize a filestate backend, using the default Pulumi directory.
	cloudURL := FilePathPrefix + "~"
	b, err := New(nil, cloudURL)
	if err != nil {
		t.Fatalf("Initializing new filestate backend: %v", err)
	}
	localBackend, ok := b.(*localBackend)
	if !ok {
		t.Fatalf("backend wasn't of type localBackend?")
	}

	wrappedBucket, ok := localBackend.bucket.(*wrappedBucket)
	if !ok {
		t.Fatalf("localBackend.bucket wasn't of type wrappedBucket?")
	}

	ctx := context.Background()
	// Perform basic file operations using wrappedBucket and verify that it will
	// successfully handle both "/" and "\" as file separators. (And probably fail in
	// exciting ways if you try to give it a file on a system that supports "\" or "/" as
	// a valid character in a filename.)
	t.Run("SanityCheck", func(t *testing.T) {
		randomData := []byte("Just some random data")

		err := wrappedBucket.WriteAll(ctx, ".pulumi/bucket-test/foo", randomData, &blob.WriterOptions{})
		mustNotHaveError(t, "WriteAll", err)

		readData, err := wrappedBucket.ReadAll(ctx, `.pulumi\bucket-test\foo`)
		mustNotHaveError(t, "ReadAll", err)
		assert.EqualValues(t, randomData, readData, "data read from bucket doesn't match what was written")

		// Verify the leading slash isn't necessary.
		err = wrappedBucket.Delete(ctx, ".pulumi/bucket-test/foo")
		mustNotHaveError(t, "Delete", err)

		exists, err := wrappedBucket.Exists(ctx, ".pulumi/bucket-test/foo")
		mustNotHaveError(t, "Exists", err)
		assert.False(t, exists, "Deleted file still found?")
	})

	// Verify ListObjects / listBucket works with regard to differeing file separators too.
	t.Run("ListObjects", func(t *testing.T) {
		randomData := []byte("Just some random data")
		filenames := []string{"a.json", "b.json", "c.json"}

		// Write some data.
		for _, filename := range filenames {
			key := fmt.Sprintf(`.pulumi\bucket-test\%s`, filename)
			err := wrappedBucket.WriteAll(ctx, key, randomData, &blob.WriterOptions{})
			mustNotHaveError(t, "WriteAll", err)
		}

		// Verify it is found. NOTE: This requires that any files created
		// during other tests have successfully been cleaned up too.
		objects, err := listBucket(wrappedBucket, `.pulumi\bucket-test`)
		mustNotHaveError(t, "listBucket", err)
		if len(objects) != len(filenames) {
			assert.Equal(t, 3, len(objects), "listBucket returned unexpected number of objects.")
			for _, object := range objects {
				t.Logf("Got object: %+v", object)
			}
		}
	})
}
