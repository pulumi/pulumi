package executable

import (
	"testing"
)

func TestSplitGoPathShouldReturnExpected(t *testing.T) {
	tt := []struct {
		path     string
		os       string
		expected int
	}{
		{
			path:     "/home/user/go:/usr/local/go",
			os:       "linux",
			expected: 2,
		},
		{
			path:     "C:/Users/User/Documents/go;C:/Workspace/go",
			os:       "windows",
			expected: 2,
		},
		{
			path:     "/home/user/go",
			os:       "linux",
			expected: 1,
		},
	}

	for _, test := range tt {
		paths := splitGoPath(test.path, test.os)
		if len(paths) != test.expected {
			t.Errorf("expected path length to be %d, got %d", test.expected, len(paths))
		}
	}
}
