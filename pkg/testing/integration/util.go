// Copyright 2016-2024, Pulumi Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package integration

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
)

// DecodeMapString takes a string of the form key1=value1:key2=value2 and returns a go map.
func DecodeMapString(val string) (map[string]string, error) {
	newMap := make(map[string]string)

	if val != "" {
		for _, overrideClause := range strings.Split(val, ":") {
			data := strings.Split(overrideClause, "=")
			if len(data) != 2 {
				return nil, fmt.Errorf(
					"could not decode %s as an override, should be of the form <package>=<version>",
					overrideClause)
			}
			packageName := data[0]
			packageVersion := data[1]
			newMap[packageName] = packageVersion
		}
	}

	return newMap, nil
}

// ReplaceInFile does a find and replace for a given string within a file.
func ReplaceInFile(old, new, path string) error {
	rawContents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	newContents := strings.ReplaceAll(string(rawContents), old, new)
	return os.WriteFile(path, []byte(newContents), 0o600)
}

// getCmdBin returns the binary named bin in location loc or, if it hasn't yet been initialized, will lazily
// populate it by either using the default def or, if empty, looking on the current $PATH.
func getCmdBin(loc *string, bin, def string) (string, error) {
	if *loc == "" {
		*loc = def
		if *loc == "" {
			var err error
			*loc, err = exec.LookPath(bin)
			if err != nil {
				return "", fmt.Errorf("Expected to find `%s` binary on $PATH: %w", bin, err)
			}
		}
	}
	return *loc, nil
}

func uniqueSuffix() string {
	// .<timestamp>.<five random hex characters>
	timestamp := time.Now().Format("20060102-150405")
	suffix, err := resource.NewUniqueHex("."+timestamp+".", 5, -1)
	contract.AssertNoErrorf(err, "could not generate random suffix")
	return suffix
}

const (
	commandOutputFolderName = "command-output"
)

func writeCommandOutput(commandName, runDir string, output []byte) (string, error) {
	logFileDir := filepath.Join(runDir, commandOutputFolderName)
	if err := os.MkdirAll(logFileDir, 0o700); err != nil {
		return "", fmt.Errorf("Failed to create '%s': %w", logFileDir, err)
	}

	logFile := filepath.Join(logFileDir, commandName+uniqueSuffix()+".log")

	if err := os.WriteFile(logFile, output, 0o600); err != nil {
		return "", fmt.Errorf("Failed to write '%s': %w", logFile, err)
	}

	return logFile, nil
}

// CopyFile copies a single file from src to dst
// From https://blog.depado.eu/post/copy-files-and-directories-in-go
func CopyFile(src, dst string) error {
	var err error
	var srcfd *os.File
	var dstfd *os.File
	var srcinfo os.FileInfo
	var n int64

	if srcfd, err = os.Open(src); err != nil {
		return err
	}
	defer srcfd.Close()

	if dstfd, err = os.Create(dst); err != nil {
		return err
	}
	defer dstfd.Close()

	if n, err = io.Copy(dstfd, srcfd); err != nil {
		return err
	}
	if srcinfo, err = os.Stat(src); err != nil {
		return err
	}
	if n != srcinfo.Size() {
		return fmt.Errorf("failed to copy all bytes from %v to %v", src, dst)
	}
	return os.Chmod(dst, srcinfo.Mode())
}

// CopyDir copies a whole directory recursively
// From https://blog.depado.eu/post/copy-files-and-directories-in-go
func CopyDir(src, dst string) error {
	var err error
	var fds []os.DirEntry
	var srcinfo os.FileInfo

	if srcinfo, err = os.Stat(src); err != nil {
		return err
	}

	if err = os.MkdirAll(dst, srcinfo.Mode()); err != nil {
		return err
	}

	if fds, err = os.ReadDir(src); err != nil {
		return err
	}
	for _, fd := range fds {
		srcfp := path.Join(src, fd.Name())
		dstfp := path.Join(dst, fd.Name())

		if fd.IsDir() {
			if err = CopyDir(srcfp, dstfp); err != nil {
				fmt.Println(err)
			}
		} else {
			if err = CopyFile(srcfp, dstfp); err != nil {
				fmt.Println(err)
			}
		}
	}
	return nil
}

// AssertHTTPResultWithRetry attempts to assert that an HTTP endpoint exists
// and evaluate its response.
func AssertHTTPResultWithRetry(
	t *testing.T,
	output interface{},
	headers map[string]string,
	maxWait time.Duration,
	check func(string) bool,
) bool {
	hostname, ok := output.(string)
	if !assert.True(t, ok, fmt.Sprintf("expected `%s` output", output)) {
		return false
	}
	if !strings.HasPrefix(hostname, "http://") && !strings.HasPrefix(hostname, "https://") {
		hostname = "http://" + hostname
	}
	var err error
	var resp *http.Response
	startTime := time.Now()
	count, sleep := 0, 0
	for {
		now := time.Now()
		req, err := http.NewRequest("GET", hostname, nil)
		if !assert.NoError(t, err, "error reading request: %v", err) {
			return false
		}

		for k, v := range headers {
			// Host header cannot be set via req.Header.Set(), and must be set
			// directly.
			if strings.ToLower(k) == "host" {
				req.Host = v
				continue
			}
			req.Header.Set(k, v)
		}

		client := &http.Client{Timeout: time.Second * 10}
		resp, err = client.Do(req)

		if err == nil && resp.StatusCode == 200 {
			break
		}
		if now.Sub(startTime) >= maxWait {
			t.Logf("Timeout after %v. Unable to http.get %v successfully.", maxWait, hostname)
			break
		}
		count++
		// delay 10s, 20s, then 30s and stay at 30s
		if sleep > 30 {
			sleep = 30
		} else {
			sleep += 10
		}
		time.Sleep(time.Duration(sleep) * time.Second)
		t.Logf("Http Error: %v\n", err)
		t.Logf("  Retry: %v, elapsed wait: %v, max wait %v\n", count, now.Sub(startTime), maxWait)
	}
	if !assert.NoError(t, err) {
		return false
	}
	// Read the body
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if !assert.NoError(t, err) {
		return false
	}
	// Verify it matches expectations
	return check(string(body))
}

func CheckRuntimeOptions(t *testing.T, root string, expected map[string]interface{}) {
	t.Helper()

	var config struct {
		Runtime struct {
			Name    string                 `yaml:"name"`
			Options map[string]interface{} `yaml:"options"`
		} `yaml:"runtime"`
	}
	yamlFile, err := os.ReadFile(filepath.Join(root, "Pulumi.yaml"))
	if err != nil {
		t.Logf("could not read Pulumi.yaml in %s", root)
		t.FailNow()
	}
	if err := yaml.Unmarshal(yamlFile, &config); err != nil {
		t.Logf("could not parse Pulumi.yaml in %s", root)
		t.FailNow()
	}

	require.Equal(t, expected, config.Runtime.Options)
}

func createTemporaryGoFolder(prefix string) (string, error) {
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		usr, userErr := user.Current()
		if userErr != nil {
			return "", userErr
		}
		gopath = filepath.Join(usr.HomeDir, "go")
	}

	folder := fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano(), rand.Intn(1000000)) //nolint:gosec

	testRoot := filepath.Join(gopath, "src", folder)
	err := os.MkdirAll(testRoot, 0o700)
	if err != nil {
		return "", err
	}

	return testRoot, err
}
