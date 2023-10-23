package filestate

import (
	"context"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	s3V1 "github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/golang/glog"
	"github.com/pulumi/pulumi/pkg/v3/util"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"gocloud.dev/blob"
)

// Bucket is a wrapper around an underlying gocloud blob.Bucket.  It ensures that we pass all paths
// to it normalized to forward-slash form like it requires.
type Bucket interface {
	Copy(ctx context.Context, dstKey, srcKey string, opts *blob.CopyOptions) (err error)
	Delete(ctx context.Context, key string) (err error)
	List(opts *blob.ListOptions) *blob.ListIterator
	SignedURL(ctx context.Context, key string, opts *blob.SignedURLOptions) (string, error)
	ReadAll(ctx context.Context, key string) (_ []byte, err error)
	WriteAll(ctx context.Context, key string, p []byte, opts *blob.WriterOptions) (err error)
	Exists(ctx context.Context, key string) (bool, error)
}

// wrappedBucket encapsulates a true gocloud blob.Bucket, but ensures that all paths we send to it
// are appropriately normalized to use forward slashes as required by it.  Without this, we may use
// filepath.join which can make paths like `c:\temp\etc`.  gocloud's fileblob then converts those
// backslashes to the hex string __0x5c__, breaking things on windows completely.
type wrappedBucket struct {
	bucket *blob.Bucket
}

const (
	s3BucketEncryptionTypeEnvVarKey = "PULUMI_S3_BUCKET_ENCRYPTION_TYPE"
	s3BucketKmsKeyIDEnvVarKey       = "PULUMI_S3_BUCKET_ENCRYPTION_KMS_KEY_ID"
)

func (b *wrappedBucket) Copy(ctx context.Context, dstKey, srcKey string, opts *blob.CopyOptions) (err error) {
	var optsCopy blob.CopyOptions
	if opts != nil {
		optsCopy = *opts
	} else {
		optsCopy = blob.CopyOptions{}
	}
	optsCopy.BeforeCopy = beforeMutation
	return b.bucket.Copy(ctx, filepath.ToSlash(dstKey), filepath.ToSlash(srcKey), &optsCopy)
}

func (b *wrappedBucket) Delete(ctx context.Context, key string) (err error) {
	return b.bucket.Delete(ctx, filepath.ToSlash(key))
}

func (b *wrappedBucket) List(opts *blob.ListOptions) *blob.ListIterator {
	optsCopy := *opts
	optsCopy.Prefix = filepath.ToSlash(opts.Prefix)
	return b.bucket.List(&optsCopy)
}

func (b *wrappedBucket) SignedURL(ctx context.Context, key string, opts *blob.SignedURLOptions) (string, error) {
	return b.bucket.SignedURL(ctx, filepath.ToSlash(key), opts)
}

func (b *wrappedBucket) ReadAll(ctx context.Context, key string) (_ []byte, err error) {
	return b.bucket.ReadAll(ctx, filepath.ToSlash(key))
}

func (b *wrappedBucket) WriteAll(ctx context.Context, key string, p []byte, opts *blob.WriterOptions) (err error) {
	var optsCopy blob.WriterOptions
	if opts != nil {
		optsCopy = *opts
	} else {
		optsCopy = blob.WriterOptions{}
	}
	optsCopy.BeforeWrite = beforeMutation
	return b.bucket.WriteAll(ctx, filepath.ToSlash(key), p, &optsCopy)
}

func (b *wrappedBucket) Exists(ctx context.Context, key string) (bool, error) {
	return b.bucket.Exists(ctx, filepath.ToSlash(key))
}

// listBucket returns a list of all files in the bucket within a given directory. go-cloud sorts the results by key
func listBucket(ctx context.Context, bucket Bucket, dir string) ([]*blob.ListObject, error) {
	bucketIter := bucket.List(&blob.ListOptions{
		Delimiter: "/",
		Prefix:    dir + "/",
	})

	files := []*blob.ListObject{}

	for {
		file, err := bucketIter.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("could not list bucket: %w", err)
		}
		files = append(files, file)
	}

	return files, nil
}

// objectName returns the filename of a ListObject (an object from a bucket).
func objectName(obj *blob.ListObject) string {
	// If obj.Key ends in "/" we want to trim that to get the name just before
	key := strings.TrimSuffix(obj.Key, "/")
	_, filename := path.Split(key)
	return filename
}

// removeAllByPrefix deletes all objects with a given prefix (i.e. filepath)
func removeAllByPrefix(ctx context.Context, bucket Bucket, dir string) error {
	files, err := listBucket(ctx, bucket, dir)
	if err != nil {
		return fmt.Errorf("unable to list bucket objects for removal: %w", err)
	}

	for _, file := range files {
		err = bucket.Delete(ctx, file.Key)
		if err != nil {
			logging.V(5).Infof("error deleting object: %v (%v) skipping", file.Key, err)
		}
	}

	return nil
}

var beforeMutation = func(as func(interface{}) bool) error {
	loggingLevel := glog.Level(7)

	environmentVariables := util.GetEnvironmentVariables()

	var bucketEncryptionType *string
	if v, exists := environmentVariables[s3BucketEncryptionTypeEnvVarKey]; exists {
		logging.V(loggingLevel).Infof("Setting bucket encryption type to %s", v)
		bucketEncryptionType = &v
	}

	var bucketEncryptionKMSKeyID *string
	if v, exists := environmentVariables[s3BucketKmsKeyIDEnvVarKey]; exists {
		logging.V(loggingLevel).Infof("Setting bucket encryption id to %s", v)
		bucketEncryptionKMSKeyID = &v
	}

	// No encryption settings specified, don't bother proceeding since we won't need
	// to set a value anyway
	if bucketEncryptionType == nil && bucketEncryptionKMSKeyID == nil {
		logging.V(loggingLevel).Info("No 'PULUMI_S3_BUCKET' environment variables are set")
		return nil
	}

	// Support AWS SDK V1
	var copyObjectInputV1 *s3V1.CopyObjectInput
	if as(&copyObjectInputV1) {
		logging.V(loggingLevel).Infof("Request was (v1) s3.CopyObjectInput")
		copyObjectInputV1.ServerSideEncryption = bucketEncryptionType
		copyObjectInputV1.SSEKMSKeyId = bucketEncryptionKMSKeyID

		return nil
	}

	// Support AWS SDK V2
	var copyObjectInputV2 *s3.CopyObjectInput
	if as(&copyObjectInputV2) {
		logging.V(loggingLevel).Infof("Request was s3.CopyObjectInput")
		if bucketEncryptionType != nil {
			copyObjectInputV2.ServerSideEncryption = types.ServerSideEncryption(*bucketEncryptionType)
		}
		copyObjectInputV2.SSEKMSKeyId = bucketEncryptionKMSKeyID

		return nil
	}

	// Support AWS SDK V1
	var uploadInput *s3manager.UploadInput
	if as(&uploadInput) {
		logging.V(loggingLevel).Infof("Request was s3manager.UploadInput")
		uploadInput.ServerSideEncryption = bucketEncryptionType
		uploadInput.SSEKMSKeyId = bucketEncryptionKMSKeyID

		return nil
	}

	// Support AWS SDK V2
	var putObjectInput *s3.PutObjectInput
	if as(&putObjectInput) {
		logging.V(loggingLevel).Infof("Request was s3.PutObjectInput")
		if bucketEncryptionType != nil {
			putObjectInput.ServerSideEncryption = types.ServerSideEncryption(*bucketEncryptionType)
		}
		putObjectInput.SSEKMSKeyId = bucketEncryptionKMSKeyID

		return nil
	}

	return nil
}
