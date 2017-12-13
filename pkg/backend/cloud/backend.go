// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cloud

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/cheggaaa/pb"
	"github.com/golang/glog"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/cloud/apitype"
	"github.com/pulumi/pulumi/pkg/backend/state"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/pack"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/archive"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// Backend extends the base backend interface with specific information about cloud backends.
type Backend interface {
	backend.Backend
	CloudURL() string
}

type cloudBackend struct {
	cloudURL string
}

// New creates a new Pulumi backend for the given cloud API URL.
func New(cloudURL string) Backend {
	return &cloudBackend{cloudURL: cloudURL}
}

func (b *cloudBackend) Name() string     { return b.cloudURL }
func (b *cloudBackend) CloudURL() string { return b.cloudURL }

func (b *cloudBackend) GetStack(stackName tokens.QName) (backend.Stack, error) {
	// IDEA: query the stack directly instead of listing them.
	stacks, err := b.ListStacks()
	if err != nil {
		return nil, err
	}
	for _, stack := range stacks {
		if stack.Name() == stackName {
			return stack, nil
		}
	}
	return nil, nil
}

// CreateStackOptions is an optional bag of options specific to creating cloud stacks.
type CreateStackOptions struct {
	// CloudName is the optional PPC name to create the stack in.  If omitted, the organization's default PPC is used.
	CloudName string
}

func (b *cloudBackend) CreateStack(stackName tokens.QName, opts interface{}) error {
	projID, err := getCloudProjectIdentifier()
	if err != nil {
		return err
	}

	var cloudName string
	if opts != nil {
		if cloudOpts, ok := opts.(CreateStackOptions); ok {
			cloudName = cloudOpts.CloudName
		} else {
			return errors.New("expected a CloudStackOptions value for opts parameter")
		}
	}

	createStackReq := apitype.CreateStackRequest{
		CloudName: cloudName,
		StackName: string(stackName),
	}

	var createStackResp apitype.CreateStackResponse
	path := fmt.Sprintf("/orgs/%s/programs/%s/%s/stacks", projID.Owner, projID.Repository, projID.Project)
	if err := pulumiRESTCall(b.cloudURL, "POST", path, &createStackReq, &createStackResp); err != nil {
		return err
	}
	fmt.Printf("Created stack '%s' hosted in Pulumi Cloud PPC %s\n",
		stackName, createStackResp.CloudName)

	return nil
}

func (b *cloudBackend) ListStacks() ([]backend.Stack, error) {
	stacks, err := b.listCloudStacks()
	if err != nil {
		return nil, err
	}

	// Map to a summary slice.
	var results []backend.Stack
	for _, stack := range stacks {
		results = append(results, newStack(stack, b))
	}

	return results, nil
}

func (b *cloudBackend) RemoveStack(stackName tokens.QName, force bool) (bool, error) {
	projID, err := getCloudProjectIdentifier()
	if err != nil {
		return false, err
	}

	queryParam := ""
	if force {
		queryParam = "?force=true"
	}
	path := fmt.Sprintf("/orgs/%s/programs/%s/%s/stacks/%s%s",
		projID.Owner, projID.Repository, projID.Project, string(stackName), queryParam)

	// TODO[pulumi/pulumi-service#196] When the service returns a well known response for "this stack still has
	//     resources and `force` was not true", we should sniff for that message and return a true for the boolean.
	return false, pulumiRESTCall(b.cloudURL, "DELETE", path, nil, nil)
}

// updateKind is an enum for describing the kinds of updates we support.
type updateKind string

const (
	update  updateKind = "update"
	preview updateKind = "preview"
	destroy updateKind = "destroy"
)

func (b *cloudBackend) Preview(stackName tokens.QName, debug bool, _ engine.PreviewOptions) error {
	return b.updateStack(preview, stackName, debug)
}

func (b *cloudBackend) Update(stackName tokens.QName, debug bool, _ engine.DeployOptions) error {
	return b.updateStack(update, stackName, debug)
}

func (b *cloudBackend) Destroy(stackName tokens.QName, debug bool, _ engine.DestroyOptions) error {
	return b.updateStack(destroy, stackName, debug)
}

