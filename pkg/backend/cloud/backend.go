// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.

package cloud

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"runtime"
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
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/archive"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/retry"
	"github.com/pulumi/pulumi/pkg/workspace"

	survey "gopkg.in/AlecAivazis/survey.v1"
	surveycore "gopkg.in/AlecAivazis/survey.v1/core"
)

const (
	// PulumiCloudURL is the Cloud URL used if no environment or explicit cloud is chosen.
	PulumiCloudURL = "https://" + defaultAPIURLPrefix + "pulumi.com"
	// defaultAPIURLPrefix is the assumed Cloud URL prefix for typical Pulumi Cloud API endpoints.
	defaultAPIURLPrefix = "api."
	// defaultAPIEnvVar can be set to override the default cloud chosen, if `--cloud` is not present.
	defaultURLEnvVar = "PULUMI_API"
	// AccessTokenEnvVar is the environment variable used to bypass a prompt on login.
	AccessTokenEnvVar = "PULUMI_ACCESS_TOKEN"
)

// DefaultURL returns the default cloud URL.  This may be overridden using the PULUMI_API environment
// variable.  If no override is found, and we are authenticated with a cloud, choose that.  Otherwise,
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

	// If that didn't work, see if we have a current cloud, and use that. Note we need to be careful
	// to ignore the local cloud.
	if creds, err := workspace.GetStoredCredentials(); err == nil {
		if creds.Current != "" && !local.IsLocalBackendURL(creds.Current) {
			return creds.Current
		}
	}

	// If none of those led to a cloud URL, simply return the default.
	return PulumiCloudURL
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
	DownloadTemplate(name string, progress bool) (io.ReadCloser, error)
	ListTemplates() ([]workspace.Template, error)

	CancelCurrentUpdate(stackRef backend.StackReference) error
	StackConsoleURL(stackRef backend.StackReference) (string, error)
}

type cloudBackend struct {
	d      diag.Sink
	url    string
	client *client.Client
}

// New creates a new Pulumi backend for the given cloud API URL and token.
func New(d diag.Sink, cloudURL string) (Backend, error) {
	cloudURL = ValueOrDefaultURL(cloudURL)
	apiToken, err := workspace.GetAccessToken(cloudURL)
	if err != nil {
		return nil, errors.Wrap(err, "getting stored credentials")
	}

	return &cloudBackend{
		d:      d,
		url:    cloudURL,
		client: client.NewClient(cloudURL, apiToken),
	}, nil
}

// Login logs into the target cloud URL and returns the cloud backend for it.
func Login(d diag.Sink, cloudURL string) (Backend, error) {
	cloudURL = ValueOrDefaultURL(cloudURL)

	// If we have a saved access token, and it is valid, use it.
	existingToken, err := workspace.GetAccessToken(cloudURL)
	if err == nil && existingToken != "" {
		if valid, _ := IsValidAccessToken(cloudURL, existingToken); valid {
			// Save the token. While it hasn't changed this will update the current cloud we are logged into, as well.
			if err = workspace.StoreAccessToken(cloudURL, existingToken, true); err != nil {
				return nil, err
			}

			return New(d, cloudURL)
		}
	}

	// We intentionally don't accept command-line args for the user's access token. Having it in
	// .bash_history is not great, and specifying it via flag isn't of much use.
	accessToken := os.Getenv(AccessTokenEnvVar)
	if accessToken != "" {
		fmt.Printf("Using access token from %s\n", AccessTokenEnvVar)
	} else {
		token, readerr := cmdutil.ReadConsoleNoEcho(
			fmt.Sprintf("Enter your Pulumi access token from %s", cloudConsoleURL(cloudURL, "account")))
		if readerr != nil {
			return nil, readerr
		}
		accessToken = token
	}

	// Try and use the credentials to see if they are valid.
	valid, err := IsValidAccessToken(cloudURL, accessToken)
	if err != nil {
		return nil, err
	} else if !valid {
		return nil, errors.Errorf("invalid access token")
	}

	// Save them.
	if err = workspace.StoreAccessToken(cloudURL, accessToken, true); err != nil {
		return nil, err
	}

	return New(d, cloudURL)
}

