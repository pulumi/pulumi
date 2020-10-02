package restblob

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/pkg/errors"
	"gocloud.dev/blob"
	"gocloud.dev/blob/driver"
	"gocloud.dev/gcerrors"
)

func init() {
	blob.DefaultURLMux().RegisterBucket("resthttp", &RestBlob{})
	blob.DefaultURLMux().RegisterBucket("resthttps", &RestBlob{})
}

type RestBlob struct{}

func (r *RestBlob) OpenBucketURL(ctx context.Context, u *url.URL) (*blob.Bucket, error) {
	u.Scheme = strings.TrimPrefix(u.Scheme, "rest")
	b := &Bucket{
		address: u,
		client:  http.DefaultClient,
	}
	b.index = &BucketIndex{
		address: b.address.String(),
		client:  b.client,
	}
	return blob.NewBucket(b), nil
}

type Bucket struct {
	address *url.URL
	client  *http.Client
	index   *BucketIndex
	// TODO: add support for the optional
	// locking feature in terraform http state backends
	// lockAddress   string
	// lockMethod    string
	// unlockAddress string
	// unlockMethod  string
}

type RestClient struct {
	address string
	client  *http.Client
}

func (c *RestClient) NewRestClient(address string) *RestClient {
	return &RestClient{
		address: address,
		client:  http.DefaultClient,
	}
}

// bloburl takes a blob key and returns it's blob url
// on the rest backend.
func (b *Bucket) bloburl(key string) string {
	// i've tried a few different key encoding.
	// i initially wanted something that is still
	// easily human readable because the key names
	// may be presented to users in some sort of web ui
	// that their rest http state backend may provide
	// but that proved troublesom as some http backends
	// that i tested, namely gitlab, throw 404 errors when
	// paths include "." characters.
	// i've chosen base64 encoding so that the code functions
	// but it'd be best to find a better compatible key encoding.
	key = strings.TrimPrefix(key, strings.TrimPrefix(b.address.Path, "/"))
	key = strings.TrimPrefix(key, "/")
	key = base64.StdEncoding.EncodeToString([]byte(key))

	// unfortuantly we can't just escape the key to put
	// in in the rest http URL parameter (path segment)
	// because some backends like gitlab throw 404
	// errors if the path component contains a "." dot character.
	// key = url.PathEscape(key)

	return fmt.Sprintf("%s/%s", b.address, key)
}

type Meta struct {
	// if attributes become a requirement
	// then we'll store them here
}

type Index struct {
	// the list of keys for Files pulumi needs to know about
	Files map[string]Meta `json:"files"`
}

type BucketIndex struct {
	address string
	client  *http.Client
	index   *Index
}

func (idx *BucketIndex) load(ctx context.Context) error {
	if idx.index != nil {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("%s/index.json", idx.address), nil)
	if err != nil {
		return err
	}

	res, err := idx.client.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to get state backend index")
	}

	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return errors.Wrap(err, "failed to read state backend index")
	}

	idx.index = &Index{}
	if err := json.Unmarshal(data, idx); err != nil {
		return errors.Wrap(err, "corrupt state file index (bad json)")
	}

	if idx.index.Files == nil {
		// the index might not exist yet
		// i.e. pulumi hasn't actually written anything
		// so we'll just initilize an empty list of files
		// so that the rest of our code doesn't need to nil check
		idx.index.Files = map[string]Meta{}
	}

	return nil
}

func (idx *BucketIndex) List(ctx context.Context) (map[string]Meta, error) {
	if err := idx.load(ctx); err != nil {
		return nil, err
	}
	return idx.index.Files, nil
}

func (idx *BucketIndex) Add(ctx context.Context, key string) error {
	if err := idx.load(ctx); err != nil {
		return err
	}

	idx.index.Files[key] = Meta{}

	if err := idx.save(ctx); err != nil {
		return err
	}

	return nil
}

func (idx *BucketIndex) Remove(ctx context.Context, key string) error {
	if err := idx.load(ctx); err != nil {
		return err
	}

	delete(idx.index.Files, key)

	if err := idx.save(ctx); err != nil {
		return err
	}

	return nil
}

func (idx *BucketIndex) save(ctx context.Context) error {
	data, err := json.MarshalIndent(idx.index, "", "  ")
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", fmt.Sprintf("%s/index.json", idx.address), bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := idx.client.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode >= 400 {
		return NewRestError(res)
	}

	return nil
}

func (b *Bucket) NewRangeReader(ctx context.Context, key string, offset, length int64, opts *driver.ReaderOptions) (driver.Reader, error) {
	return NewBlobReader(ctx, b, key)
}

func (b *Bucket) ListPaged(ctx context.Context, opts *driver.ListOptions) (*driver.ListPage, error) {
	files, err := b.index.List(ctx)
	if err != nil {
		return nil, err
	}

	page := &driver.ListPage{
		Objects: []*driver.ListObject{},
	}

	for key, _ := range files {
		if opts != nil && opts.Prefix != "" {
			if !strings.HasPrefix(key, opts.Prefix) {
				continue
			}
		}
		page.Objects = append(page.Objects, &driver.ListObject{
			Key: key,
		})
	}

	return page, nil
}

