package azure

import (
	"context"
	"io/ioutil"
	"net/url"
	"runtime"
	"strings"

	"github.com/Azure/azure-storage-blob-go/2016-05-31/azblob"
	"github.com/pulumi/pulumi/pkg/backend/filestate/storage"
)

const (
	// URLPrefix is a unique schema used to identify this bucket provider
	URLPrefix = "azure://"

	accessTokenEnvVar = "AZURE_STORAGE_ACCOUNT_KEY"
)

var _ storage.Bucket = (*Bucket)(nil) // enforces compile time check for interface compatibility

// Bucket is a blob storage implementation using Azure Blob Storage
type Bucket struct {
	url *azblob.ContainerURL
}

// AccountName returns the buckets associated Azure Blob Storage account name
func (b *Bucket) AccountName() string {
	u := b.url.URL()
	return extractAccountNameFromURL(&u)
}

func extractAccountNameFromURL(u *url.URL) string {
	return strings.Split(u.Hostname(), ".")[0]
}

// NewBucket creates a new Bucket instance
func NewBucket(url, accountKey string) (storage.Bucket, error) {

	contURL, err := newContainerURL(url, accountKey)
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

	bucket := Bucket{
		url: contURL,
	}

	return &bucket, nil
}

func newContainerURL(cloudURL, accountKey string) (*azblob.ContainerURL, error) {
	URL, err := url.Parse(strings.Replace(cloudURL, URLPrefix, "https://", 1))
	if err != nil {
		return nil, err
	}

	accountName := extractAccountNameFromURL(URL)
	creds := azblob.NewSharedKeyCredential(accountName, accountKey)
	pipe := azblob.NewPipeline(creds, azblob.PipelineOptions{
		Retry: azblob.RetryOptions{
			Policy:   azblob.RetryPolicyExponential,
			MaxTries: 3,
		},
	})

	cURL := azblob.NewContainerURL(*URL, pipe)
	return &cURL, nil
}

// ListFiles will list all blobs under a given prefix. This method will handle paging of
// responses and create an aggregated array of blob paths. We don't anticipate loading
// enough blobs to cause any significant memory pressure in Pulumi.
func (b *Bucket) ListFiles(ctx context.Context, prefix string) ([]string, error) {
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
func (b *Bucket) WriteFile(ctx context.Context, path string, bytes []byte) error {
	blobURL := b.url.NewBlockBlobURL(path)
	numCores := uint16(runtime.NumCPU())
	_, err := azblob.UploadBufferToBlockBlob(ctx, bytes, blobURL, azblob.UploadToBlockBlobOptions{
		Parallelism: numCores,
	})
	return err
}

// ReadFile will read the specified blob's contents
func (b *Bucket) ReadFile(ctx context.Context, path string) ([]byte, error) {
	blobURL := b.url.NewBlockBlobURL(path)
	res, err := blobURL.GetBlob(ctx, azblob.BlobRange{}, azblob.BlobAccessConditions{}, false)
	if err != nil {
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
func (b *Bucket) DeleteFile(ctx context.Context, path string) error {
	blboURL := b.url.NewBlockBlobURL(path)
	_, err := blboURL.Delete(ctx, azblob.DeleteSnapshotsOptionNone, azblob.BlobAccessConditions{})
	return err
}

// RenameFile will rename a specified blob with a new name. This method is not handle
// atomically and failure may result in duplicate blobs.
func (b *Bucket) RenameFile(ctx context.Context, path, newPath string) error {
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
func (b *Bucket) DeleteFiles(ctx context.Context, prefix string) error {
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
func (b *Bucket) IsNotExist(err error) bool {
	if err != nil {
		if serr, ok := err.(azblob.StorageError); ok {
			if serr.ServiceCode() == azblob.ServiceCodeBlobNotFound {
				return true
			}
		}
	}
	return false
}
