// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/pack"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/util/archive"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"

	"github.com/pulumi/pulumi/pkg/tokens"
)

type pulumiCloudPulumiBackend struct{}

func (b *pulumiCloudPulumiBackend) CreateStack(stackName tokens.QName, opts StackCreationOptions) error {
	projID, err := getCloudProjectIdentifier()
	if err != nil {
		return err
	}

	createStackReq := apitype.CreateStackRequest{
		CloudName: opts.Cloud,
		StackName: stackName.String(),
	}

	var createStackResp apitype.CreateStackResponse
	path := fmt.Sprintf("/orgs/%s/programs/%s/%s/stacks", projID.Owner, projID.Repository, projID.Project)
	if err := pulumiRESTCall("POST", path, &createStackReq, &createStackResp); err != nil {
		return err
	}
	fmt.Printf("Created Stack '%s' hosted in Cloud '%s'\n", stackName, createStackResp.CloudName)

	return nil
}

func (b *pulumiCloudPulumiBackend) GetStacks() ([]stackSummary, error) {
	stacks, err := getCloudStacks()
	if err != nil {
		return nil, err
	}

	// Map to a summary slice.
	var summaries []stackSummary
	for _, stack := range stacks {
		summary := stackSummary{
			Name:          stack.StackName,
			LastDeploy:    "n/a", // TODO(pulumi-service/issues#249): Make this info available.
			ResourceCount: strconv.Itoa(len(stack.Resources)),
		}
		// If the stack hasn't been pushed to, it's resource count doesn't matter.
		if stack.ActiveUpdate == "" {
			summary.ResourceCount = "n/a"
		}
		summaries = append(summaries, summary)
	}

	return summaries, nil
}

func (b *pulumiCloudPulumiBackend) RemoveStack(stackName tokens.QName, force bool) error {
	projID, err := getCloudProjectIdentifier()
	if err != nil {
		return err
	}

	queryParam := ""
	if force {
		queryParam = "?force=true"
	}
	path := fmt.Sprintf("/orgs/%s/programs/%s/%s/stacks/%s%s",
		projID.Owner, projID.Repository, projID.Project, string(stackName), queryParam)

	// TODO[pulumi/pulumi-service#196] When the service returns a well known response for "this stack still has resources and `force`
	// was not true", we should sniff for that message and return errHasResources
	return pulumiRESTCall("DELETE", path, nil, nil)
}

// updateKind is an enum for describing the kinds of updates we support.
type updateKind = int

const (
	update updateKind = iota
	preview
	destroy
)

func (b *pulumiCloudPulumiBackend) Preview(stackName tokens.QName, debug bool, _ engine.PreviewOptions) error {
	return updateStack(preview, stackName, debug)
}

func (b *pulumiCloudPulumiBackend) Update(stackName tokens.QName, debug bool, _ engine.DeployOptions) error {
	return updateStack(update, stackName, debug)
}

func (b *pulumiCloudPulumiBackend) Destroy(stackName tokens.QName, debug bool, _ engine.DestroyOptions) error {
	return updateStack(destroy, stackName, debug)
}

// updateStack performs a the provided type of update on a stack hosted in the Pulumi Cloud.
func updateStack(kind updateKind, stackName tokens.QName, debug bool) error {
	// First create the update object.
	projID, err := getCloudProjectIdentifier()
	if err != nil {
		return err
	}
	updateRequest, err := makeProgramUpdateRequest(stackName)
	if err != nil {
		return err
	}

	// Generate the URL we'll use for all the REST calls.
	var action string
	switch kind {
	case update:
		action = "update"
	case preview:
		action = "preview"
	case destroy:
		action = "destroy"
	default:
		contract.Failf("unsupported update kind: %v", kind)
	}
	restURLRoot := fmt.Sprintf(
		"/orgs/%s/programs/%s/%s/stacks/%s/%s",
		projID.Owner, projID.Repository, projID.Project, string(stackName), action)

	// Create the initial update object.
	var updateResponse apitype.UpdateProgramResponse
	if err = pulumiRESTCall("POST", restURLRoot, &updateRequest, &updateResponse); err != nil {
		return err
	}

	// Upload the program's contents to the signed URL if appropriate.
	if kind != destroy {
		err = uploadProgram(updateResponse.UploadURL, debug /* print upload size to STDOUT */)
		if err != nil {
			return err
		}
	}

	// Start the update.
	restURLWithUpdateID := fmt.Sprintf("%s/%s", restURLRoot, updateResponse.UpdateID)
	var startUpdateResponse apitype.StartUpdateResponse
	if err = pulumiRESTCall("POST", restURLWithUpdateID, nil /* no req body */, &startUpdateResponse); err != nil {
		return err
	}
	if kind == update {
		fmt.Printf("Updating Stack '%s' to version %d...\n", string(stackName), startUpdateResponse.Version)
	}

	// Wait for the update to complete, which also polls and renders event output to STDOUT.
	status, err := waitForUpdate(restURLWithUpdateID)
	fmt.Println() // The PPC's final message we print to STDOUT doesn't include a newline.

	if err != nil {
		return errors.Wrapf(err, "waiting for %s", action)
	}
	if status != apitype.StatusSucceeded {
		return errors.Errorf("%s unsuccessful: status %v", action, status)
	}
	fmt.Printf("%s completed successfully.\n", action)
	return nil
}