func (b *Bucket) NewTypedWriter(ctx context.Context, key, contentType string, opts *driver.WriterOptions) (driver.Writer, error) {
	return NewBlobWriter(ctx, b, key), nil
}

func (b *Bucket) Copy(ctx context.Context, dstKey, srcKey string, opts *driver.CopyOptions) error {
	r, err := b.NewRangeReader(ctx, srcKey, 0, -1, nil)
	if err != nil {
		return errors.Wrap(err, "failed to read source blob for copy")
	}

	w, err := b.NewTypedWriter(ctx, dstKey, "application/json", nil)
	if err != nil {
		return errors.Wrap(err, "failed to create destination blob writer for copy")
	}

	if _, err := io.Copy(w, r); err != nil {
		return errors.Wrap(err, "failed to copy blob")
	}

	return nil
}

func (b *Bucket) Delete(ctx context.Context, key string) error {
	req, err := http.NewRequestWithContext(ctx, "DELETE", b.bloburl(key), nil)
	if err != nil {
		return err
	}

	res, err := b.client.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode >= 400 {
		return NewRestError(res)
	}

	if err := b.index.Remove(ctx, key); err != nil {
		return err
	}

	return nil
}

func (b *Bucket) SignedURL(ctx context.Context, key string, opts *driver.SignedURLOptions) (string, error) {
	return "", errors.New("signed urls aren't supported with this state backend")
}

func (b *Bucket) ErrorCode(err error) gcerrors.ErrorCode {
	if resterr, ok := err.(*RestError); ok {
		switch resterr.StatusCode {
		case 200:
			return gcerrors.OK
		case 400:
			return gcerrors.InvalidArgument
		case 401:
			return gcerrors.PermissionDenied
		case 403:
			return gcerrors.PermissionDenied
		case 404:
			return gcerrors.NotFound
		case 500:
			return gcerrors.Internal
		}
	}
	return gcerrors.Unknown
}

func (b *Bucket) As(i interface{}) bool {
	return false
}

func (b *Bucket) ErrorAs(err error, i interface{}) bool {
	if resterr, ok := err.(*RestError); ok {
		if p, ok := i.(**RestError); ok {
			*p = resterr
			return true
		}
	}
	return false
}

func (b *Bucket) Attributes(ctx context.Context, key string) (*driver.Attributes, error) {
	// pulumi doesn't actually store metadata on blobs
	// so we don't need this implementation.
	// if this changes in the future then the design of
	// the index struct is ready to support metadata for blobs
	return nil, errors.New("Attributes not implemented")
}

func (b *Bucket) Close() error {
	b.client.CloseIdleConnections()
	return nil
}

type BlobReader struct {
	io.ReadCloser
}

func NewBlobReader(ctx context.Context, b *Bucket, key string) (*BlobReader, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", b.bloburl(key), nil)
	if err != nil {
		return nil, err
	}

	res, err := b.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err, "failed to request file from state backend")
	}

	if res.StatusCode >= 400 {
		return nil, NewRestError(res)
	}

	return &BlobReader{
		ReadCloser: res.Body,
	}, nil
}

func (r *BlobReader) Attributes() *driver.ReaderAttributes {
	// pulumi doesn't actually use attributes
	return nil
}

func (r *BlobReader) As(i interface{}) bool {
	return false
}

type BlobWriter struct {
	ctx    context.Context
	writer *bytes.Buffer
	key    string
	bucket *Bucket
}

func NewBlobWriter(ctx context.Context, b *Bucket, key string) *BlobWriter {
	return &BlobWriter{
		ctx:    ctx,
		writer: &bytes.Buffer{},
		bucket: b,
		key:    key,
	}
}

func (w *BlobWriter) Write(p []byte) (n int, err error) {
	return w.writer.Write(p)
}

func (w *BlobWriter) Close() error {
	req, err := http.NewRequestWithContext(w.ctx, "POST", w.bucket.bloburl(w.key), w.writer)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	u := req.URL.String()
	fmt.Println(u)

	res, err := w.bucket.client.Do(req)
	if err != nil {
		return err
	}
	if res.StatusCode >= 400 {
		return NewRestError(res)
	}

	// after the blob has been written via HTTP
	// we'll ensure that the key is in the bucket index
	if err := w.bucket.index.Add(w.ctx, w.key); err != nil {
		return err
	}

	return nil
}

type RestError struct {
	StatusCode int
	Err        error
}

func NewRestError(res *http.Response) *RestError {
	m, _ := ioutil.ReadAll(res.Body)
	return &RestError{
		StatusCode: res.StatusCode,
		Err:        errors.New(string(m)),
	}
}

func (r *RestError) Error() string {
	return fmt.Sprintf("status %d: body %v", r.StatusCode, r.Err)
}