// updateStack performs a the provided type of update on a stack hosted in the Pulumi Cloud.
func (b *cloudBackend) updateStack(action updateKind, stackName tokens.QName, debug bool) error {
	// Print a banner so it's clear this is going to the cloud.
	var actionLabel string
	switch action {
	case update:
		actionLabel = "Updating"
	case preview:
		actionLabel = "Previewing"
	case destroy:
		actionLabel = "Destroying"
	default:
		contract.Failf("unsupported update kind: %v", action)
	}
	fmt.Printf(
		colors.ColorizeText(
			colors.BrightMagenta+"%s stack '%s' in the Pulumi Cloud"+colors.Reset+" ☁️\n"),
		actionLabel, stackName)

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
	restURLRoot := fmt.Sprintf(
		"/orgs/%s/programs/%s/%s/stacks/%s/%s",
		projID.Owner, projID.Repository, projID.Project, string(stackName), action)

	// Create the initial update object.
	var updateResponse apitype.UpdateProgramResponse
	if err = pulumiRESTCall(b.cloudURL, "POST", restURLRoot, &updateRequest, &updateResponse); err != nil {
		return err
	}

	// Upload the program's contents to the signed URL if appropriate.
	if action != destroy {
		err = uploadProgram(updateResponse.UploadURL, true /* print upload size to STDOUT */)
		if err != nil {
			return err
		}
	}

	// Start the update.
	restURLWithUpdateID := fmt.Sprintf("%s/%s", restURLRoot, updateResponse.UpdateID)
	var startUpdateResponse apitype.StartUpdateResponse
	if err = pulumiRESTCall(b.cloudURL, "POST", restURLWithUpdateID,
		nil /* no req body */, &startUpdateResponse); err != nil {
		return err
	}
	if action == update {
		glog.V(7).Infof("Stack %s being updated to version %d", stackName, startUpdateResponse.Version)
	}

	// Wait for the update to complete, which also polls and renders event output to STDOUT.
	status, err := b.waitForUpdate(restURLWithUpdateID)
	if err != nil {
		return errors.Wrapf(err, "waiting for %s", action)
	} else if status != apitype.StatusSucceeded {
		return errors.Errorf("%s unsuccessful: status %v", action, status)
	}
	return nil
}

// uploadProgram archives the current Pulumi program and uploads it to a signed URL. "current"
// meaning whatever Pulumi program is found in the CWD or parent directory.
// If set, printSize will print the size of the data being uploaded.
func uploadProgram(uploadURL string, progress bool) error {
	programPath, err := workspace.DetectPackage()
	if err != nil {
		return err
	}
	pkg, err := workspace.GetPackage()
	if err != nil {
		return err
	}

	parsedURL, err := url.Parse(uploadURL)
	if err != nil {
		return errors.Wrap(err, "parsing URL")
	}

	// programPath is the path to the Pulumi.yaml file. Need its parent folder.
	programFolder := filepath.Dir(programPath)
	archiveContents, err := archive.Process(programFolder, pkg.UseDefaultIgnores())
	if err != nil {
		return errors.Wrap(err, "creating archive")
	}
	var archiveReader io.Reader = archiveContents

	// If progress is requested, show a little animated ASCII progress bar.
	if progress {
		bar := pb.New(archiveContents.Len())
		archiveReader = bar.NewProxyReader(archiveReader)
		bar.Prefix(colors.ColorizeText(colors.SpecUnimportant + "Uploading program: "))
		bar.Postfix(colors.ColorizeText(colors.Reset))
		bar.SetMaxWidth(80)
		bar.SetUnits(pb.U_BYTES)
		bar.Start()
		defer func() {
			bar.Finish()
		}()
	}

	resp, err := http.DefaultClient.Do(&http.Request{
		Method:        "PUT",
		URL:           parsedURL,
		ContentLength: int64(archiveContents.Len()),
		Body:          ioutil.NopCloser(archiveReader),
	})
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return errors.Wrap(err, "upload failed")
	}
	return nil
}

func (b *cloudBackend) GetLogs(stackName tokens.QName,
	query operations.LogQuery) ([]operations.LogEntry, error) {
	// TODO[pulumi/pulumi-service#227]: Relax these conditions once the service can take these arguments.
	if query.StartTime != nil || query.EndTime != nil {
		return nil, errors.New("cloud backend does not (yet) support filtering logs by start time or end time")
	}

	projID, err := getCloudProjectIdentifier()
	if err != nil {
		return nil, err
	}

	var response apitype.LogsResult
	path := fmt.Sprintf("/orgs/%s/programs/%s/%s/stacks/%s/logs",
		projID.Owner, projID.Repository, projID.Project, string(stackName))
	if err = pulumiRESTCall(b.cloudURL, "GET", path, nil, &response); err != nil {
		return nil, err
	}

	logs := make([]operations.LogEntry, 0, len(response.Logs))
	for _, entry := range response.Logs {
		logs = append(logs, operations.LogEntry(entry))
	}

	return logs, nil
}

// listCloudStacks returns all stacks for the current repository x workspace on the Pulumi Cloud.
func (b *cloudBackend) listCloudStacks() ([]apitype.Stack, error) {
	projID, err := getCloudProjectIdentifier()
	if err != nil {
		return nil, err
	}

	// Query all stacks for the project on Pulumi.
	var stacks []apitype.Stack
	path := fmt.Sprintf("/orgs/%s/programs/%s/%s/stacks", projID.Owner, projID.Repository, projID.Project)
	if err := pulumiRESTCall(b.cloudURL, "GET", path, nil, &stacks); err != nil {
		return nil, err
	}
	return stacks, nil
}

