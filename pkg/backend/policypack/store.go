// Copyright 2016-2020, Pulumi Corporation.
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

package policypack

import (
	"context"
	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/sdk/v2/go/common/util/contract"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	schemeFile  = "file://"
	schemeHTTPS = "https://"
)

func publishLocal(url string, archive io.Reader) error {
	path := url
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return errors.Wrapf(err, "MkdirAll failed for path '%s'", path)
	}

	f, err := os.OpenFile(path, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return errors.Wrapf(err, "Open failed for path '%s'", path)
	}
	defer contract.IgnoreClose(f)

	_, uploadErr := io.Copy(f, archive)
	return uploadErr
}

func publishRemote(url string, archive io.Reader) error {
	putS3Req, err := http.NewRequest(http.MethodPut, url, archive)
	if err != nil {
		return errors.Wrapf(err, "Failed to upload compressed PolicyPack")
	}

	_, uploadErr := http.DefaultClient.Do(putS3Req)
	return uploadErr
}

// Publish publishes the provided archive to the URI based on the scheme.
// Currently supported schemes include https:// and file://.
func Publish(ctx context.Context, url string, archive io.Reader) error {
	var err error
	switch {
	case strings.HasPrefix(url, schemeHTTPS):
		err = publishRemote(url, archive)
	case strings.HasPrefix(url, schemeFile), strings.HasPrefix(url, "/"):
		err = publishLocal(url, archive)
	default:
		return errors.Errorf("unknown scheme found in URL %v", url)
	}
	return err
}

func downloadRemoteFile(url string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to download compressed PolicyPack")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to download compressed PolicyPack")
	}
	defer resp.Body.Close()

	tarball, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to download compressed PolicyPack")
	}

	return tarball, nil
}

func Download(ctx context.Context, url string) ([]byte, error) {
	var b []byte
	var err error
	switch {
	case strings.HasPrefix(url, schemeHTTPS):
		b, err = downloadRemoteFile(url)
	case strings.HasPrefix(url, schemeFile), strings.HasPrefix(url, "/"):
		b, err = ioutil.ReadFile(url)
	default:
		return nil, errors.Errorf("unknown scheme found in URL %v", url)
	}
	return b, err
}
