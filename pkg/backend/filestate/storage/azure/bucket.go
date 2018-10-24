package azure

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/url"
	"runtime"
	"strings"
	"time"

	"github.com/Azure/azure-storage-blob-go/2016-05-31/azblob"
	azlock "github.com/lawrencegripper/goazurelocking"
	"github.com/pulumi/pulumi/pkg/backend/filestate/storage"
)

const (
	// URLPrefix is a unique schema used to identify this bucket provider
	URLPrefix = "azure://"

	accessTokenEnvVar = "AZURE_STORAGE_ACCOUNT_KEY"
)

var _ storage.Bucket = (*bucket)(nil) // enforces compile time check for interface compatibility

// bucket is a blob storage implementation using Azure Blob Storage
type bucket struct {
	url     *azblob.ContainerURL
	lock    azlock.Lock
	accKey  string
	accName string
}

func extractAccountNameFromURL(u *url.URL) (string, error) {
	var parts []string
	if parts = strings.Split(u.Hostname(), "."); len(parts) < 1 {
		return "", fmt.Errorf("unexpected format for url %s", u.String())
	}
	return parts[0], nil
}

// NewBucket creates a new bucket instance
func NewBucket(cloudURL, accountKey string) (storage.Bucket, error) {

	URL, err := url.Parse(strings.Replace(cloudURL, URLPrefix, "https://", 1))
	if err != nil {
		return nil, err
	}

	accountName, err := extractAccountNameFromURL(URL)
	if err != nil {
		return nil, err
	}

	contURL, err := newContainerURL(*URL, accountName, accountKey)
	if err != nil {
		return nil, err
	}

	_, err = contURL.Create(context.Background(), azblob.Metadata{}, azblob.PublicAccessNone)
	if err != nil {
		if serr, ok := err.(azblob.StorageError); ok {
			if serr.ServiceCode() != azblob.ServiceCodeContainerAlreadyExists {
				return nil, err
			}
		}
	}

	bkt := bucket{
		url:     contURL,
		accName: accountName,
		accKey:  accountKey,
	}

	return &bkt, nil
}

func newContainerURL(URL url.URL, accountName, accountKey string) (*azblob.ContainerURL, error) {
	creds := azblob.NewSharedKeyCredential(accountName, accountKey)
	pipe := azblob.NewPipeline(creds, azblob.PipelineOptions{
		Retry: azblob.RetryOptions{
			Policy:   azblob.RetryPolicyExponential,
			MaxTries: 3,
		},
	})

	cURL := azblob.NewContainerURL(URL, pipe)
	return &cURL, nil
}

// ListFiles will list all blobs under a given prefix. This method will handle paging of
// responses and create an aggregated array of blob paths. We don't anticipate loading
// enough blobs to cause any significant memory pressure in Pulumi.
func (b *bucket) ListFiles(ctx context.Context, prefix string) ([]string, error) {
	var blobs []string

	// Page through all the blobs in the container and build a list of blob names
	for marker := (azblob.Marker{}); marker.NotDone(); {
		listBlob, err := b.url.ListBlobs(ctx, marker, azblob.ListBlobsOptions{
			Prefix: prefix,
		})
		if err != nil {
			return nil, err
		}
		marker = listBlob.NextMarker
		for _, blobInfo := range listBlob.Blobs.Blob {
			blobs = append(blobs, blobInfo.Name)
		}
	}
	return blobs, nil
}

// WriteFile will create a new block blob
func (b *bucket) WriteFile(ctx context.Context, path string, bytes []byte) error {
	blobURL := b.url.NewBlockBlobURL(path)
	numCores := uint16(runtime.NumCPU())
	_, err := azblob.UploadBufferToBlockBlob(ctx, bytes, blobURL, azblob.UploadToBlockBlobOptions{
		Parallelism: numCores,
	})
	return err
}

// ReadFile will read the specified blob's contents
func (b *bucket) ReadFile(ctx context.Context, path string) ([]byte, error) {
	blobURL := b.url.NewBlockBlobURL(path)
	res, err := blobURL.GetBlob(ctx, azblob.BlobRange{}, azblob.BlobAccessConditions{}, false)
	if err != nil {
		if serr, ok := err.(azblob.StorageError); ok {
			// See error.go for justification
			werr := &StorageErrorWithoutCause{
				msg:         serr.Error(),
				serviceCode: string(serr.ServiceCode()),
				res:         serr.Response(),
			}
			return nil, werr
		}
		return nil, err
	}
	defer res.Body().Close() // nolint: errcheck
	// Caution: This will load the entire blob into memory.
	blob, err := ioutil.ReadAll(res.Body())
	if err != nil {
		return nil, err
	}
	return blob, nil
}

// DeleteFile will delete the specified blob
func (b *bucket) DeleteFile(ctx context.Context, path string) error {
	blboURL := b.url.NewBlockBlobURL(path)
	_, err := blboURL.Delete(ctx, azblob.DeleteSnapshotsOptionNone, azblob.BlobAccessConditions{})
	return err
}

// RenameFile will rename a specified blob with a new name. This method is not handle
// atomically and failure may result in duplicate blobs.
func (b *bucket) RenameFile(ctx context.Context, path, newPath string) error {
	original, err := b.ReadFile(ctx, path)
	if err != nil {
		return err
	}
	err = b.WriteFile(ctx, newPath, original)
	if err != nil {
		return err
	}
	return b.DeleteFile(ctx, path)
}

// DeleteFiles will delete all the blobs under a given prefix
func (b *bucket) DeleteFiles(ctx context.Context, prefix string) error {
	blobs, err := b.ListFiles(ctx, prefix)
	if err != nil {
		return err
	}
	for _, blob := range blobs {
		err = b.DeleteFile(ctx, blob)
		if err != nil {
			return err
		}
	}
	return nil
}

// IsNotExist will return true if a given error is a blob not found error
func (b *bucket) IsNotExist(err error) bool {
	if err != nil {
		if serr, ok := err.(azblob.StorageError); ok {
			if serr.ServiceCode() == azblob.ServiceCodeBlobNotFound {
				return true
			}
		}
	}
	return false
}

// Lock is a blocking call to try and take the lock for this
// stack. Once it has the lock it will return a unlocker
// function that can then be used by the client.
func (b *bucket) Lock(ctx context.Context, stackName string) (storage.UnlockFn, error) {
	lock, err := azlock.NewLockInstance(ctx, b.accName,
		b.accKey, stackName, time.Second*30)
	if err != nil {
		return nil, err
	}
	err = lock.Lock()
	if err != nil {
		return nil, err
	}

	b.lock = *lock
	return lock.Unlock, nil
}
