package httputil

import (
	"context"
	"net/http"
	"time"

	"github.com/pulumi/pulumi/pkg/util/retry"
)

// maxRetryCount is the number of times to try an http request before giving up an returning the last error
const maxRetryCount = 5

// DoWithRetry is like http.DefaultClient.Do expect that if it returns an error, we retry the operation again after a
// slight delay. Note that in the case where the server returns an error response (e.g. 4xx or 5xx) we do not rety the
// request. This method only concerns itself with trying to get a response back from the server.
func DoWithRetry(req *http.Request) (*http.Response, error) {
	_, res, err := retry.Until(context.Background(), retry.Acceptor{
		Accept: func(try int, nextRetryTime time.Duration) (bool, interface{}, error) {
			res, resErr := http.DefaultClient.Do(req)
			if resErr == nil {
				return true, res, nil
			}
			if try >= (maxRetryCount - 1) {
				return false, res, resErr
			}
			return false, nil, nil
		},
	})

	return res.(*http.Response), err
}

// GetWithRetry is like http.DefaultClient.Get expect that if it returns an error, we retry the operation again after a
// slight delay. Note that in the case where the server returns an error response (e.g. 4xx or 5xx) we do not rety the
// request. This method only concerns itself with trying to get a response back from the server.
func GetWithRetry(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	return DoWithRetry(req)
}
