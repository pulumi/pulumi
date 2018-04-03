// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cloud

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cheggaaa/pb"
	"github.com/golang/glog"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/cloud/client"
	"github.com/pulumi/pulumi/pkg/backend/local"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/archive"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/retry"
	"github.com/pulumi/pulumi/pkg/workspace"
)

const (
	// defaultURL is the Cloud URL used if no environment or explicit cloud is chosen.
	defaultURL = "https://" + defaultAPIURLPrefix + "pulumi.com"
	// defaultAPIURLPrefix is the assumed Cloud URL prefix for typical Pulumi Cloud API endpoints.
	defaultAPIURLPrefix = "api."
	// defaultAPIEnvVar can be set to override the default cloud chosen, if `--cloud` is not present.
	defaultURLEnvVar = "PULUMI_API"
	// AccessTokenEnvVar is the environment variable used to bypass a prompt on login.
	AccessTokenEnvVar = "PULUMI_ACCESS_TOKEN"
)

// DefaultURL returns the default cloud URL.  This may be overridden using the PULUMI_API environment
// variable.  If no override is found, and we are authenticated with only one cloud, choose that.  Otherwise,
// we will default to the https://api.pulumi.com/ endpoint.
func DefaultURL() string {
	return ValueOrDefaultURL("")
}

// ValueOrDefaultURL returns the value if specified, or the default cloud URL otherwise.
func ValueOrDefaultURL(cloudURL string) string {
	// If we have a cloud URL, just return it.
	if cloudURL != "" {
		return cloudURL
	}

	// Otherwise, respect the PULUMI_API override.
	if cloudURL := os.Getenv(defaultURLEnvVar); cloudURL != "" {
		return cloudURL
	}

	// If that didn't work, see if we're authenticated with any clouds.
	urls, current, err := CurrentBackendURLs()
	if err == nil {
		if current != "" {
			// If there's a current cloud selected, return that.
			return current
		} else if len(urls) == 1 {
			// Else, if we're authenticated with a single cloud, use that.
			return urls[0]
		}
	}

	// If none of those led to a cloud URL, simply return the default.
	return defaultURL
}

// barCloser is an implementation of io.Closer that finishes a progress bar upon Close() as well as closing its
// underlying readCloser.
type barCloser struct {
	bar        *pb.ProgressBar
	readCloser io.ReadCloser
}

func (bc *barCloser) Read(dest []byte) (int, error) {
	return bc.readCloser.Read(dest)
}

func (bc *barCloser) Close() error {
	bc.bar.Finish()
	return bc.readCloser.Close()
}

func newBarProxyReadCloser(bar *pb.ProgressBar, r io.Reader) io.ReadCloser {
	return &barCloser{
		bar:        bar,
		readCloser: bar.NewProxyReader(r),
	}
}

// Backend extends the base backend interface with specific information about cloud backends.
type Backend interface {
	backend.Backend
	CloudURL() string
	DownloadPlugin(info workspace.PluginInfo, progress bool) (io.ReadCloser, error)
	ListTemplates() ([]workspace.Template, error)
	DownloadTemplate(name string, progress bool) (io.ReadCloser, error)
}

type cloudBackend struct {
	d      diag.Sink
	name   string
	client *client.Client
}

// New creates a new Pulumi backend for the given cloud API URL and token.
func New(d diag.Sink, apiURL string) (Backend, error) {
	apiToken, err := workspace.GetAccessToken(apiURL)
	if err != nil {
		return nil, errors.Wrap(err, "getting stored credentials")
	}

	return &cloudBackend{
		d:      d,
		name:   apiURL,
		client: client.NewClient(apiURL, apiToken),
	}, nil
}

func (b *cloudBackend) Name() string     { return b.name }
func (b *cloudBackend) CloudURL() string { return b.name }