// getDecryptedConfig returns the stack's configuration with any secrets in plain-text.
func getDecryptedConfig(stackName tokens.QName) (map[tokens.ModuleMember]string, error) {
	cfg, err := state.Configuration(stackName)
	if err != nil {
		return nil, errors.Wrap(err, "getting configuration")
	}

	decrypter, err := state.DefaultCrypter(cfg)
	if err != nil {
		return nil, errors.Wrap(err, "getting a default encrypter")
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

// getCloudProjectIdentifier returns information about the current repository and project, based on the current
// working directory.
func getCloudProjectIdentifier() (*cloudProjectIdentifier, error) {
	w, err := workspace.New()
	if err != nil {
		return nil, err
	}

	path, err := workspace.DetectPackage()
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
	programPath, err := workspace.DetectPackage()
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
	valueOrEmpty := func(s *string) string {
		if s != nil {
			return *s
		}
		return ""
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
		Main:        pkg.Main,
		Description: valueOrEmpty(pkg.Description),
		Config:      textConfig,
	}, nil
}

// waitForUpdate waits for the current update of a Pulumi program to reach a terminal state. Returns the
// final state. "path" is the URL endpoint to poll for updates.
func (b *cloudBackend) waitForUpdate(path string) (apitype.UpdateStatus, error) {
	// Events occur in sequence, filter out all the ones we have seen before in each request.
	eventIndex := "0"
	for {
		var updateResults apitype.UpdateResults
		pathWithIndex := fmt.Sprintf("%s?afterIndex=%s", path, eventIndex)
		if err := pulumiRESTCall(b.cloudURL, "GET", pathWithIndex, nil, &updateResults); err != nil {
			// If our request to the Pulumi Service returned a 504 (Gateway Timeout), ignore it and keep continuing.
			// TODO(pulumi/pulumi-ppc/issues/60): Elminate these timeouts all together.
			if errResp, ok := err.(*apitype.ErrorResponse); ok && errResp.Code == 504 {
				time.Sleep(1 * time.Second)
				continue
			}
			return apitype.StatusFailed, err
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
	if !ok || rawEntry == nil {
		return
	}
	text := rawEntry.(string)
	if colorize, ok := event.Fields["colorize"].(bool); ok && colorize {
		text = colors.ColorizeText(text)
	}
	fmt.Fprint(stream, text)
}

// Login logs into the target cloud URL.
func Login(cloudURL string) error {
	fmt.Printf("Logging into Pulumi Cloud: %s\n", cloudURL)

	// We intentionally don't accept command-line args for the user's access token. Having it in
	// .bash_history is not great, and specifying it via flag isn't of much use.
	accessToken := os.Getenv(AccessTokenEnvVar)
	if accessToken != "" {
		fmt.Printf("Using access token from %s\n", AccessTokenEnvVar)
	} else {
		token, readerr := cmdutil.ReadConsole("Enter your Pulumi access token")
		if readerr != nil {
			return readerr
		}
		accessToken = token
	}

	// Try and use the credentials to see if they are valid.
	valid, err := isValidAccessToken(cloudURL, accessToken)
	if err != nil {
		return err
	} else if !valid {
		return fmt.Errorf("invalid access token")
	}

	// Save them.
	return workspace.StoreAccessToken(cloudURL, accessToken)
}

// isValidAccessToken tries to use the provided Pulumi access token and returns if it is accepted
// or not. Returns error on any unexpected error.
func isValidAccessToken(cloud, accessToken string) (bool, error) {
	// Make a request to get the authenticated user. If it returns a successful result, the token
	// checks out.
	if err := pulumiRESTCallWithAccessToken(cloud, "GET", "/user", nil, nil, accessToken); err != nil {
		if errResp, ok := err.(*apitype.ErrorResponse); ok && errResp.Code == 401 {
			return false, nil
		}
		return false, fmt.Errorf("testing access token: %v", err)
	}
	return true, nil
}

// Logout logs out of the target cloud URL.
func Logout(cloudURL string) error {
	return workspace.DeleteAccessToken(cloudURL)
}

// CurrentBackends returns a list of the cloud backends the user is currently logged into.
func CurrentBackends() ([]Backend, error) {
	creds, err := workspace.GetStoredCredentials()
	if err != nil {
		return nil, err
	}

	var backends []Backend
	if creds.AccessTokens != nil {
		// Sort the URLs so that we return them in a deterministic order.
		var cloudURLs []string
		for url := range creds.AccessTokens {
			cloudURLs = append(cloudURLs, url)
		}
		sort.Strings(cloudURLs)

		for _, url := range cloudURLs {
			backends = append(backends, New(url))
		}
	}

	return backends, nil
}
