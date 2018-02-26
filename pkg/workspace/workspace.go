// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package workspace

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/contract"
)

// W offers functionality for interacting with Pulumi workspaces.
type W interface {
	Settings() *Settings                        // returns a mutable pointer to the optional workspace settings info.
	Repository() *Repository                    // returns the repository this project belongs to.
	StackPath(stack tokens.QName) string        // returns the path to store stack information.
	BackupDirectory() (string, error)           // returns the directory to store backup stack files.
	HistoryDirectory(stack tokens.QName) string // returns the directory to store a stack's history information.
	Project() (*Project, error)                 // returns a copy of the project associated with this workspace.
	Save() error                                // saves any modifications to the workspace.
}

type projectWorkspace struct {
	name     tokens.PackageName // the package this workspace is associated with.
	project  string             // the path to the Pulumi.[yaml|json] file for this project.
	settings *Settings          // settings for this workspace.
	repo     *Repository        // the repo this workspace is associated with.
}

// New creates a new workspace using the current working directory.
func New() (W, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return NewFrom(cwd)
}

// NewFrom creates a new Pulumi workspace in the given directory. Requires a Pulumi.yaml file be present in the
// folder hierarchy between dir and the .pulumi folder.
func NewFrom(dir string) (W, error) {
	repo, err := GetRepository(dir)
	if err != nil {
		return nil, err
	}

	path, err := DetectProjectPathFrom(dir)
	if err != nil {
		return nil, err
	} else if path == "" {
		return nil, errors.New("no Pulumi.yaml project file found")
	}

	proj, err := LoadProject(path)
	if err != nil {
		return nil, err
	}

	w := projectWorkspace{
		name:    proj.Name,
		project: path,
		repo:    repo,
	}

	err = w.readSettings()
	if err != nil {
		return nil, err
	}

	if w.settings.ConfigDeprecated == nil {
		w.settings.ConfigDeprecated = make(map[tokens.QName]config.Map)
	}

	return &w, nil
}

func (pw *projectWorkspace) Settings() *Settings {
	return pw.settings
}

func (pw *projectWorkspace) Repository() *Repository {
	return pw.repo
}

func (pw *projectWorkspace) Project() (*Project, error) {
	return LoadProject(pw.project)
}

func (pw *projectWorkspace) Save() error {
	// let's remove all the empty entries from the config array
	for k, v := range pw.settings.ConfigDeprecated {
		if len(v) == 0 {
			delete(pw.settings.ConfigDeprecated, k)
		}
	}

	settingsFile := pw.settingsPath()

	// ensure the path exists
	err := os.MkdirAll(filepath.Dir(settingsFile), 0700)
	if err != nil {
		return err
	}

	b, err := json.MarshalIndent(pw.settings, "", "    ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(settingsFile, b, 0600)
}

func (pw *projectWorkspace) StackPath(stack tokens.QName) string {
	path := filepath.Join(pw.Repository().Root, StackDir, pw.name.String())
	if stack != "" {
		path = filepath.Join(path, qnamePath(stack)+".json")
	}
	return path
}

func (pw *projectWorkspace) BackupDirectory() (string, error) {
	user, err := user.Current()
	if user == nil || err != nil {
		return "", errors.New("failed to get current user")
	}

	projectDir := filepath.Dir(pw.project)
	projectBackupDirName := filepath.Base(projectDir) + "-" + sha1HexString(projectDir)

	return filepath.Join(user.HomeDir, BookkeepingDir, BackupDir, projectBackupDirName), nil
}

func (pw *projectWorkspace) HistoryDirectory(stack tokens.QName) string {
	path := filepath.Join(pw.Repository().Root, HistoryDir, pw.name.String())
	if stack != "" {
		return filepath.Join(path, qnamePath(stack))
	}
	return path
}

func (pw *projectWorkspace) readSettings() error {
	settingsPath := pw.settingsPath()

	b, err := ioutil.ReadFile(settingsPath)
	if err != nil && os.IsNotExist(err) {
		// not an error to not have an existing settings file.
		pw.settings = &Settings{}
		return nil
	} else if err != nil {
		return err
	}

	var settings Settings

	err = json.Unmarshal(b, &settings)
	if err != nil {
		return err
	}

	pw.settings = &settings
	return nil
}

func (pw *projectWorkspace) settingsPath() string {
	return filepath.Join(pw.Repository().Root, WorkspaceDir, pw.name.String(), WorkspaceFile)
}

// sha1HexString returns a hex string of the sha1 hash of value.
func sha1HexString(value string) string {
	h := sha1.New()
	_, err := h.Write([]byte(value))
	contract.AssertNoError(err)
	return hex.EncodeToString(h.Sum(nil))
}

// qnamePath just cleans a name and makes sure it's appropriate to use as a path.
func qnamePath(nm tokens.QName) string {
	return stringNamePath(string(nm))
}

// stringNamePart cleans a string component of a name and makes sure it's appropriate to use as a path.
func stringNamePath(nm string) string {
	return strings.Replace(nm, tokens.QNameDelimiter, string(os.PathSeparator), -1)
}
