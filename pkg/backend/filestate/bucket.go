package filestate

import (
	"context"
	"io"
	"path"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"gocloud.dev/blob"
)

// listBucket returns a list of all files in the bucket within a given directory. go-cloud sorts the results by key
func listBucket(bucket *blob.Bucket, dir string) ([]*blob.ListObject, error) {
	bucketIter := bucket.List(&blob.ListOptions{
		Delimiter: "/",
		Prefix:    dir + "/",
	})

	files := []*blob.ListObject{}

	ctx := context.Background()
	for {
		file, err := bucketIter.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.Wrap(err, "could not list bucket")
		}
		files = append(files, file)
	}

	return files, nil
}

// objectName returns the filename of a ListObject (an object from a bucket)
func objectName(obj *blob.ListObject) string {
	_, filename := path.Split(obj.Key)
	return filename
}

// removeAllByPrefix deletes all objects with a given prefix (i.e. filepath)
func removeAllByPrefix(bucket *blob.Bucket, dir string) error {
	files, err := listBucket(bucket, dir)
	if err != nil {
		return errors.Wrap(err, "unable to list bucket objects for removal")
	}

	for _, file := range files {
		err = bucket.Delete(context.Background(), file.Key)
		if err != nil {
			logging.V(5).Infof("error deleting object: %v (%v) skipping", file.Key, err)
		}
	}

	return nil
}

// renameObject renames an object in a bucket. the rename requires a download/upload of the object due to a go-cloud API limitation
func renameObject(bucket *blob.Bucket, source string, dest string) error {
	byts, err := bucket.ReadAll(context.Background(), source)
	if err != nil {
		return errors.Wrap(err, "reading source object to be renamed")
	}

	err = bucket.WriteAll(context.Background(), dest, byts, nil)
	if err != nil {
		return errors.Wrapf(err, "writing destination object during rename of %s", source)
	}

	err = bucket.Delete(context.Background(), source)
	if err != nil {
		logging.V(5).Infof("error deleting source object after rename: %v (%v) skipping", source, err)
	}

	return nil
}
