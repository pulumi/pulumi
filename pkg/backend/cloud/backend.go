// Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

package cloud

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sort"
	"time"

	"github.com/cheggaaa/pb"
	"github.com/golang/glog"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/state"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/pack"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/archive"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/retry"
	"github.com/pulumi/pulumi/pkg/workspace"
)

// Backend extends the base backend interface with specific information about cloud backends.
type Backend interface {
	backend.Backend
	CloudURL() string
}

type cloudBackend struct {
	d        diag.Sink
	cloudURL string
}

// New creates a new Pulumi backend for the given cloud API URL.
func New(d diag.Sink, cloudURL string) Backend {
	return &cloudBackend{d: d, cloudURL: cloudURL}
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

	var createStackResp apitype.CreateStackResponseByName
	path := fmt.Sprintf("/orgs/%s/programs/%s/%s/stacks", projID.Owner, projID.Repository, projID.Project)
	if err := pulumiRESTCall(b.cloudURL, "POST", path, nil, &createStackReq, &createStackResp); err != nil {
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
	return false, pulumiRESTCall(b.cloudURL, "DELETE", path, nil, nil, nil)
}

// cloudCrypter is an encrypter/decrypter that uses the Pulumi cloud to encrypt/decrypt a stack's secrets.
type cloudCrypter struct {
	backend   *cloudBackend
	stackName string
}

func (c *cloudCrypter) EncryptValue(plaintext string) (string, error) {
	projID, err := getCloudProjectIdentifier()
	if err != nil {
		return "", err
	}

	path := fmt.Sprintf("/orgs/%s/programs/%s/%s/stacks/%s/encrypt",
		projID.Owner, projID.Repository, projID.Project, c.stackName)

	var resp apitype.EncryptValueResponse
	req := apitype.EncryptValueRequest{Plaintext: []byte(plaintext)}
	if err := pulumiRESTCall(c.backend.cloudURL, "POST", path, nil, &req, &resp); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(resp.Ciphertext), nil
}

func (c *cloudCrypter) DecryptValue(cipherstring string) (string, error) {
	projID, err := getCloudProjectIdentifier()
	if err != nil {
		return "", err
	}

	ciphertext, err := base64.StdEncoding.DecodeString(cipherstring)
	if err != nil {
		return "", err
	}

	path := fmt.Sprintf("/orgs/%s/programs/%s/%s/stacks/%s/decrypt",
		projID.Owner, projID.Repository, projID.Project, c.stackName)

	var resp apitype.DecryptValueResponse
	req := apitype.DecryptValueRequest{Ciphertext: ciphertext}
	if err := pulumiRESTCall(c.backend.cloudURL, "POST", path, nil, &req, &resp); err != nil {
		return "", err
	}
	return string(resp.Plaintext), nil
}

func (b *cloudBackend) GetStackCrypter(stackName tokens.QName) (config.Crypter, error) {
	return &cloudCrypter{backend: b, stackName: string(stackName)}, nil
}

// updateKind is an enum for describing the kinds of updates we support.
type updateKind string

const (
	update  updateKind = "update"
	preview updateKind = "preview"
	destroy updateKind = "destroy"
)

func (b *cloudBackend) Preview(stackName tokens.QName, pkg *pack.Package, root string,
	debug bool, _ engine.UpdateOptions) error {

	return b.updateStack(preview, stackName, pkg, root, debug)
}

func (b *cloudBackend) Update(stackName tokens.QName, pkg *pack.Package, root string,
	debug bool, _ engine.UpdateOptions) error {

	return b.updateStack(update, stackName, pkg, root, debug)
}

func (b *cloudBackend) Destroy(stackName tokens.QName, pkg *pack.Package, root string,
	debug bool, _ engine.UpdateOptions) error {

	return b.updateStack(destroy, stackName, pkg, root, debug)
}

// updateStack performs a the provided type of update on a stack hosted in the Pulumi Cloud.
func (b *cloudBackend) updateStack(action updateKind, stackName tokens.QName, pkg *pack.Package, root string,
	debug bool) error {

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
	updateRequest, err := b.makeProgramUpdateRequest(stackName, pkg)
	if err != nil {
		return err
	}

	// Generate the URL we'll use for all the REST calls.
	restURLRoot := fmt.Sprintf(
		"/orgs/%s/programs/%s/%s/stacks/%s/%s",
		projID.Owner, projID.Repository, projID.Project, string(stackName), action)

	// Create the initial update object.
	var updateResponse apitype.UpdateProgramResponse
	if err = pulumiRESTCall(b.cloudURL, "POST", restURLRoot, nil, &updateRequest, &updateResponse); err != nil {
		return err
	}

	// Upload the program's contents to the signed URL if appropriate.
	if action != destroy {
		err = uploadProgram(pkg, root, updateResponse.UploadURL, true /* print upload size to STDOUT */)
		if err != nil {
			return err
		}
	}

	// Start the update.
	restURLWithUpdateID := fmt.Sprintf("%s/%s", restURLRoot, updateResponse.UpdateID)
	var startUpdateResponse apitype.StartUpdateResponse
	if err = pulumiRESTCall(b.cloudURL, "POST", restURLWithUpdateID,
		nil, nil /* no req body */, &startUpdateResponse); err != nil {
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
func uploadProgram(pkg *pack.Package, programFolder, uploadURL string, progress bool) error {
	parsedURL, err := url.Parse(uploadURL)
	if err != nil {
		return errors.Wrap(err, "parsing URL")
	}

	// programPath is the path to the Pulumi.yaml file. Need its parent folder.
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
	logQuery operations.LogQuery) ([]operations.LogEntry, error) {

	projID, err := getCloudProjectIdentifier()
	if err != nil {
		return nil, err
	}

	var response apitype.LogsResult
	path := fmt.Sprintf("/orgs/%s/programs/%s/%s/stacks/%s/logs",
		projID.Owner, projID.Repository, projID.Project, string(stackName))
	if err = pulumiRESTCall(b.cloudURL, "GET", path, logQuery, nil, &response); err != nil {
		return nil, err
	}

	logs := make([]operations.LogEntry, 0, len(response.Logs))
	for _, entry := range response.Logs {
		logs = append(logs, operations.LogEntry(entry))
	}

	return logs, nil
}

func (b *cloudBackend) ExportDeployment(stackName tokens.QName) (json.RawMessage, error) {
	projID, err := getCloudProjectIdentifier()
	if err != nil {
		return nil, err
	}

	var response apitype.ExportStackResponse
	path := fmt.Sprintf("/orgs/%s/programs/%s/%s/stacks/%s/export",
		projID.Owner, projID.Repository, projID.Project, string(stackName))
	if err := pulumiRESTCall(b.cloudURL, "GET", path, nil, nil, &response); err != nil {
		return nil, err
	}

	return response.Deployment, nil
}

func (b *cloudBackend) ImportDeployment(stackName tokens.QName, deployment json.RawMessage) error {
	projID, err := getCloudProjectIdentifier()
	if err != nil {
		return err
	}

	stackPath := fmt.Sprintf("/orgs/%s/programs/%s/%s/stacks/%s",
		projID.Owner, projID.Repository, projID.Project, string(stackName))

	request := apitype.ImportStackRequest{Deployment: deployment}
	var response apitype.ImportStackResponse
	if err = pulumiRESTCall(b.cloudURL, "POST", stackPath+"/import", nil, &request, &response); err != nil {
		return err
	}

	// Wait for the import to complete, which also polls and renders event output to STDOUT.
	importPath := fmt.Sprintf("%s/update/%s", stackPath, response.UpdateID)
	status, err := b.waitForUpdate(importPath)
	if err != nil {
		return errors.Wrap(err, "waiting for import")
	} else if status != apitype.StatusSucceeded {
		return errors.Errorf("import unsuccessful: status %v", status)
	}
	return nil
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
	if err := pulumiRESTCall(b.cloudURL, "GET", path, nil, nil, &stacks); err != nil {
		return nil, err
	}
	return stacks, nil
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
func (b *cloudBackend) makeProgramUpdateRequest(stackName tokens.QName,
	pkg *pack.Package) (apitype.UpdateProgramRequest, error) {

	// Convert the configuration into its wire form.
	cfg, err := state.Configuration(b.d, stackName)
	if err != nil {
		return apitype.UpdateProgramRequest{}, errors.Wrap(err, "getting configuration")
	}
	wireConfig := make(map[string]apitype.ConfigValue)
	for k, cv := range cfg {
		v, err := cv.Value(config.NopDecrypter)
		contract.AssertNoError(err)

		wireConfig[string(k)] = apitype.ConfigValue{
			String: v,
			Secret: cv.Secure(),
		}
	}

	description := ""
	if pkg.Description != nil {
		description = *pkg.Description
	}
	return apitype.UpdateProgramRequest{
		Name:        string(pkg.Name),
		Runtime:     pkg.Runtime,
		Main:        pkg.Main,
		Description: description,
		Config:      wireConfig,
	}, nil
}

// waitForUpdate waits for the current update of a Pulumi program to reach a terminal state. Returns the
// final state. "path" is the URL endpoint to poll for updates.
func (b *cloudBackend) waitForUpdate(path string) (apitype.UpdateStatus, error) {
	// Events occur in sequence, filter out all the ones we have seen before in each request.
	eventIndex := "0"
	for {
		// Query for the latest update results, including log entries so we can provide active status updates.
		pathWithIndex := fmt.Sprintf("%s?afterIndex=%s", path, eventIndex)
		_, results, err := retry.Until(context.Background(), retry.Acceptor{
			Accept: func(try int, nextRetryTime time.Duration) (bool, interface{}, error) {
				return b.tryNextUpdate(pathWithIndex, try, nextRetryTime)
			},
		})
		if err != nil {
			return apitype.StatusFailed, err
		}

		// We got a result, print it out.
		updateResults := results.(apitype.UpdateResults)
		for _, event := range updateResults.Events {
			printEvent(event)
			eventIndex = event.Index
		}

		// Check if in termal state and if so return.
		switch updateResults.Status {
		case apitype.StatusFailed:
			fallthrough
		case apitype.StatusSucceeded:
			return updateResults.Status, nil
		}
	}
}

// tryNextUpdate tries to get the next update for a Pulumi program.  This may time or error out, which resutls in a
// false returned in the first return value.  If a non-nil error is returned, this operation should fail.
func (b *cloudBackend) tryNextUpdate(pathWithIndex string,
	try int, nextRetryTime time.Duration) (bool, interface{}, error) {
	// Perform the REST call.
	var results apitype.UpdateResults
	err := pulumiRESTCall(b.cloudURL, "GET", pathWithIndex, nil, nil, &results)

	// If there is no error, we're done.
	if err == nil {
		return true, results, nil
	}

	// There are three kinds of errors we might see:
	//     1) Expected HTTP errors (like timeouts); silently retry.
	//     2) Unexpected HTTP errors (like Unauthorized, etc); exit with an error.
	//     3) Anything else; this could be any number of things, including transient errors (flaky network).
	//        In this case, we warn the user and keep retrying; they can ^C if it's not transient.
	warn := true
	if errResp, ok := err.(*apitype.ErrorResponse); ok {
		if errResp.Code == 504 {
			// If our request to the Pulumi Service returned a 504 (Gateway Timeout), ignore it and keep
			// continuing.  The sole exception is if we've done this 10 times.  At that point, we will have
			// been waiting for many seconds, and want to let the user know something might be wrong.
			// TODO(pulumi/pulumi-ppc/issues/60): Elminate these timeouts all together.
			if try < 10 {
				warn = false
			}
			glog.V(3).Infof("Expected %s HTTP %d error after %d retries (retrying): %v",
				b.cloudURL, errResp.Code, try, err)
		} else {
			// Otherwise, we will issue an error.
			glog.V(3).Infof("Unexpected %s HTTP %d error after %d retries (erroring): %v",
				b.cloudURL, errResp.Code, try, err)
			return false, nil, err
		}
	} else {
		glog.V(3).Infof("Unexpected %s error after %d retries (retrying): %v", b.cloudURL, try, err)
	}

	// Issue a warning if appropriate.
	if warn {
		b.d.Warningf(diag.Message("error querying update status: %v"), err)
		b.d.Warningf(diag.Message("retrying in %vs... ^C to stop (this will not cancel the update)"),
			nextRetryTime.Seconds())
	}

	return false, nil, nil
}

func printEvent(event apitype.UpdateEvent) {
	// Pluck out the string.
	if raw, ok := event.Fields["text"]; ok && raw != nil {
		if text, ok := raw.(string); ok {
			// Colorize by default, but honor the engine's settings, if any.
			if colorize, ok := event.Fields["colorize"].(string); ok {
				text = colors.Colorization(colorize).Colorize(text)
			} else {
				text = colors.ColorizeText(text)
			}

			// Choose the stream to write to (by default stdout).
			var stream io.Writer
			if event.Kind == apitype.StderrEvent {
				stream = os.Stderr
			} else {
				stream = os.Stdout
			}

			// And write to it.
			fmt.Fprint(stream, text)
		}
	}
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
	return workspace.StoreAccessToken(cloudURL, accessToken, true)
}

// isValidAccessToken tries to use the provided Pulumi access token and returns if it is accepted
// or not. Returns error on any unexpected error.
func isValidAccessToken(cloud, accessToken string) (bool, error) {
	// Make a request to get the authenticated user. If it returns a successful result, the token
	// checks out.
	if err := pulumiRESTCallWithAccessToken(cloud, "GET", "/user", nil, nil, nil, accessToken); err != nil {
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
func CurrentBackends(d diag.Sink) ([]Backend, string, error) {
	urls, current, err := CurrentBackendURLs()
	if err != nil {
		return nil, "", err
	}

	var backends []Backend
	for _, url := range urls {
		backends = append(backends, New(d, url))
	}
	return backends, current, nil
}

// CurrentBackendURLs returns a list of the cloud backend URLS the user is currently logged into.
func CurrentBackendURLs() ([]string, string, error) {
	creds, err := workspace.GetStoredCredentials()
	if err != nil {
		return nil, "", err
	}

	var current string
	var cloudURLs []string
	if creds.AccessTokens != nil {
		current = creds.Current

		// Sort the URLs so that we return them in a deterministic order.
		for url := range creds.AccessTokens {
			cloudURLs = append(cloudURLs, url)
		}
		sort.Strings(cloudURLs)
	}

	return cloudURLs, current, nil
}