// uploadProgram archives the current Pulumi program and uploads it to a signed URL. "current"
// meaning whatever Pulumi program is found in the CWD or parent directory.
// If set, printSize will print the size of the data being uploaded.
func uploadProgram(uploadURL string, printSize bool) error {
	cwd, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "getting working directory")
	}
	programPath, err := workspace.DetectPackage(cwd)
	if err != nil {
		return errors.Wrap(err, "looking for Pulumi package")
	}
	if programPath == "" {
		return errors.New("no Pulumi package found")
	}
	// programPath is the path to the Pulumi.yaml file. Need its parent folder.
	programFolder := filepath.Dir(programPath)
	archiveContents, err := archive.Process(programFolder)
	if err != nil {
		return errors.Wrap(err, "creating archive")
	}

	if printSize {
		mb := float32(archiveContents.Len()) / (1024.0 * 1024.0)
		fmt.Printf("Uploading %.2fMiB\n", mb)
	}

	parsedURL, err := url.Parse(uploadURL)
	if err != nil {
		return errors.Wrap(err, "parsing URL")
	}

	resp, err := http.DefaultClient.Do(&http.Request{
		Method:        "PUT",
		URL:           parsedURL,
		ContentLength: int64(archiveContents.Len()),
		Body:          ioutil.NopCloser(archiveContents),
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return errors.Wrap(err, "upload failed")
	}
	return nil
}

func (b *pulumiCloudPulumiBackend) GetLogs(stackName tokens.QName, query operations.LogQuery) ([]operations.LogEntry, error) {
	// TODO[pulumi/pulumi-service#227]: Relax these conditions once the service can take these arguments.
	if query.StartTime != nil || query.EndTime != nil {
		return nil, errors.New("cloud backend does not (yet) support filtering logs by start time or end time")
	}

	projID, err := getCloudProjectIdentifier()
	if err != nil {
		return nil, err
	}

	var response apitype.LogsResult
	path := fmt.Sprintf("/orgs/%s/programs/%s/%s/stacks/%s/logs", projID.Owner, projID.Repository, projID.Project, string(stackName))
	if err = pulumiRESTCall("GET", path, nil, &response); err != nil {
		return nil, err
	}

	logs := make([]operations.LogEntry, 0, len(response.Logs))
	for _, entry := range response.Logs {
		logs = append(logs, operations.LogEntry(entry))
	}

	return logs, nil
}

// getCloudStacks returns all stacks for the current repository x workspace on the Pulumi Cloud.
func getCloudStacks() ([]apitype.Stack, error) {
	projID, err := getCloudProjectIdentifier()
	if err != nil {
		return nil, err
	}

	// Query all stacks for the project on Pulumi.
	var stacks []apitype.Stack
	path := fmt.Sprintf("/orgs/%s/programs/%s/%s/stacks", projID.Owner, projID.Repository, projID.Project)
	if err := pulumiRESTCall("GET", path, nil, &stacks); err != nil {
		return nil, err
	}
	return stacks, nil
}

// getDecryptedConfig returns the stack's configuration with any secrets in plain-text.
func getDecryptedConfig(stackName tokens.QName) (map[tokens.ModuleMember]string, error) {
	cfg, err := getConfiguration(stackName)
	if err != nil {
		return nil, errors.Wrap(err, "getting configuration")
	}

	var decrypter config.ValueDecrypter = panicCrypter{}
	if hasSecureValue(cfg) {
		decrypter, err = getSymmetricCrypter()
		if err != nil {
			return nil, errors.Wrap(err, "getting symmetric crypter")
		}
	}

	textConfig := make(map[tokens.ModuleMember]string)
	for key := range cfg {
		decrypted, err := cfg[key].Value(decrypter)
		if err != nil {
			return nil, errors.Wrap(err, "could not decrypt configuration value")
		}
		textConfig[key] = decrypted
	}
	return textConfig, nil
}