// CloudConsoleURL returns a link to the cloud console with the given path elements.  If a console link cannot be
// created, we return the empty string instead (this can happen if the endpoint isn't a recognized pattern).
func (b *cloudBackend) CloudConsoleURL(paths ...string) string {
	// To produce a cloud console URL, we assume that the URL is of the form `api.xx.yy`, and simply strip off the
	// `api.` part.  If that is not the case, we will return an empty string because we don't recognize the pattern.
	url := b.CloudURL()
	ix := strings.Index(url, defaultAPIURLPrefix)
	if ix == -1 {
		return ""
	}
	return url[:ix] + path.Join(append([]string{url[ix+len(defaultAPIURLPrefix):]}, paths...)...)
}

// CloudConsoleProjectPath returns the project path components for getting to a stack in the cloud console.  This path
// must, of course, be combined with the actual console base URL by way of the CloudConsoleURL function above.
func (b *cloudBackend) CloudConsoleProjectPath(projID client.ProjectIdentifier) string {
	return path.Join(projID.Owner, projID.Repository, projID.Project)
}

// CloudConsoleStackPath returns the stack path components for getting to a stack in the cloud console.  This path
// must, of coursee, be combined with the actual console base URL by way of the CloudConsoleURL function above.
func (b *cloudBackend) CloudConsoleStackPath(stackID client.StackIdentifier) string {
	return path.Join(b.CloudConsoleProjectPath(stackID.ProjectIdentifier), stackID.Stack)
}

// DownloadPlugin downloads a plugin as a tarball from the release endpoint.  The returned reader is a stream
// that reads the tar.gz file, which should be expanded and closed after the download completes.  If progress
// is true, the download will display a progress bar using stdout.
func (b *cloudBackend) DownloadPlugin(info workspace.PluginInfo, progress bool) (io.ReadCloser, error) {
	// Figure out the OS/ARCH pair for the download URL.
	var os string
	switch runtime.GOOS {
	case "darwin", "linux", "windows":
		os = runtime.GOOS
	default:
		return nil, errors.Errorf("unsupported plugin OS: %s", runtime.GOOS)
	}
	var arch string
	switch runtime.GOARCH {
	case "amd64":
		arch = runtime.GOARCH
	default:
		return nil, errors.Errorf("unsupported plugin architecture: %s", runtime.GOARCH)
	}

	// Now make the client request.
	result, size, err := b.client.DownloadPlugin(info, os, arch)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to download plugin")
	}

	// If progress is requested, and we know the length, show a little animated ASCII progress bar.
	if progress && size != -1 {
		bar := pb.New(int(size))
		result = newBarProxyReadCloser(bar, result)
		bar.Prefix(colors.ColorizeText(colors.SpecUnimportant + "Downloading plugin: "))
		bar.Postfix(colors.ColorizeText(colors.Reset))
		bar.SetMaxWidth(80)
		bar.SetUnits(pb.U_BYTES)
		bar.Start()
	}

	return result, nil
}

func (b *cloudBackend) ListTemplates() ([]workspace.Template, error) {
	return b.client.ListTemplates()
}

func (b *cloudBackend) DownloadTemplate(name string, progress bool) (io.ReadCloser, error) {
	result, size, err := b.client.DownloadTemplate(name)
	if err != nil {
		return nil, errors.Wrap(err, "failed to download template")
	}

	// If progress is requested, and we know the length, show a little animated ASCII progress bar.
	if progress && size != -1 {
		bar := pb.New(int(size))
		result = newBarProxyReadCloser(bar, result)
		bar.Prefix(colors.ColorizeText(colors.SpecUnimportant + "Downloading template: "))
		bar.Postfix(colors.ColorizeText(colors.Reset))
		bar.SetMaxWidth(80)
		bar.SetUnits(pb.U_BYTES)
		bar.Start()
	}

	return result, nil
}

func (b *cloudBackend) GetStack(stackName tokens.QName) (backend.Stack, error) {
	stackID, err := getCloudStackIdentifier(stackName)
	if err != nil {
		return nil, err
	}

	stack, err := b.client.GetStack(stackID)
	if err != nil {
		// If this was a 404, return nil, nil as per this method's contract.
		if errResp, ok := err.(*apitype.ErrorResponse); ok && errResp.Code == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}

	return newStack(stack, b), nil
}

