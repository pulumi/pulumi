package util

import (
	"os"
	"strings"
)

var cache map[string]string

// GetEnvironmentVariables returns the environment variables for the system
// represented as a map of key value pairs to the environment variable and value
func GetEnvironmentVariables() map[string]string {
	if cache == nil {
		cache = make(map[string]string)
		for _, e := range os.Environ() {
			if i := strings.Index(e, "="); i >= 0 {
				cache[e[:i]] = e[i+1:]
			}
		}
	}

	return cache
}