// getCloudProjectIdentifier returns information about the current repository and project, based on the current working
// directory.
func getCloudProjectIdentifier() (*cloudProjectIdentifier, error) {
	w, err := newWorkspace()
	if err != nil {
		return nil, err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	path, err := workspace.DetectPackage(cwd)
	if err != nil {
		return nil, err
	}

	pkg, err := pack.Load(path)
	if err != nil {
		return nil, err
	}

	repo := w.Repository()
	return &cloudProjectIdentifier{
		Owner:      repo.Owner,
		Repository: repo.Name,
		Project:    pkg.Name,
	}, nil
}

// makeProgramUpdateRequest constructs the apitype.UpdateProgramRequest based on the local machine state.
func makeProgramUpdateRequest(stackName tokens.QName) (apitype.UpdateProgramRequest, error) {
	// Zip up the Pulumi program's directory, which may be a parent of CWD.
	cwd, err := os.Getwd()
	if err != nil {
		return apitype.UpdateProgramRequest{}, errors.Errorf("getting working directory: %v", err)
	}
	programPath, err := workspace.DetectPackage(cwd)
	if err != nil {
		return apitype.UpdateProgramRequest{}, errors.Errorf("looking for Pulumi package: %v", err)
	}
	if programPath == "" {
		return apitype.UpdateProgramRequest{}, errors.Errorf("no Pulumi package found")
	}

	// Load the package, since we now require passing the Runtime with the update request.
	pkg, err := pack.Load(programPath)
	if err != nil {
		return apitype.UpdateProgramRequest{}, err
	}
	description := ""
	if pkg.Description != nil {
		description = *pkg.Description
	}

	// Gather up configuration.
	// TODO(pulumi-service/issues/221): Have pulumi.com handle the encryption/decryption.
	textConfig, err := getDecryptedConfig(stackName)
	if err != nil {
		return apitype.UpdateProgramRequest{}, errors.Wrap(err, "getting decrypted configuration")
	}

	return apitype.UpdateProgramRequest{
		Name:        pkg.Name,
		Runtime:     pkg.Runtime,
		Description: description,
		Config:      textConfig,
	}, nil
}

// waitForUpdate waits for the current update of a Pulumi program to reach a terminal state. Returns the
// final state. "path" is the URL endpoint to poll for updates.
func waitForUpdate(path string) (apitype.UpdateStatus, error) {
	time.Sleep(5 * time.Second)

	// Events occur in sequence, filter out all the ones we have seen before in each request.
	eventIndex := "0"
	for {
		time.Sleep(2 * time.Second)

		var updateResults apitype.UpdateResults
		pathWithIndex := fmt.Sprintf("%s?afterIndex=%s", path, eventIndex)
		if err := pulumiRESTCall("GET", pathWithIndex, nil, &updateResults); err != nil {
			// If our request to the Pulumi Service returned a 504 (Gateway Timeout), ignore it
			// and keep continuing.
			// TODO(pulumi/pulumi-ppc/issues/60): Elminate these timeouts all together.
			if errResp, ok := err.(*apitype.ErrorResponse); ok && errResp.Code == 504 {
				time.Sleep(5 * time.Second)
				continue
			}
		}

		for _, event := range updateResults.Events {
			printEvent(event)
			eventIndex = event.Index
		}

		// Check if in termal state.
		updateStatus := apitype.UpdateStatus(updateResults.Status)
		switch updateStatus {
		case apitype.StatusFailed:
			fallthrough
		case apitype.StatusSucceeded:
			return updateStatus, nil
		}
	}
}

func printEvent(event apitype.UpdateEvent) {
	stream := os.Stdout // Ignoring event.Kind which could be StderrEvent.
	rawEntry, ok := event.Fields["text"]
	if !ok {
		return
	}
	text := rawEntry.(string)
	if colorize, ok := event.Fields["colorize"].(bool); ok && colorize {
		text = colors.ColorizeText(text)
	}
	fmt.Fprint(stream, text)
}