// CreateStackOptions is an optional bag of options specific to creating cloud stacks.
type CreateStackOptions struct {
	// CloudName is the optional PPC name to create the stack in.  If omitted, the organization's default PPC is used.
	CloudName string
}

func (b *cloudBackend) CreateStack(stackName tokens.QName, opts interface{}) (backend.Stack, error) {
	project, err := getCloudProjectIdentifier()
	if err != nil {
		return nil, err
	}

	var cloudName string
	if opts != nil {
		if cloudOpts, ok := opts.(CreateStackOptions); ok {
			cloudName = cloudOpts.CloudName
		} else {
			return nil, errors.New("expected a CloudStackOptions value for opts parameter")
		}
	}

	stack, err := b.client.CreateStack(project, cloudName, string(stackName))
	if err != nil {
		return nil, err
	}
	fmt.Printf("Created stack '%s' hosted in Pulumi Cloud PPC %s\n", stackName, stack.CloudName)

	return newStack(stack, b), nil
}

func (b *cloudBackend) ListStacks() ([]backend.Stack, error) {
	project, err := getCloudProjectIdentifier()
	if err != nil {
		return nil, err
	}

	stacks, err := b.client.ListStacks(project)
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
	stack, err := getCloudStackIdentifier(stackName)
	if err != nil {
		return false, err
	}

	return b.client.DeleteStack(stack, force)
}

// cloudCrypter is an encrypter/decrypter that uses the Pulumi cloud to encrypt/decrypt a stack's secrets.
type cloudCrypter struct {
	backend *cloudBackend
	stack   client.StackIdentifier
}