func (b *cloudBackend) StackConsoleURL(stackRef backend.StackReference) (string, error) {
	stackID, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return "", err
	}

	return b.cloudConsoleStackPath(stackID), nil
}

func (b *cloudBackend) Name() string {
	if b.url == PulumiCloudURL {
		return "pulumi.com"
	}

	return b.url
}

func (b *cloudBackend) CloudURL() string { return b.url }

func (b *cloudBackend) ParseStackReference(s string) (backend.StackReference, error) {
	split := strings.Split(s, "/")
	var owner string
	var stackName string

	if len(split) == 1 {
		stackName = split[0]
	} else if len(split) == 2 {
		owner = split[0]
		stackName = split[1]
	} else {
		return nil, errors.Errorf("could not parse stack name '%s'", s)
	}

	if owner == "" {
		currentUser, userErr := b.client.GetPulumiAccountName()
		if userErr != nil {
			return nil, userErr
		}
		owner = currentUser
	}

	return cloudBackendReference{
		owner: owner,
		name:  tokens.QName(stackName),
		b:     b,
	}, nil
}

// CloudConsoleURL returns a link to the cloud console with the given path elements.  If a console link cannot be
// created, we return the empty string instead (this can happen if the endpoint isn't a recognized pattern).
func (b *cloudBackend) CloudConsoleURL(paths ...string) string {
	return cloudConsoleURL(b.CloudURL(), paths...)
}

func cloudConsoleURL(cloudURL string, paths ...string) string {
	// To produce a cloud console URL, we assume that the URL is of the form `api.xx.yy`, and simply strip off the
	// `api.` part.  If that is not the case, we will return an empty string because we don't recognize the pattern.
	ix := strings.Index(cloudURL, defaultAPIURLPrefix)
	if ix == -1 {
		return ""
	}
	return cloudURL[:ix] + path.Join(append([]string{cloudURL[ix+len(defaultAPIURLPrefix):]}, paths...)...)
}

// cloudConsoleProjectPath returns the project path components for getting to a stack in the cloud console.  This path
// must, of course, be combined with the actual console base URL by way of the CloudConsoleURL function above.
func (b *cloudBackend) cloudConsoleProjectPath(projID client.ProjectIdentifier) string {
	// When projID.Repository is the empty string, we are using the new identity model. In this case, the service
	// does not include project or repository information in URLS, so the "project path" is simply the owner.
	//
	// TODO(ellismg)[pulumi/pulumi#1241] Clean this up once we remove pulumi init
	if projID.Repository == "" {
		return projID.Owner
	}

	return path.Join(projID.Owner, projID.Repository, projID.Project)
}

// CloudConsoleStackPath returns the stack path components for getting to a stack in the cloud console.  This path
// must, of coursee, be combined with the actual console base URL by way of the CloudConsoleURL function above.
func (b *cloudBackend) cloudConsoleStackPath(stackID client.StackIdentifier) string {
	return path.Join(b.cloudConsoleProjectPath(stackID.ProjectIdentifier), stackID.Stack)
}

