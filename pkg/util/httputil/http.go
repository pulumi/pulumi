package httputil

import (
	"context"
	"net/http"
	"time"

	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/retry"
)

// maxRetryCount is the number of times to try an http request before giving up an returning the last error
const maxRetryCount = 5

// DoWithRetry calls client.Do, and in the case of an error, retries the operation again after a slight delay.
func DoWithRetry(req *http.Request, client *http.Client) (*http.Response, error) {
	inRange := func(test, lower, upper int) bool {
		return lower <= test && test <= upper
	}

	_, res, err := retry.Until(context.Background(), retry.Acceptor{
		Accept: func(try int, nextRetryTime time.Duration) (bool, interface{}, error) {
			res, resErr := client.Do(req)
			if resErr == nil && !inRange(res.StatusCode, 500, 599) {
				return true, res, nil
			}
			if try >= (maxRetryCount - 1) {
				return false, res, resErr
			}

			// Close the response body, if present, since our caller can't.
			if resErr == nil {
				contract.IgnoreError(res.Body.Close())
			}
			return false, nil, nil
		},
	})

	if err != nil {
		return nil, err
	}

	return res.(*http.Response), nil
}

// GetWithRetry issues a GET request with the given client, and in the case of an error, retries the operation again
// after a slight delay.
func GetWithRetry(url string, client *http.Client) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	return DoWithRetry(req, client)
}