func (c *cloudCrypter) EncryptValue(plaintext string) (string, error) {
	ciphertext, err := c.backend.client.EncryptValue(c.stack, []byte(plaintext))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func (c *cloudCrypter) DecryptValue(cipherstring string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(cipherstring)
	if err != nil {
		return "", err
	}
	plaintext, err := c.backend.client.DecryptValue(c.stack, ciphertext)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func (b *cloudBackend) GetStackCrypter(stackName tokens.QName) (config.Crypter, error) {
	stack, err := getCloudStackIdentifier(stackName)
	if err != nil {
		return nil, err
	}

	return &cloudCrypter{backend: b, stack: stack}, nil
}

var actionLabels = map[string]string{
	string(client.UpdateKindUpdate):  "Updating",
	string(client.UpdateKindPreview): "Previewing",
	string(client.UpdateKindDestroy): "Destroying",
	"import": "Importing",
}

func (b *cloudBackend) Preview(stackName tokens.QName, pkg *workspace.Project, root string,
	debug bool, opts engine.UpdateOptions, displayOpts backend.DisplayOptions) error {

	return b.updateStack(client.UpdateKindPreview, stackName, pkg, root, debug, backend.UpdateMetadata{}, opts,
		displayOpts)
}

func (b *cloudBackend) Update(stackName tokens.QName, pkg *workspace.Project, root string,
	debug bool, m backend.UpdateMetadata, opts engine.UpdateOptions, displayOpts backend.DisplayOptions) error {

	return b.updateStack(client.UpdateKindUpdate, stackName, pkg, root, debug, m, opts, displayOpts)
}

func (b *cloudBackend) Destroy(stackName tokens.QName, pkg *workspace.Project, root string,
	debug bool, m backend.UpdateMetadata, opts engine.UpdateOptions, displayOpts backend.DisplayOptions) error {

	return b.updateStack(client.UpdateKindDestroy, stackName, pkg, root, debug, m, opts, displayOpts)
}

func (b *cloudBackend) createAndStartUpdate(action client.UpdateKind, stackName tokens.QName, pkg *workspace.Project,
	root string, debug bool, m backend.UpdateMetadata,
	opts engine.UpdateOptions) (client.UpdateIdentifier, int, string, error) {

	stack, err := getCloudStackIdentifier(stackName)
	if err != nil {
		return client.UpdateIdentifier{}, 0, "", err
	}
	context, main, err := getContextAndMain(pkg, root)
	if err != nil {
		return client.UpdateIdentifier{}, 0, "", err
	}
	workspaceStack, err := workspace.DetectProjectStack(stackName)
	if err != nil {
		return client.UpdateIdentifier{}, 0, "", errors.Wrap(err, "getting configuration")
	}
	metadata := apitype.UpdateMetadata{
		Message:     m.Message,
		Environment: m.Environment,
	}
	getContents := func() (io.ReadCloser, int64, error) {
		const showProgress = true
		return getUpdateContents(context, pkg.UseDefaultIgnores(), showProgress)
	}
	update, err := b.client.CreateUpdate(action, stack, pkg, workspaceStack.Config, main, metadata, opts, getContents)
	if err != nil {
		return client.UpdateIdentifier{}, 0, "", err
	}

	// Start the update.
	version, token, err := b.client.StartUpdate(update)
	if err != nil {
		return client.UpdateIdentifier{}, 0, "", err
	}
	if action == client.UpdateKindUpdate {
		glog.V(7).Infof("Stack %s being updated to version %d", stackName, version)
	}

	return update, version, token, nil
}

// updateStack performs a the provided type of update on a stack hosted in the Pulumi Cloud.
func (b *cloudBackend) updateStack(action client.UpdateKind, stackName tokens.QName, pkg *workspace.Project,
	root string, debug bool, m backend.UpdateMetadata, opts engine.UpdateOptions,
	displayOpts backend.DisplayOptions) error {

	// Print a banner so it's clear this is going to the cloud.
	actionLabel, ok := actionLabels[string(action)]
	contract.Assertf(ok, "unsupported update kind: %v", action)
	fmt.Printf(
		colors.ColorizeText(
			colors.BrightMagenta+"%s stack '%s' in the Pulumi Cloud"+colors.Reset+cmdutil.EmojiOr(" ☁️", "")+"\n"),
		actionLabel, stackName)

	// First get the stack.
	stack, err := b.GetStack(stackName)
	if err != nil {
		return err
	} else if stack == nil {
		return errors.New("stack not found")
	}

	// Create an update object (except if this won't yield an update; i.e., doing a local preview).
	var update client.UpdateIdentifier
	var version int
	var token string
	if !stack.(Stack).RunLocally() || action != client.UpdateKindPreview {
		update, version, token, err = b.createAndStartUpdate(action, stackName, pkg, root, debug, m, opts)
	}
	if err != nil {
		return err
	}
	if version != 0 {
		// Print a URL afterwards to redirect to the version URL.
		base := b.CloudConsoleStackPath(update.StackIdentifier)
		if link := b.CloudConsoleURL(base, "updates", strconv.Itoa(version)); link != "" {
			defer func() {
				fmt.Printf(
					colors.ColorizeText(
						colors.BrightMagenta+"Permalink: %s"+colors.Reset+"\n"), link)
			}()
		}
	}

	// If we are targeting a stack that uses local operations, run the appropriate engine action locally.
	if stack.(Stack).RunLocally() {
		return b.runEngineAction(action, stackName, pkg, root, debug, opts, displayOpts, update, token)
	}

	// Otherwise, wait for the update to complete while rendering its events to stdout/stderr.
	status, err := b.waitForUpdate(actionLabel, update, displayOpts)
	if err != nil {
		return errors.Wrapf(err, "waiting for %s", action)
	} else if status != apitype.StatusSucceeded {
		return errors.Errorf("%s unsuccessful: status %v", action, status)
	}

	return nil
}

// uploadArchive archives the current Pulumi program and uploads it to a signed URL. "current"
// meaning whatever Pulumi program is found in the CWD or parent directory.
// If set, printSize will print the size of the data being uploaded.
func getUpdateContents(context string, useDefaultIgnores bool, progress bool) (io.ReadCloser, int64, error) {
	archiveContents, err := archive.Process(context, useDefaultIgnores)
	if err != nil {
		return nil, 0, errors.Wrap(err, "creating archive")
	}

	archiveReader := ioutil.NopCloser(archiveContents)

	// If progress is requested, show a little animated ASCII progress bar.
	if progress {
		bar := pb.New(archiveContents.Len())
		archiveReader = newBarProxyReadCloser(bar, archiveReader)
		bar.Prefix(colors.ColorizeText(colors.SpecUnimportant + "Uploading program: "))
		bar.Postfix(colors.ColorizeText(colors.Reset))
		bar.SetMaxWidth(80)
		bar.SetUnits(pb.U_BYTES)
		bar.Start()
	}

	return archiveReader, int64(archiveContents.Len()), nil
}

func (b *cloudBackend) runEngineAction(action client.UpdateKind, stackName tokens.QName, pkg *workspace.Project,
	root string, debug bool, opts engine.UpdateOptions, displayOpts backend.DisplayOptions,
	update client.UpdateIdentifier, token string) error {

	u, err := b.newUpdate(stackName, pkg, root, update, token)
	if err != nil {
		return err
	}

	events := make(chan engine.Event)
	done := make(chan bool)

	actionLabel, ok := actionLabels[string(action)]
	contract.Assertf(ok, "unsupported update kind: %v", action)
	go u.RecordAndDisplayEvents(actionLabel, events, done, debug, displayOpts)

	switch action {
	case client.UpdateKindPreview:
		err = engine.Preview(u, events, opts)
	case client.UpdateKindUpdate:
		_, err = engine.Update(u, events, opts)
	case client.UpdateKindDestroy:
		_, err = engine.Destroy(u, events, opts)
	}

	<-done
	close(events)
	close(done)

	if action != client.UpdateKindPreview {
		status := apitype.UpdateStatusSucceeded
		if err != nil {
			status = apitype.UpdateStatusFailed
		}
		completeErr := u.Complete(status)
		if completeErr != nil {
			err = multierror.Append(err, completeErr)
		}
	}
	return err
}

func (b *cloudBackend) GetHistory(stackName tokens.QName) ([]backend.UpdateInfo, error) {
	stack, err := getCloudStackIdentifier(stackName)
	if err != nil {
		return nil, err
	}

	updates, err := b.client.GetStackUpdates(stack)
	if err != nil {
		return nil, err
	}

	// Convert apitype.UpdateInfo objects to the backend type.
	var beUpdates []backend.UpdateInfo
	for _, update := range updates {
		// Convert types from the apitype package into their internal counterparts.
		cfg, err := convertConfig(update.Config)
		if err != nil {
			return nil, errors.Wrap(err, "converting configuration")
		}

		beUpdates = append(beUpdates, backend.UpdateInfo{
			Kind:            backend.UpdateKind(update.Kind),
			Message:         update.Message,
			Environment:     update.Environment,
			Config:          cfg,
			Result:          backend.UpdateResult(update.Result),
			StartTime:       update.StartTime,
			EndTime:         update.EndTime,
			Deployment:      update.Deployment,
			ResourceChanges: convertResourceChanges(update.ResourceChanges),
		})
	}

	return beUpdates, nil
}

// convertResourceChanges converts the apitype version of engine.ResourceChanges into the internal version.
func convertResourceChanges(changes map[apitype.OpType]int) engine.ResourceChanges {
	b := make(engine.ResourceChanges)
	for k, v := range changes {
		b[deploy.StepOp(k)] = v
	}
	return b
}

// convertResourceChanges converts the apitype version of config.Map into the internal version.
func convertConfig(apiConfig map[string]apitype.ConfigValue) (config.Map, error) {
	c := make(config.Map)
	for rawK, rawV := range apiConfig {
		k, err := config.ParseKey(rawK)
		if err != nil {
			return nil, err
		}
		if rawV.Secret {
			c[k] = config.NewSecureValue(rawV.String)
		} else {
			c[k] = config.NewValue(rawV.String)
		}
	}
	return c, nil
}

func (b *cloudBackend) GetLogs(stackName tokens.QName, logQuery operations.LogQuery) ([]operations.LogEntry, error) {
	stack, err := b.GetStack(stackName)
	if err != nil {
		return nil, err
	}
	if stack == nil {
		return nil, errors.New("stack not found")
	}

	// If we're dealing with a stack that runs its operations locally, get the stack's target and fetch the logs
	// directly
	if stack.(Stack).RunLocally() {
		target, targetErr := b.getTarget(stackName)
		if targetErr != nil {
			return nil, targetErr
		}
		return local.GetLogsForTarget(target, logQuery)
	}

	// Otherwise, fetch the logs from the service.
	stackID, err := getCloudStackIdentifier(stackName)
	if err != nil {
		return nil, err
	}
	return b.client.GetStackLogs(stackID, logQuery)
}

func (b *cloudBackend) ExportDeployment(stackName tokens.QName) (*apitype.UntypedDeployment, error) {
	stack, err := getCloudStackIdentifier(stackName)
	if err != nil {
		return nil, err
	}

	deployment, err := b.client.ExportStackDeployment(stack)
	if err != nil {
		return nil, err
	}

	return &apitype.UntypedDeployment{Deployment: deployment}, nil
}

func (b *cloudBackend) ImportDeployment(stackName tokens.QName, deployment *apitype.UntypedDeployment) error {
	stack, err := getCloudStackIdentifier(stackName)
	if err != nil {
		return err
	}

	update, err := b.client.ImportStackDeployment(stack, deployment.Deployment)
	if err != nil {
		return err
	}

	// Wait for the import to complete, which also polls and renders event output to STDOUT.
	status, err := b.waitForUpdate(actionLabels["import"], update, backend.DisplayOptions{Color: colors.Always})
	if err != nil {
		return errors.Wrap(err, "waiting for import")
	} else if status != apitype.StatusSucceeded {
		return errors.Errorf("import unsuccessful: status %v", status)
	}
	return nil
}

// getCloudProjectIdentifier returns information about the current repository and project, based on the current
// working directory.
func getCloudProjectIdentifier() (client.ProjectIdentifier, error) {
	w, err := workspace.New()
	if err != nil {
		return client.ProjectIdentifier{}, err
	}

	proj, err := workspace.DetectProject()
	if err != nil {
		return client.ProjectIdentifier{}, err
	}

	repo := w.Repository()
	return client.ProjectIdentifier{
		Owner:      repo.Owner,
		Repository: repo.Name,
		Project:    string(proj.Name),
	}, nil
}

// getCloudStackIdentifier returns information about the given stack in the current repository and project, based on
// the current working directory.
func getCloudStackIdentifier(stackName tokens.QName) (client.StackIdentifier, error) {
	project, err := getCloudProjectIdentifier()
	if err != nil {
		return client.StackIdentifier{}, errors.Wrap(err, "failed to detect project")
	}

	return client.StackIdentifier{
		ProjectIdentifier: project,
		Stack:             string(stackName),
	}, nil
}

type DisplayEventType string

const (
	UpdateEvent   DisplayEventType = "UpdateEvent"
	ShutdownEvent DisplayEventType = "Shutdown"
)

type displayEvent struct {
	Kind    DisplayEventType
	Payload interface{}
}

// waitForUpdate waits for the current update of a Pulumi program to reach a terminal state. Returns the
// final state. "path" is the URL endpoint to poll for updates.
func (b *cloudBackend) waitForUpdate(actionLabel string, update client.UpdateIdentifier,
	displayOpts backend.DisplayOptions) (apitype.UpdateStatus, error) {

	events, done := make(chan displayEvent), make(chan bool)
	defer func() {
		events <- displayEvent{Kind: ShutdownEvent, Payload: nil}
		<-done
		close(events)
		close(done)
	}()
	go displayEvents(strings.ToLower(actionLabel), events, done, displayOpts)

	// Events occur in sequence, filter out all the ones we have seen before in each request.
	eventIndex := "0"
	for {
		// Query for the latest update results, including log entries so we can provide active status updates.
		_, results, err := retry.Until(context.Background(), retry.Acceptor{
			Accept: func(try int, nextRetryTime time.Duration) (bool, interface{}, error) {
				return b.tryNextUpdate(update, eventIndex, try, nextRetryTime)
			},
		})
		if err != nil {
			return apitype.StatusFailed, err
		}

		// We got a result, print it out.
		updateResults := results.(apitype.UpdateResults)
		for _, event := range updateResults.Events {
			events <- displayEvent{Kind: UpdateEvent, Payload: event}
			eventIndex = event.Index
		}

		// Check if in termal state and if so return.
		switch updateResults.Status {
		case apitype.StatusFailed, apitype.StatusSucceeded:
			return updateResults.Status, nil
		}
	}
}

func displayEvents(action string, events <-chan displayEvent, done chan<- bool, opts backend.DisplayOptions) {
	prefix := fmt.Sprintf("%s%s...", cmdutil.EmojiOr("✨ ", "@ "), action)
	spinner, ticker := cmdutil.NewSpinnerAndTicker(prefix, nil)

	defer func() {
		spinner.Reset()
		ticker.Stop()
		done <- true
	}()

	for {
		select {
		case <-ticker.C:
			spinner.Tick()
		case event := <-events:
			if event.Kind == ShutdownEvent {
				return
			}

			payload := event.Payload.(apitype.UpdateEvent)
			// Pluck out the string.
			if raw, ok := payload.Fields["text"]; ok && raw != nil {
				if text, ok := raw.(string); ok {
					text = opts.Color.Colorize(text)

					// Choose the stream to write to (by default stdout).
					var stream io.Writer
					if payload.Kind == apitype.StderrEvent {
						stream = os.Stderr
					} else {
						stream = os.Stdout
					}

					if text != "" {
						spinner.Reset()
						fmt.Fprint(stream, text)
					}
				}
			}
		}
	}
}

// tryNextUpdate tries to get the next update for a Pulumi program.  This may time or error out, which resutls in a
// false returned in the first return value.  If a non-nil error is returned, this operation should fail.
func (b *cloudBackend) tryNextUpdate(update client.UpdateIdentifier, afterIndex string, try int,
	nextRetryTime time.Duration) (bool, interface{}, error) {

	// If there is no error, we're done.
	results, err := b.client.GetUpdateEvents(update, afterIndex)
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
				b.CloudURL(), errResp.Code, try, err)
		} else {
			// Otherwise, we will issue an error.
			glog.V(3).Infof("Unexpected %s HTTP %d error after %d retries (erroring): %v",
				b.CloudURL(), errResp.Code, try, err)
			return false, nil, err
		}
	} else {
		glog.V(3).Infof("Unexpected %s error after %d retries (retrying): %v", b.CloudURL(), try, err)
	}

	// Issue a warning if appropriate.
	if warn {
		b.d.Warningf(diag.Message("error querying update status: %v"), err)
		b.d.Warningf(diag.Message("retrying in %vs... ^C to stop (this will not cancel the update)"),
			nextRetryTime.Seconds())
	}

	return false, nil, nil
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
	// Make a request to get the authenticated user. If it returns a successful response,
	// we know the access token is legit. We also parse the response as JSON and confirm
	// it has a githubLogin field that is non-empty (like the Pulumi Service would return).
	_, githubLogin, _, err := client.NewClient(cloud, accessToken).DescribeUser()
	if err != nil {
		if errResp, ok := err.(*apitype.ErrorResponse); ok && errResp.Code == 401 {
			return false, nil
		}
		return false, errors.Wrapf(err, "getting user info from %v", cloud)
	}

	if githubLogin == "" {
		return false, errors.New("unexpected response from cloud API")
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
		b, err := New(d, url)
		if err != nil {
			return nil, "", errors.Wrapf(err, "creating backend for %s", url)
		}

		backends = append(backends, b)
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