// Logout logs out of the target cloud URL.
func (b *cloudBackend) Logout() error {
	return workspace.DeleteAccessToken(b.CloudURL())
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

func (b *cloudBackend) GetStack(stackRef backend.StackReference) (backend.Stack, error) {
	stackID, err := b.getCloudStackIdentifier(stackRef)
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

func ownerFromRef(stackRef backend.StackReference) string {
	if r, ok := stackRef.(cloudBackendReference); ok {
		return r.owner
	}

	contract.Failf("bad StackReference type")
	return ""
}

func (b *cloudBackend) CreateStack(stackRef backend.StackReference, opts interface{}) (backend.Stack, error) {
	if opts == nil {
		opts = CreateStackOptions{}
	}

	cloudOpts, ok := opts.(CreateStackOptions)
	if !ok {
		return nil, errors.New("expected a CloudStackOptions value for opts parameter")
	}

	project, err := b.getCloudProjectIdentifier(ownerFromRef(stackRef))
	if err != nil {
		return nil, err
	}

	tags, err := backend.GetStackTags()
	if err != nil {
		return nil, errors.Wrap(err, "error determining initial tags")
	}

	apistack, err := b.client.CreateStack(project, cloudOpts.CloudName, string(stackRef.StackName()), tags)
	if err != nil {
		return nil, err
	}

	stack := newStack(apistack, b)
	fmt.Printf("Created stack '%s'", stack.Name())
	if !stack.RunLocally() {
		fmt.Printf(" in PPC %s", stack.CloudName())
	}
	fmt.Println()

	return stack, nil
}

func (b *cloudBackend) ListStacks(projectFilter *tokens.PackageName) ([]backend.Stack, error) {
	project, err := b.getCloudProjectIdentifier("")
	if err != nil {
		return nil, err
	}

	stacks, err := b.client.ListStacks(project, projectFilter)
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

func (b *cloudBackend) RemoveStack(stackRef backend.StackReference, force bool) (bool, error) {
	stack, err := b.getCloudStackIdentifier(stackRef)
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

func (b *cloudBackend) GetStackCrypter(stackRef backend.StackReference) (config.Crypter, error) {
	stack, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return nil, err
	}

	return &cloudCrypter{backend: b, stack: stack}, nil
}

var (
	updateTextMap = map[string]struct {
		previewText string
		text        string
	}{
		string(client.UpdateKindUpdate):  {"update of", "Updating"},
		string(client.UpdateKindRefresh): {"refresh of", "Refreshing"},
		string(client.UpdateKindDestroy): {"destroy of", "Destroying"},
		string(client.UpdateKindImport):  {"import to", "Importing into"},
	}
)

func getActionLabel(key string, dryRun bool) string {
	v := updateTextMap[key]
	contract.Assert(v.previewText != "")
	contract.Assert(v.text != "")

	if dryRun {
		return "Previewing " + v.previewText
	}

	return v.text
}

type response string

const (
	yes     response = "yes"
	no      response = "no"
	details response = "details"
)

func getStack(b *cloudBackend, stackRef backend.StackReference) (backend.Stack, error) {
	stack, err := b.GetStack(stackRef)
	if err != nil {
		return nil, err
	} else if stack == nil {
		return nil, errors.New("stack not found")
	}

	return stack, nil
}

func createDiff(events []engine.Event, displayOpts backend.DisplayOptions) string {
	buff := &bytes.Buffer{}

	seen := make(map[resource.URN]engine.StepEventMetadata)
	displayOpts.SummaryDiff = true

	for _, e := range events {
		msg := local.RenderDiffEvent(e, seen, displayOpts)
		if msg != "" {
			if e.Type == engine.SummaryEvent {
				msg = "\n" + msg
			}

			_, err := buff.WriteString(msg)
			contract.IgnoreError(err)
		}
	}

	return strings.TrimSpace(buff.String())
}

func (b *cloudBackend) PreviewThenPrompt(
	updateKind client.UpdateKind,
	stack backend.Stack, pkg *workspace.Project, root string,
	m backend.UpdateMetadata, opts engine.UpdateOptions,
	displayOpts backend.DisplayOptions, scopes backend.CancellationScopeSource) error {

	// create a channel to hear about the update events from the engine. this will be used so that
	// we can build up the diff display in case the user asks to see the details of the diff
	var eventsChannel chan engine.Event
	events := []engine.Event{}

	// if we're previewing, we don't need to store the events as we're not going to prompt
	// the user to get details of what's happening.
	if !opts.Preview {
		eventsChannel = make(chan engine.Event)
		defer func() {
			close(eventsChannel)
		}()

		go func() {
			// pull the events from the channel and store them locally
			for e := range eventsChannel {
				if e.Type == engine.ResourcePreEvent ||
					e.Type == engine.ResourceOutputsEvent ||
					e.Type == engine.SummaryEvent {

					events = append(events, e)
				}
			}
		}()
	}

	err := b.updateStack(
		updateKind, stack, pkg, root, m,
		opts, displayOpts, eventsChannel, true /*dryRun*/, scopes)

	if err != nil || opts.Preview {
		// if we're just previewing, then we can stop at this point.
		return err
	}

	for {
		var response string

		surveycore.DisableColor = true
		surveycore.QuestionIcon = ""
		surveycore.SelectFocusIcon = colors.ColorizeText(colors.BrightGreen + ">" + colors.Reset)

		options := []string{string(yes), string(no)}

		// if this is a managed stack, then we can get the details for the operation, as we will
		// have been able to collect the details while the preview ran.  For ppc stacks, we don't
		// have that information since all the PPC does is forward stdout events to us.
		if stack.(Stack).RunLocally() {
			options = append(options, string(details))
		}
		err = survey.AskOne(&survey.Select{
			Message: colors.ColorizeText(colors.BrightWhite + "Do you want to proceed?" + colors.Reset),
			Options: options,
			Default: string(no),
		}, &response, nil)

		if err != nil {
			return err
		}

		if response == string(no) {
			return errors.New("confirmation declined")
		}

		if response == string(yes) {
			return nil
		}

		if response == string(details) {
			diff := createDiff(events, displayOpts)
			_, err = os.Stdout.WriteString(diff + "\n\n")
			contract.IgnoreError(err)
			continue
		}

		// Anything else just causes us to bail out.  This can happen, for example, when
		// ctrl-c is hit.
		return err
	}
}

func (b *cloudBackend) PreviewThenPromptThenExecute(
	updateKind client.UpdateKind,
	stackRef backend.StackReference, pkg *workspace.Project, root string,
	m backend.UpdateMetadata, opts engine.UpdateOptions,
	displayOpts backend.DisplayOptions, scopes backend.CancellationScopeSource) error {

	// First get the stack.
	stack, err := getStack(b, stackRef)
	if err != nil {
		return err
	}

	if !opts.Force {
		// If we're not forcing, then preview the operation to the user and ask them if
		// they want to proceed.
		err = b.PreviewThenPrompt(updateKind, stack, pkg, root, m, opts, displayOpts, scopes)
		if err != nil || opts.Preview {
			return err
		}
	}

	// Now do the real operation.  We don't care about the events it issues, so just
	// pass a nil channel along.
	var unused chan engine.Event
	return b.updateStack(
		updateKind, stack, pkg,
		root, m, opts, displayOpts, unused, false /*dryRun*/, scopes)
}

func (b *cloudBackend) Update(stackRef backend.StackReference, pkg *workspace.Project, root string,
	m backend.UpdateMetadata, opts engine.UpdateOptions, displayOpts backend.DisplayOptions,
	scopes backend.CancellationScopeSource) error {

	return b.PreviewThenPromptThenExecute(client.UpdateKindUpdate, stackRef, pkg, root, m, opts, displayOpts, scopes)
}

func (b *cloudBackend) Refresh(stackRef backend.StackReference, pkg *workspace.Project, root string,
	m backend.UpdateMetadata, opts engine.UpdateOptions, displayOpts backend.DisplayOptions,
	scopes backend.CancellationScopeSource) error {

	return b.PreviewThenPromptThenExecute(client.UpdateKindRefresh, stackRef, pkg, root, m, opts, displayOpts, scopes)
}

func (b *cloudBackend) Destroy(stackRef backend.StackReference, pkg *workspace.Project, root string,
	m backend.UpdateMetadata, opts engine.UpdateOptions, displayOpts backend.DisplayOptions,
	scopes backend.CancellationScopeSource) error {

	return b.PreviewThenPromptThenExecute(client.UpdateKindDestroy, stackRef, pkg, root, m, opts, displayOpts, scopes)
}

func (b *cloudBackend) createAndStartUpdate(
	action client.UpdateKind, stackRef backend.StackReference,
	pkg *workspace.Project, root string, m backend.UpdateMetadata,
	opts engine.UpdateOptions, dryRun bool) (client.UpdateIdentifier, int, string, error) {

	stack, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return client.UpdateIdentifier{}, 0, "", err
	}
	context, main, err := getContextAndMain(pkg, root)
	if err != nil {
		return client.UpdateIdentifier{}, 0, "", err
	}
	workspaceStack, err := workspace.DetectProjectStack(stackRef.StackName())
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
	update, err := b.client.CreateUpdate(
		action, stack, pkg, workspaceStack.Config, main, metadata, opts, dryRun, getContents)
	if err != nil {
		return client.UpdateIdentifier{}, 0, "", err
	}

	// Start the update. We use this opportunity to pass new tags to the service, to pick up any
	// metadata changes.
	tags, err := backend.GetStackTags()
	if err != nil {
		return client.UpdateIdentifier{}, 0, "", errors.Wrap(err, "getting stack tags")
	}
	version, token, err := b.client.StartUpdate(update, tags)
	if err != nil {
		return client.UpdateIdentifier{}, 0, "", err
	}
	if action == client.UpdateKindUpdate {
		glog.V(7).Infof("Stack %s being updated to version %d", stackRef, version)
	}

	return update, version, token, nil
}

// updateStack performs a the provided type of update on a stack hosted in the Pulumi Cloud.
func (b *cloudBackend) updateStack(
	action client.UpdateKind, stack backend.Stack, pkg *workspace.Project,
	root string, m backend.UpdateMetadata, opts engine.UpdateOptions,
	displayOpts backend.DisplayOptions, callerEventsOpt chan<- engine.Event, dryRun bool,
	scopes backend.CancellationScopeSource) error {

	// Print a banner so it's clear this is going to the cloud.
	actionLabel := getActionLabel(string(action), dryRun)
	fmt.Printf(
		colors.ColorizeText(colors.BrightMagenta+"%s stack '%s'"+colors.Reset+"\n"),
		actionLabel, stack.Name())

	// Create an update object (except if this won't yield an update; i.e., doing a local preview).
	var update client.UpdateIdentifier
	var version int
	var token string
	var err error
	if !stack.(Stack).RunLocally() || !dryRun {
		update, version, token, err = b.createAndStartUpdate(
			action, stack.Name(), pkg, root, m, opts, dryRun)
	}
	if err != nil {
		return err
	}

	if version != 0 {
		// Print a URL afterwards to redirect to the version URL.
		base := b.cloudConsoleStackPath(update.StackIdentifier)
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
		return b.runEngineAction(
			action, stack.Name(), pkg, root, opts, displayOpts,
			update, token, callerEventsOpt, dryRun, scopes)
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

func (b *cloudBackend) runEngineAction(
	action client.UpdateKind, stackRef backend.StackReference, pkg *workspace.Project,
	root string, opts engine.UpdateOptions, displayOpts backend.DisplayOptions,
	update client.UpdateIdentifier, token string,
	callerEventsOpt chan<- engine.Event, dryRun bool, scopes backend.CancellationScopeSource) error {

	u, err := b.newUpdate(stackRef, pkg, root, update, token)
	if err != nil {
		return err
	}

	persister := b.newSnapshotPersister(u.update, u.tokenSource)
	manager := backend.NewSnapshotManager(stackRef.StackName(), persister, u.GetTarget().Snapshot)
	displayEvents := make(chan engine.Event)
	displayDone := make(chan bool)

	go u.RecordAndDisplayEvents(
		getActionLabel(string(action), dryRun), displayEvents, displayDone, displayOpts)

	engineEvents := make(chan engine.Event)

	scope := scopes.NewScope(engineEvents, dryRun)
	defer scope.Close()

	go func() {
		// Pull in all events from the engine and send to them to the two listeners.
		for e := range engineEvents {
			displayEvents <- e

			if callerEventsOpt != nil {
				callerEventsOpt <- e
			}
		}
	}()

	// Depending on the action, kick off the relevant engine activity.  Note that we don't immediately check and
	// return error conditions, because we will do so below after waiting for the display channels to close.
	engineCtx := &engine.Context{Cancel: scope.Context(), Events: engineEvents, SnapshotManager: manager}
	switch action {
	case client.UpdateKindUpdate:
		if dryRun {
			err = engine.Preview(u, engineCtx, opts)
		} else {
			_, err = engine.Update(u, engineCtx, opts, dryRun)
		}
	case client.UpdateKindRefresh:
		_, err = engine.Refresh(u, engineCtx, opts, dryRun)
	case client.UpdateKindDestroy:
		_, err = engine.Destroy(u, engineCtx, opts, dryRun)
	default:
		contract.Failf("Unrecognized action type: %s", action)
	}

	// Wait for the display to finish showing all the events.
	<-displayDone
	close(engineEvents)
	close(displayEvents)
	close(displayDone)
	contract.IgnoreClose(manager)

	if !dryRun {
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

func (b *cloudBackend) CancelCurrentUpdate(stackRef backend.StackReference) error {
	stackID, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return err
	}
	stack, err := b.client.GetStack(stackID)
	if err != nil {
		return err
	}

	// Compute the update identifier and attempt to cancel the update.
	//
	// NOTE: the update kind is not relevant; the same endpoint will work for updates of all kinds.
	updateID := client.UpdateIdentifier{
		StackIdentifier: stackID,
		UpdateKind:      client.UpdateKindUpdate,
		UpdateID:        stack.ActiveUpdate,
	}
	return b.client.CancelUpdate(updateID)
}

func (b *cloudBackend) GetHistory(stackRef backend.StackReference) ([]backend.UpdateInfo, error) {
	stack, err := b.getCloudStackIdentifier(stackRef)
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
		// Decode the deployment.
		if update.Version > 1 {
			return nil, errors.Errorf("unsupported checkpoint version %v", update.Version)
		}
		var deployment apitype.DeploymentV1
		if err := json.Unmarshal([]byte(update.Deployment), &deployment); err != nil {
			return nil, err
		}

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
			Deployment:      &deployment,
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

func (b *cloudBackend) GetLogs(stackRef backend.StackReference,
	logQuery operations.LogQuery) ([]operations.LogEntry, error) {

	stack, err := b.GetStack(stackRef)
	if err != nil {
		return nil, err
	}
	if stack == nil {
		return nil, errors.New("stack not found")
	}

	// If we're dealing with a stack that runs its operations locally, get the stack's target and fetch the logs
	// directly
	if stack.(Stack).RunLocally() {
		target, targetErr := b.getTarget(stackRef)
		if targetErr != nil {
			return nil, targetErr
		}
		return local.GetLogsForTarget(target, logQuery)
	}

	// Otherwise, fetch the logs from the service.
	stackID, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return nil, err
	}
	return b.client.GetStackLogs(stackID, logQuery)
}

func (b *cloudBackend) ExportDeployment(stackRef backend.StackReference) (*apitype.UntypedDeployment, error) {
	stack, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return nil, err
	}

	deployment, err := b.client.ExportStackDeployment(stack)
	if err != nil {
		return nil, err
	}

	return &deployment, nil
}

func (b *cloudBackend) ImportDeployment(stackRef backend.StackReference, deployment *apitype.UntypedDeployment) error {
	stack, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return err
	}

	update, err := b.client.ImportStackDeployment(stack, deployment.Deployment)
	if err != nil {
		return err
	}

	// Wait for the import to complete, which also polls and renders event output to STDOUT.
	status, err := b.waitForUpdate(
		getActionLabel("import", false /*dryRun*/), update,
		backend.DisplayOptions{Color: colors.Always})
	if err != nil {
		return errors.Wrap(err, "waiting for import")
	} else if status != apitype.StatusSucceeded {
		return errors.Errorf("import unsuccessful: status %v", status)
	}
	return nil
}

// getCloudProjectIdentifier returns information about the current repository and project, based on the current
// working directory.
func (b *cloudBackend) getCloudProjectIdentifier(owner string) (client.ProjectIdentifier, error) {
	w, err := workspace.New()
	if err != nil {
		return client.ProjectIdentifier{}, err
	}

	proj, err := workspace.DetectProject()
	if err != nil {
		return client.ProjectIdentifier{}, err
	}

	repo := w.Repository()

	// If we have repository information (this is the case when `pulumi init` has been run, use that.) To support the
	// old and new model, we either set the Repository field in ProjectIdentifer or keep it as the empty string. The
	// client type uses this to decide what REST endpoints to hit.
	//
	// TODO(ellismg)[pulumi/pulumi#1241] Clean this up once we remove pulumi init
	if repo != nil {
		return client.ProjectIdentifier{
			Owner:      repo.Owner,
			Repository: repo.Name,
			Project:    string(proj.Name),
		}, nil
	}

	if owner == "" {
		owner, err = b.client.GetPulumiAccountName()
		if err != nil {
			return client.ProjectIdentifier{}, err
		}
	}

	// Otherwise, we are on the new plan.
	return client.ProjectIdentifier{
		Owner:   owner,
		Project: string(proj.Name),
	}, nil
}

// getCloudStackIdentifier returns information about the given stack in the current repository and project, based on
// the current working directory.
func (b *cloudBackend) getCloudStackIdentifier(stackRef backend.StackReference) (client.StackIdentifier, error) {
	project, err := b.getCloudProjectIdentifier(ownerFromRef(stackRef))
	if err != nil {
		return client.StackIdentifier{}, errors.Wrap(err, "failed to detect project")
	}

	return client.StackIdentifier{
		ProjectIdentifier: project,
		Stack:             string(stackRef.StackName()),
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

	// The UpdateEvents API returns a continuation token to only get events after the previous call.
	var continuationToken *string
	for {
		// Query for the latest update results, including log entries so we can provide active status updates.
		_, results, err := retry.Until(context.Background(), retry.Acceptor{
			Accept: func(try int, nextRetryTime time.Duration) (bool, interface{}, error) {
				return b.tryNextUpdate(update, continuationToken, try, nextRetryTime)
			},
		})
		if err != nil {
			return apitype.StatusFailed, err
		}

		// We got a result, print it out.
		updateResults := results.(apitype.UpdateResults)
		for _, event := range updateResults.Events {
			events <- displayEvent{Kind: UpdateEvent, Payload: event}
		}

		continuationToken = updateResults.ContinuationToken
		// A nil continuation token means there are no more events to read and the update has finished.
		if continuationToken == nil {
			return updateResults.Status, nil
		}
	}
}

func displayEvents(
	action string, events <-chan displayEvent, done chan<- bool, opts backend.DisplayOptions) {

	prefix := fmt.Sprintf("%s%s...", cmdutil.EmojiOr("✨ ", "@ "), action)
	spinner, ticker := cmdutil.NewSpinnerAndTicker(prefix, nil, 8 /*timesPerSecond*/)

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

			// Pluck out the string.
			payload := event.Payload.(apitype.UpdateEvent)
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

// tryNextUpdate tries to get the next update for a Pulumi program.  This may time or error out, which results in a
// false returned in the first return value.  If a non-nil error is returned, this operation should fail.
func (b *cloudBackend) tryNextUpdate(update client.UpdateIdentifier, continuationToken *string, try int,
	nextRetryTime time.Duration) (bool, interface{}, error) {

	// If there is no error, we're done.
	results, err := b.client.GetUpdateEvents(update, continuationToken)
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
		b.d.Warningf(diag.Message("" /*urn*/, "error querying update status: %v"), err)
		b.d.Warningf(diag.Message("" /*urn*/, "retrying in %vs... ^C to stop (this will not cancel the update)"),
			nextRetryTime.Seconds())
	}

	return false, nil, nil
}

// IsValidAccessToken tries to use the provided Pulumi access token and returns if it is accepted
// or not. Returns error on any unexpected error.
func IsValidAccessToken(cloudURL, accessToken string) (bool, error) {
	// Make a request to get the authenticated user. If it returns a successful response,
	// we know the access token is legit. We also parse the response as JSON and confirm
	// it has a githubLogin field that is non-empty (like the Pulumi Service would return).
	_, err := client.NewClient(cloudURL, accessToken).GetPulumiAccountName()
	if err != nil {
		if errResp, ok := err.(*apitype.ErrorResponse); ok && errResp.Code == 401 {
			return false, nil
		}
		return false, errors.Wrapf(err, "getting user info from %v", cloudURL)
	}

	return true, nil
}
