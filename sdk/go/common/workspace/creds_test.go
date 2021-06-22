package workspace

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConcurrentCredentialsWrites(t *testing.T) {
	creds, err := GetStoredCredentials()
	assert.NoError(t, err)

	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			err := StoreCredentials(creds)
			assert.NoError(t, err)
		}()
		go func() {
			defer wg.Done()
			_, err := GetStoredCredentials()
			assert.NoError(t, err)
		}()
	}
	wg.Wait()
}
