// Copyright 2025, Pulumi Corporation.
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

package pluginstorage

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/pulumi/pulumi/sdk/v3/go/common/util/archive"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"

	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
)

type PluginContent interface {
	io.Closer

	writeToDir(pathToDir string) error
}

func SingleFilePlugin(f *os.File, spec workspace.PluginSpec) PluginContent {
	return singleFilePlugin{F: f, Kind: spec.Kind, Name: spec.Name}
}

type singleFilePlugin struct {
	F    *os.File
	Kind apitype.PluginKind
	Name string
}

func (p singleFilePlugin) writeToDir(finalDir string) error {
	bytes, err := io.ReadAll(p.F)
	if err != nil {
		return err
	}

	finalPath := filepath.Join(finalDir, fmt.Sprintf("pulumi-%s-%s", p.Kind, p.Name))
	if runtime.GOOS == "windows" {
		finalPath += ".exe"
	}
	// We are writing an executable.
	return os.WriteFile(finalPath, bytes, 0o700) //nolint:gosec
}

func (p singleFilePlugin) Close() error {
	return p.F.Close()
}

func TarPlugin(tgz io.ReadCloser) PluginContent {
	return tarPlugin{Tgz: tgz}
}

type tarPlugin struct {
	Tgz io.ReadCloser
}

func (p tarPlugin) Close() error {
	return p.Tgz.Close()
}

func (p tarPlugin) writeToDir(finalPath string) error {
	return archive.ExtractTGZ(p.Tgz, finalPath)
}

func DirPlugin(rootPath string) PluginContent {
	return dirPlugin{Root: rootPath}
}

type dirPlugin struct {
	Root string
}

func (p dirPlugin) Close() error {
	return nil
}

func (p dirPlugin) writeToDir(dstRoot string) error {
	return filepath.WalkDir(p.Root, func(srcPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relPath := strings.TrimPrefix(srcPath, p.Root)
		dstPath := filepath.Join(dstRoot, relPath)

		if srcPath == p.Root {
			return nil
		}
		if d.IsDir() {
			return os.Mkdir(dstPath, 0o700)
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		src, err := os.Open(srcPath)
		if err != nil {
			return err
		}
		defer src.Close()

		dst, err := os.OpenFile(dstPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer dst.Close()

		_, err = io.Copy(dst, src)
		return err
	})
}
