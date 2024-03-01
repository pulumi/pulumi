package workspace

import (
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

//nolint:paralleltest // mutates environment
func TestConcurrentCredentialsWrites(t *testing.T) {
	// save and remember to restore creds in ~/.pulumi/credentials
	// as the test will be modifying them
	oldCreds, err := GetStoredCredentials()
	assert.NoError(t, err)
	defer func() {
		err := StoreCredentials(oldCreds)
		assert.NoError(t, err)
	}()

	// use test creds that have at least 1 AccessToken to force a
	// disk write and contention
	testCreds := Credentials{
		AccessTokens: map[string]string{
			"token-name": "token-value",
		},
	}

	runtime.GOMAXPROCS(5000)

	// using 1000 may trigger sporadic 'Too many open files'
	n := 5000

	wg := &sync.WaitGroup{}
	wg.Add(2 * n)

	// Store testCreds initially so asserts in
	// GetStoredCredentials goroutines find the expected data
	err = StoreCredentials(testCreds)
	assert.NoError(t, err)

	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			err := StoreCredentials(testCreds)
			assert.NoError(t, err)
		}()
		go func() {
			defer wg.Done()
			creds, err := GetStoredCredentials()
			assert.NoError(t, err)
			assert.Equal(t, "token-value", creds.AccessTokens["token-name"])
		}()
	}
	wg.Wait()
	assert.False(t, true)
}
