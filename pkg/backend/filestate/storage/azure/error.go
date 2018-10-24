package azure

import (
	"net/http"

	"github.com/Azure/azure-storage-blob-go/2016-05-31/azblob"
)

// StorageErrorWithoutCause is needed due to a "bug" in
// https://github.com/Azure/azure-pipeline-go/blob/master/pipeline/error.go
// which means the StorageError implements the Causer interface even when
// the cause is nil. This result in an nil error being returned
// from errors.Cause(err) rather than the root cause error.
//
// This issue is being tracked here:
// https://github.com/Azure/azure-pipeline-go/issues/13
type StorageErrorWithoutCause struct {
	msg         string
	serviceCode string
	res         *http.Response
}

func (s *StorageErrorWithoutCause) Error() string {
	return s.msg
}

func (s *StorageErrorWithoutCause) ServiceCode() azblob.ServiceCodeType {
	return azblob.ServiceCodeType(s.serviceCode)
}

func (s *StorageErrorWithoutCause) Response() *http.Response {
	return s.res
}

func (s *StorageErrorWithoutCause) Timeout() bool {
	return false
}

func (s *StorageErrorWithoutCause) Temporary() bool {
	return false
}
