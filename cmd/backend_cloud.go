// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/pkg/errors"
	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/pack"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/util/archive"
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

func (b *pulumiCloudPulumiBackend) Preview(stackName tokens.QName, debug bool, opts engine.PreviewOptions) error {
	projID, err := getCloudProjectIdentifier()
	if err != nil {
		return err
	}
	updateRequest, err := makeProgramUpdateRequest(stackName)
	if err != nil {
		return err
	}

	var updateResponse apitype.PreviewUpdateResponse
	path := fmt.Sprintf("/orgs/%s/programs/%s/%s/stacks/%s/preview",
		projID.Owner, projID.Repository, projID.Project, string(stackName))
	if err = pulumiRESTCall("POST", path, &updateRequest, &updateResponse); err != nil {
		return err
	}
	fmt.Printf("Previewing update to Stack '%s'...\n", string(stackName))

	// Wait for the update to complete.
	status, err := waitForUpdate(fmt.Sprintf("%s/%s", path, updateResponse.PreviewID))
	fmt.Println() // The PPC's final message we print to STDOUT doesn't include a newline.

	if err != nil {
		return errors.Errorf("waiting for preview: %v", err)
	}
	if status == apitype.StatusSucceeded {
		fmt.Println("Preview resulted in success.")
		return nil
	}
	return errors.Errorf("preview result was unsuccessful: status %v", status)
}

func (b *pulumiCloudPulumiBackend) Update(stackName tokens.QName, debug bool, opts engine.DeployOptions) error {
	projID, err := getCloudProjectIdentifier()
	if err != nil {
		return err
	}
	updateRequest, err := makeProgramUpdateRequest(stackName)
	if err != nil {
		return err
	}

	var updateResponse apitype.UpdateProgramResponse
	path := fmt.Sprintf("/orgs/%s/programs/%s/%s/stacks/%s/update", projID.Owner, projID.Repository, projID.Project, string(stackName))
	if err = pulumiRESTCall("POST", path, &updateRequest, &updateResponse); err != nil {
		return err
	}
	fmt.Printf("Updating Stack '%s' to version %d...\n", string(stackName), updateResponse.Version)

	// Wait for the update to complete.
	status, err := waitForUpdate(path)
	fmt.Println() // The PPC's final message we print to STDOUT doesn't include a newline.

	if err != nil {
		return errors.Errorf("waiting for update: %v", err)
	}
	if status == apitype.StatusSucceeded {
		fmt.Println("Update completed successfully.")
		return nil
	}
	return errors.Errorf("update unsuccessful: status %v", status)
}

func (b *pulumiCloudPulumiBackend) Destroy(stackName tokens.QName, debug bool, opts engine.DestroyOptions) error {
	// TODO[pulumi/pulumi#516]: Once pulumi.com supports previews of destroys, remove this code
	if opts.DryRun {
		return errors.New("Pulumi.com does not support previewing destroy operations yet")
	}

	projID, err := getCloudProjectIdentifier()
	if err != nil {
		return err
	}
	updateRequest, err := makeProgramUpdateRequest(stackName)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/orgs/%s/programs/%s/%s/stacks/%s/destroy", projID.Owner, projID.Repository, projID.Project, string(stackName))

	if err = pulumiRESTCall("POST", path, &updateRequest, nil /*destroy does not return data upon success*/); err != nil {
		return err
	}
	fmt.Printf("Destroying Stack '%s'...\n", string(stackName))

	// Wait for the update to complete.
	status, err := waitForUpdate(path)
	fmt.Println() // The PPC's final message we print to STDOUT doesn't include a newline.

	if err != nil {
		return errors.Errorf("waiting for destroy: %v", err)
	}
	if status == apitype.StatusSucceeded {
		fmt.Println("destroy complete.")
		return nil
	}
	return errors.Errorf("destroy unsuccessful: status %v", status)
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

// makeProgramUpdateRequest handles detecting the program, building a zip file of it, base64 encoding
// that and then returning an apitype.UpdateProgramRequest with all the relevant information to send
// to Pulumi.com
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
	// programPath is the path to the pulumi.yaml file. Need its parent folder.
	programFolder := filepath.Dir(programPath)
	archive, err := archive.EncodePath(programFolder)
	if err != nil {
		return apitype.UpdateProgramRequest{}, errors.Errorf("creating archive: %v", err)
	}

	// Load the package, since we now require passing the Runtime with the update request.
	pkg, err := pack.Load(programPath)
	if err != nil {
		return apitype.UpdateProgramRequest{}, err
	}

	// Gather up configuration.
	// TODO(pulumi-service/issues/221): Have pulumi.com handle the encryption/decryption.
	textConfig, err := getDecryptedConfig(stackName)
	if err != nil {
		return apitype.UpdateProgramRequest{}, errors.Wrap(err, "getting decrypted configuration")
	}

	return apitype.UpdateProgramRequest{
		Name:           pkg.Name,
		Runtime:        pkg.Runtime,
		ProgramArchive: archive,
		Config:         textConfig,
	}, nil
}

// waitForUpdate waits for the current update of a Pulumi program to reach a terminal state. Returns the
// final state. "path" is the URL endpoint to poll for updates.
func waitForUpdate(path string) (apitype.UpdateStatus, error) {
	time.Sleep(5 * time.Second)

	// Events occur in sequence, filter out all the ones we have seen before in each request.
	eventIndex := 0
	for {
		time.Sleep(2 * time.Second)

		var updateResults apitype.UpdateResults
		pathWithIndex := fmt.Sprintf("%s?afterIndex=%d", path, eventIndex)
		if err := pulumiRESTCall("GET", pathWithIndex, nil, &updateResults); err != nil {
			return "", err
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
