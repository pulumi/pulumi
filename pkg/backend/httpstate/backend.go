// Copyright 2016-2018, Pulumi Corporation.
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

package httpstate

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/cheggaaa/pb"
	"github.com/hashicorp/go-multierror"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/skratchdot/open-golang/open"

	"github.com/pulumi/pulumi/pkg/apitype"
	"github.com/pulumi/pulumi/pkg/backend"
	"github.com/pulumi/pulumi/pkg/backend/display"
	"github.com/pulumi/pulumi/pkg/backend/filestate"
	"github.com/pulumi/pulumi/pkg/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/diag"
	"github.com/pulumi/pulumi/pkg/diag/colors"
	"github.com/pulumi/pulumi/pkg/engine"
	"github.com/pulumi/pulumi/pkg/operations"
	"github.com/pulumi/pulumi/pkg/resource"
	"github.com/pulumi/pulumi/pkg/resource/config"
	"github.com/pulumi/pulumi/pkg/resource/deploy"
	"github.com/pulumi/pulumi/pkg/tokens"
	"github.com/pulumi/pulumi/pkg/util/cmdutil"
	"github.com/pulumi/pulumi/pkg/util/contract"
	"github.com/pulumi/pulumi/pkg/util/logging"
	"github.com/pulumi/pulumi/pkg/util/result"
	"github.com/pulumi/pulumi/pkg/util/retry"
	"github.com/pulumi/pulumi/pkg/workspace"
)

const (
	// PulumiCloudURL is the Cloud URL used if no environment or explicit cloud is chosen.
	PulumiCloudURL = "https://" + defaultAPIDomainPrefix + "pulumi.com"
	// defaultAPIDomainPrefix is the assumed Cloud URL prefix for typical Pulumi Cloud API endpoints.
	defaultAPIDomainPrefix = "api."
	// defaultConsoleDomainPrefix is the assumed Cloud URL prefix typically used for the Pulumi Console.
	defaultConsoleDomainPrefix = "app."

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
		if creds.Current != "" && !filestate.IsLocalBackendURL(creds.Current) {
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

	DownloadPlugin(
		ctx context.Context, info workspace.PluginInfo,
		progress bool, opts display.Options) (io.ReadCloser, error)

	CancelCurrentUpdate(ctx context.Context, stackRef backend.StackReference) error
	StackConsoleURL(stackRef backend.StackReference) (string, error)
}

type cloudBackend struct {
	d               diag.Sink
	url             string
	stackConfigFile string
	client          *client.Client
	currentProject  *workspace.Project
}

// New creates a new Pulumi backend for the given cloud API URL and token.
func New(d diag.Sink, cloudURL, stackConfigFile string) (Backend, error) {
	cloudURL = ValueOrDefaultURL(cloudURL)
	apiToken, err := workspace.GetAccessToken(cloudURL)
	if err != nil {
		return nil, errors.Wrap(err, "getting stored credentials")
	}

	// When stringifying backend references, we take the current project (if present) into account.
	currentProject, err := workspace.DetectProject()
	if err != nil {
		currentProject = nil
	}

	return &cloudBackend{
		d:               d,
		url:             cloudURL,
		stackConfigFile: stackConfigFile,
		client:          client.NewClient(cloudURL, apiToken, d),
		currentProject:  currentProject,
	}, nil
}

// loginWithBrowser uses a web-browser to log into the cloud and returns the cloud backend for it.
func loginWithBrowser(ctx context.Context, d diag.Sink, cloudURL, stackConfigFile string) (Backend, error) {
	// Locally, we generate a nonce and spin up a web server listening on a random port on localhost. We then open a
	// browser to a special endpoint on the Pulumi.com console, passing the generated nonce as well as the port of the
	// webserver we launched. This endpoint does the OAuth flow and when it completes, redirects to localhost passing
	// the nonce and the pulumi access token we created as part of the OAuth flow. If the nonces match, we set the
	// access token that was passed to us and the redirect to a special welcome page on Pulumi.com

	loginURL := cloudConsoleURL(cloudURL, "cli-login")
	finalWelcomeURL := cloudConsoleURL(cloudURL, "welcome", "cli")

	if loginURL == "" || finalWelcomeURL == "" {
		return nil, errors.New("could not determine login url")
	}

	// Listen on localhost, have the kernel pick a random port for us
	c := make(chan string)
	l, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		return nil, errors.Wrap(err, "could not start listener")
	}

	// Extract the port
	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return nil, errors.Wrap(err, "could not determine port")
	}

	// Generate a nonce we'll send with the request.
	nonceBytes := make([]byte, 32)
	_, err = cryptorand.Read(nonceBytes)
	contract.AssertNoErrorf(err, "could not get random bytes")
	nonce := hex.EncodeToString(nonceBytes)

	u, err := url.Parse(loginURL)
	contract.AssertNoError(err)

	// Generate a description to associate with the access token we'll generate, for display on the Account Settings
	// page.
	var tokenDescription string
	if host, hostErr := os.Hostname(); hostErr == nil {
		tokenDescription = fmt.Sprintf("Generated by pulumi login on %s at %s", host, time.Now().Format(time.RFC822))
	} else {
		tokenDescription = fmt.Sprintf("Generated by pulumi login at %s", time.Now().Format(time.RFC822))
	}

	// Pass our state around as query parameters on the URL we'll open the user's preferred browser to
	q := u.Query()
	q.Add("cliSessionPort", port)
	q.Add("cliSessionNonce", nonce)
	q.Add("cliSessionDescription", tokenDescription)
	u.RawQuery = q.Encode()

	// Start the webserver to listen to handle the response
	go serveBrowserLoginServer(l, nonce, finalWelcomeURL, c)

	// Launch the web browser and navigate to the login URL.
	if openErr := open.Run(u.String()); openErr != nil {
		fmt.Printf("We couldn't launch your web browser for some reason. Please visit:\n\n%s\n\n"+
			"to finish the login process.", u)
	} else {
		fmt.Println("We've launched your web browser to complete the login process.")
	}

	fmt.Println("\nWaiting for login to complete...")

	accessToken := <-c

	// Save the token and return the backend
	if err = workspace.StoreAccessToken(cloudURL, accessToken, true); err != nil {
		return nil, err
	}

	return New(d, cloudURL, stackConfigFile)
}

// Login logs into the target cloud URL and returns the cloud backend for it.
func Login(ctx context.Context, d diag.Sink, cloudURL, stackConfigFile string, opts display.Options) (Backend, error) {
	cloudURL = ValueOrDefaultURL(cloudURL)

	// If we have a saved access token, and it is valid, use it.
	existingToken, err := workspace.GetAccessToken(cloudURL)
	if err == nil && existingToken != "" {
		if valid, _ := IsValidAccessToken(ctx, cloudURL, existingToken); valid {
			// Save the token. While it hasn't changed this will update the current cloud we are logged into, as well.
			if err = workspace.StoreAccessToken(cloudURL, existingToken, true); err != nil {
				return nil, err
			}

			return New(d, cloudURL, stackConfigFile)
		}
	}

	// We intentionally don't accept command-line args for the user's access token. Having it in
	// .bash_history is not great, and specifying it via flag isn't of much use.
	accessToken := os.Getenv(AccessTokenEnvVar)
	accountLink := cloudConsoleURL(cloudURL, "account", "tokens")

	if accessToken != "" {
		// If there's already a token from the environment, use it.
		_, err = fmt.Fprintf(os.Stderr, "Logging in using access token from %s\n", AccessTokenEnvVar)
		contract.IgnoreError(err)
	} else if !cmdutil.Interactive() {
		// If interactive mode isn't enabled, the only way to specify a token is through the environment variable.
		// Fail the attempt to login.
		return nil, errors.Errorf(
			"%s must be set for login during non-interactive CLI sessions", AccessTokenEnvVar)
	} else {
		// If no access token is available from the environment, and we are interactive, prompt and offer to
		// open a browser to make it easy to generate and use a fresh token.
		line1 := fmt.Sprintf("Manage your Pulumi stacks by logging in.")
		line1len := len(line1)
		line1 = colors.Highlight(line1, "Pulumi stacks", colors.Underline+colors.Bold)
		fmt.Printf(opts.Color.Colorize(line1) + "\n")
		maxlen := line1len

		line2 := "Run `pulumi login --help` for alternative login options."
		line2len := len(line2)
		fmt.Printf(opts.Color.Colorize(line2) + "\n")
		if line2len > maxlen {
			maxlen = line2len
		}

		// In the case where we could not construct a link to the pulumi console based on the API server's hostname,
		// don't offer magic log-in or text about where to find your access token.
		if accountLink == "" {
			for {
				if accessToken, err = cmdutil.ReadConsoleNoEcho("Enter your access token"); err != nil {
					return nil, err
				}
				if accessToken != "" {
					break
				}
			}
		} else {
			line3 := fmt.Sprintf("Enter your access token from %s", accountLink)
			line3len := len(line3)
			line3 = colors.Highlight(line3, "access token", colors.BrightCyan+colors.Bold)
			line3 = colors.Highlight(line3, accountLink, colors.BrightBlue+colors.Underline+colors.Bold)
			fmt.Printf(opts.Color.Colorize(line3) + "\n")
			if line3len > maxlen {
				maxlen = line3len
			}

			line4 := "    or hit <ENTER> to log in using your browser"
			var padding string
			if pad := maxlen - len(line4); pad > 0 {
				padding = strings.Repeat(" ", pad)
			}
			line4 = colors.Highlight(line4, "<ENTER>", colors.BrightCyan+colors.Bold)

			if accessToken, err = cmdutil.ReadConsoleNoEcho(opts.Color.Colorize(line4) + padding); err != nil {
				return nil, err
			}

			if accessToken == "" {
				return loginWithBrowser(ctx, d, cloudURL, stackConfigFile)
			}
		}
	}

	// Try and use the credentials to see if they are valid.
	valid, err := IsValidAccessToken(ctx, cloudURL, accessToken)
	if err != nil {
		return nil, err
	} else if !valid {
		return nil, errors.Errorf("invalid access token")
	}

	// Save them.
	if err = workspace.StoreAccessToken(cloudURL, accessToken, true); err != nil {
		return nil, err
	}

	return New(d, cloudURL, stackConfigFile)
}

func (b *cloudBackend) StackConsoleURL(stackRef backend.StackReference) (string, error) {
	stackID, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return "", err
	}

	path := b.cloudConsoleStackPath(stackID)

	url := b.CloudConsoleURL(path)
	if url == "" {
		return "", errors.New("could not determine clould console URL")
	}
	return url, nil
}

func (b *cloudBackend) Name() string {
	if b.url == PulumiCloudURL {
		return "pulumi.com"
	}

	return b.url
}

func (b *cloudBackend) URL() string {
	user, err := b.CurrentUser()
	if err != nil {
		return cloudConsoleURL(b.url)
	}
	return cloudConsoleURL(b.url, user)
}

func (b *cloudBackend) CurrentUser() (string, error) {
	return b.client.GetPulumiAccountName(context.Background())
}

func (b *cloudBackend) CloudURL() string { return b.url }

func (b *cloudBackend) ParseStackReference(s string) (backend.StackReference, error) {
	split := strings.Split(s, "/")
	var owner string
	var projectName string
	var stackName string

	switch len(split) {
	case 1:
		stackName = split[0]
	case 2:
		owner = split[0]
		stackName = split[1]
	case 3:
		owner = split[0]
		projectName = split[1]
		stackName = split[2]
	default:
		return nil, errors.Errorf("could not parse stack name '%s'", s)
	}

	if owner == "" {
		currentUser, userErr := b.client.GetPulumiAccountName(context.Background())
		if userErr != nil {
			return nil, userErr
		}
		owner = currentUser
	}

	if projectName == "" {
		currentProject, projectErr := workspace.DetectProject()
		if projectErr != nil {
			return nil, projectErr
		}

		projectName = currentProject.Name.String()
	}

	return cloudBackendReference{
		owner:   owner,
		project: projectName,
		name:    tokens.QName(stackName),
		b:       b,
	}, nil
}

// CloudConsoleURL returns a link to the cloud console with the given path elements.  If a console link cannot be
// created, we return the empty string instead (this can happen if the endpoint isn't a recognized pattern).
func (b *cloudBackend) CloudConsoleURL(paths ...string) string {
	return cloudConsoleURL(b.CloudURL(), paths...)
}

// serveBrowserLoginServer hosts the server that completes the browser based login flow.
func serveBrowserLoginServer(l net.Listener, expectedNonce string, destinationURL string, c chan<- string) {
	handler := func(res http.ResponseWriter, req *http.Request) {
		tok := req.URL.Query().Get("accessToken")
		nonce := req.URL.Query().Get("nonce")

		if tok == "" || nonce != expectedNonce {
			res.WriteHeader(400)
			return
		}

		http.Redirect(res, req, destinationURL, http.StatusTemporaryRedirect)
		c <- tok
	}

	mux := &http.ServeMux{}
	mux.HandleFunc("/", handler)
	contract.IgnoreError(http.Serve(l, mux))
}

// CloudConsoleStackPath returns the stack path components for getting to a stack in the cloud console.  This path
// must, of course, be combined with the actual console base URL by way of the CloudConsoleURL function above.
func (b *cloudBackend) cloudConsoleStackPath(stackID client.StackIdentifier) string {
	return path.Join(stackID.Owner, stackID.Project, stackID.Stack)
}

// Logout logs out of the target cloud URL.
func (b *cloudBackend) Logout() error {
	return workspace.DeleteAccessToken(b.CloudURL())
}

// DownloadPlugin downloads a plugin as a tarball from the release endpoint.  The returned reader is a stream
// that reads the tar.gz file, which should be expanded and closed after the download completes.  If progress
// is true, the download will display a progress bar using stdout.
func (b *cloudBackend) DownloadPlugin(ctx context.Context, info workspace.PluginInfo,
	progress bool, opts display.Options) (io.ReadCloser, error) {

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
	result, size, err := b.client.DownloadPlugin(ctx, info, os, arch)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to download plugin")
	}

	// If progress is requested, and we know the length, show a little animated ASCII progress bar.
	if progress && size != -1 {
		bar := pb.New(int(size))
		result = newBarProxyReadCloser(bar, result)
		bar.Prefix(opts.Color.Colorize(colors.SpecUnimportant + "Downloading plugin: "))
		bar.Postfix(opts.Color.Colorize(colors.Reset))
		bar.SetMaxWidth(80)
		bar.SetUnits(pb.U_BYTES)
		bar.Start()
	}

	return result, nil
}

func (b *cloudBackend) GetStack(ctx context.Context, stackRef backend.StackReference) (backend.Stack, error) {
	stackID, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return nil, err
	}

	stack, err := b.client.GetStack(ctx, stackID)
	if err != nil {
		// If this was a 404, return nil, nil as per this method's contract.
		if errResp, ok := err.(*apitype.ErrorResponse); ok && errResp.Code == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}

	return newStack(stack, b), nil
}

func (b *cloudBackend) CreateStack(
	ctx context.Context, stackRef backend.StackReference, _ interface{} /* No custom options for httpstate backend. */) (
	backend.Stack, error) {
	stackID, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return nil, err
	}

	tags, err := backend.GetEnvironmentTagsForCurrentStack()
	if err != nil {
		return nil, errors.Wrap(err, "error determining initial tags")
	}

	apistack, err := b.client.CreateStack(ctx, stackID, tags)
	if err != nil {
		// If the status is 409 Conflict (stack already exists), return StackAlreadyExistsError.
		if errResp, ok := err.(*apitype.ErrorResponse); ok && errResp.Code == http.StatusConflict {
			return nil, &backend.StackAlreadyExistsError{StackName: stackID.Stack}
		}
		return nil, err
	}

	stack := newStack(apistack, b)
	fmt.Printf("Created stack '%s'\n", stack.Ref())

	return stack, nil
}

func (b *cloudBackend) ListStacks(
	ctx context.Context, projectFilter *tokens.PackageName) ([]backend.StackSummary, error) {

	var cleanedProjectName *string
	if projectFilter != nil {
		clean := cleanProjectName(string(*projectFilter))
		cleanedProjectName = &clean
	}

	apiSummaries, err := b.client.ListStacks(ctx, cleanedProjectName)
	if err != nil {
		return nil, err
	}

	// Convert []apitype.StackSummary into []backend.StackSummary.
	var backendSummaries []backend.StackSummary
	for _, apiSummary := range apiSummaries {
		backendSummary := cloudStackSummary{
			summary: apiSummary,
			b:       b,
		}
		backendSummaries = append(backendSummaries, backendSummary)
	}

	return backendSummaries, nil
}

func (b *cloudBackend) RemoveStack(ctx context.Context, stackRef backend.StackReference, force bool) (bool, error) {
	stack, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return false, err
	}

	return b.client.DeleteStack(ctx, stack, force)
}

func (b *cloudBackend) RenameStack(ctx context.Context, stackRef backend.StackReference, newName tokens.QName) error {
	stack, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return err
	}

	return b.client.RenameStack(ctx, stack, string(newName))
}

// cloudCrypter is an encrypter/decrypter that uses the Pulumi cloud to encrypt/decrypt a stack's secrets.
type cloudCrypter struct {
	backend *cloudBackend
	stack   client.StackIdentifier
}

func (c *cloudCrypter) EncryptValue(plaintext string) (string, error) {
	ciphertext, err := c.backend.client.EncryptValue(context.Background(), c.stack, []byte(plaintext))
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
	plaintext, err := c.backend.client.DecryptValue(context.Background(), c.stack, ciphertext)
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

func getStack(ctx context.Context, b *cloudBackend, stackRef backend.StackReference) (backend.Stack, error) {
	stack, err := b.GetStack(ctx, stackRef)
	if err != nil {
		return nil, err
	} else if stack == nil {
		return nil, errors.New("stack not found")
	}

	return stack, nil
}

func (b *cloudBackend) Preview(ctx context.Context, stackRef backend.StackReference,
	op backend.UpdateOperation) (engine.ResourceChanges, result.Result) {
	stack, err := getStack(ctx, b, stackRef)
	if err != nil {
		return nil, result.FromError(err)
	}

	// We can skip PreviewtThenPromptThenExecute, and just go straight to Execute.
	opts := backend.ApplierOptions{
		DryRun:   true,
		ShowLink: true,
	}
	return b.apply(
		ctx, apitype.PreviewUpdate, stack, op, opts, nil /*events*/)
}

func (b *cloudBackend) Update(ctx context.Context, stackRef backend.StackReference,
	op backend.UpdateOperation) (engine.ResourceChanges, result.Result) {
	stack, err := getStack(ctx, b, stackRef)
	if err != nil {
		return nil, result.FromError(err)
	}
	return backend.PreviewThenPromptThenExecute(ctx, apitype.UpdateUpdate, stack, op, b.apply)
}

func (b *cloudBackend) Refresh(ctx context.Context, stackRef backend.StackReference,
	op backend.UpdateOperation) (engine.ResourceChanges, result.Result) {
	stack, err := getStack(ctx, b, stackRef)
	if err != nil {
		return nil, result.FromError(err)
	}
	return backend.PreviewThenPromptThenExecute(ctx, apitype.RefreshUpdate, stack, op, b.apply)
}

func (b *cloudBackend) Destroy(ctx context.Context, stackRef backend.StackReference,
	op backend.UpdateOperation) (engine.ResourceChanges, result.Result) {
	stack, err := getStack(ctx, b, stackRef)
	if err != nil {
		return nil, result.FromError(err)
	}
	return backend.PreviewThenPromptThenExecute(ctx, apitype.DestroyUpdate, stack, op, b.apply)
}

func (b *cloudBackend) createAndStartUpdate(
	ctx context.Context, action apitype.UpdateKind, stack backend.Stack,
	op backend.UpdateOperation, dryRun bool) (client.UpdateIdentifier, int, string, error) {

	stackRef := stack.Ref()

	stackID, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return client.UpdateIdentifier{}, 0, "", err
	}
	stackConfigFile := b.stackConfigFile
	if stackConfigFile == "" {
		f, err := workspace.DetectProjectStackPath(stackRef.Name())
		if err != nil {
			return client.UpdateIdentifier{}, 0, "", err
		}
		stackConfigFile = f
	}
	workspaceStack, err := workspace.LoadProjectStack(stackConfigFile)
	if err != nil {
		return client.UpdateIdentifier{}, 0, "", errors.Wrap(err, "getting configuration")
	}
	metadata := apitype.UpdateMetadata{
		Message:     op.M.Message,
		Environment: op.M.Environment,
	}
	update, err := b.client.CreateUpdate(
		ctx, action, stackID, op.Proj, workspaceStack.Config, metadata, op.Opts.Engine, dryRun)
	if err != nil {
		return client.UpdateIdentifier{}, 0, "", err
	}

	// Start the update. We use this opportunity to pass new tags to the service, to pick up any
	// metadata changes.
	tags, err := backend.GetMergedStackTags(ctx, stack)
	if err != nil {
		return client.UpdateIdentifier{}, 0, "", errors.Wrap(err, "getting stack tags")
	}
	version, token, err := b.client.StartUpdate(ctx, update, tags)
	if err != nil {
		return client.UpdateIdentifier{}, 0, "", err
	}
	// Any non-preview update will be considered part of the stack's update history.
	if action != apitype.PreviewUpdate {
		logging.V(7).Infof("Stack %s being updated to version %d", stackRef, version)
	}

	return update, version, token, nil
}

// apply actually performs the provided type of update on a stack hosted in the Pulumi Cloud.
func (b *cloudBackend) apply(
	ctx context.Context, kind apitype.UpdateKind, stack backend.Stack,
	op backend.UpdateOperation, opts backend.ApplierOptions,
	events chan<- engine.Event) (engine.ResourceChanges, result.Result) {

	// Print a banner so it's clear this is going to the cloud.
	actionLabel := backend.ActionLabel(kind, opts.DryRun)
	fmt.Printf(op.Opts.Display.Color.Colorize(
		colors.SpecHeadline+"%s (%s):"+colors.Reset+"\n"), actionLabel, stack.Ref())

	// Create an update object to persist results.
	update, version, token, err := b.createAndStartUpdate(ctx, kind, stack, op, opts.DryRun)
	if err != nil {
		return nil, result.FromError(err)
	}

	if opts.ShowLink {
		// Print a URL at the end of the update pointing to the Pulumi Service.
		var link string
		base := b.cloudConsoleStackPath(update.StackIdentifier)
		if !opts.DryRun {
			link = b.CloudConsoleURL(base, "updates", strconv.Itoa(version))
		} else {
			link = b.CloudConsoleURL(base, "previews", update.UpdateID)
		}
		if link != "" {
			defer func() {
				fmt.Printf(
					op.Opts.Display.Color.Colorize(
						colors.SpecHeadline+"Permalink: "+
							colors.Underline+colors.BrightBlue+"%s"+colors.Reset+"\n"), link)
			}()
		}
	}

	return b.runEngineAction(ctx, kind, stack.Ref(), op, update, token, events, opts.DryRun)
}

func (b *cloudBackend) runEngineAction(
	ctx context.Context, kind apitype.UpdateKind, stackRef backend.StackReference,
	op backend.UpdateOperation, update client.UpdateIdentifier, token string,
	callerEventsOpt chan<- engine.Event, dryRun bool) (engine.ResourceChanges, result.Result) {

	contract.Assertf(token != "", "persisted actions require a token")
	u, err := b.newUpdate(ctx, stackRef, op.Proj, op.Root, update, token)
	if err != nil {
		return nil, result.FromError(err)
	}

	// displayEvents renders the event to the console and Pulumi service. The processor for the
	// will signal all events have been proceed when a value is written to the displayDone channel.
	displayEvents := make(chan engine.Event)
	displayDone := make(chan bool)
	go u.RecordAndDisplayEvents(
		backend.ActionLabel(kind, dryRun), kind, stackRef, op,
		displayEvents, displayDone, op.Opts.Display, dryRun)

	// The engineEvents channel receives all events from the engine, which we then forward onto other
	// channels for actual processing. (displayEvents and callerEventsOpt.)
	engineEvents := make(chan engine.Event)
	eventsDone := make(chan bool)
	go func() {
		for e := range engineEvents {
			displayEvents <- e
			if callerEventsOpt != nil {
				callerEventsOpt <- e
			}
		}

		close(eventsDone)
	}()

	// The backend.SnapshotManager and backend.SnapshotPersister will keep track of any changes to
	// the Snapshot (checkpoint file) in the HTTP backend.
	persister := b.newSnapshotPersister(ctx, u.update, u.tokenSource)
	snapshotManager := backend.NewSnapshotManager(persister, u.GetTarget().Snapshot)

	// Depending on the action, kick off the relevant engine activity.  Note that we don't immediately check and
	// return error conditions, because we will do so below after waiting for the display channels to close.
	cancellationScope := op.Scopes.NewScope(engineEvents, dryRun)
	engineCtx := &engine.Context{
		Cancel:          cancellationScope.Context(),
		Events:          engineEvents,
		SnapshotManager: snapshotManager,
		BackendClient:   httpstateBackendClient{backend: b},
	}
	if parentSpan := opentracing.SpanFromContext(ctx); parentSpan != nil {
		engineCtx.ParentSpan = parentSpan.Context()
	}

	var changes engine.ResourceChanges
	var res result.Result
	switch kind {
	case apitype.PreviewUpdate:
		changes, res = engine.Update(u, engineCtx, op.Opts.Engine, true)
	case apitype.UpdateUpdate:
		changes, res = engine.Update(u, engineCtx, op.Opts.Engine, dryRun)
	case apitype.RefreshUpdate:
		changes, res = engine.Refresh(u, engineCtx, op.Opts.Engine, dryRun)
	case apitype.DestroyUpdate:
		changes, res = engine.Destroy(u, engineCtx, op.Opts.Engine, dryRun)
	default:
		contract.Failf("Unrecognized update kind: %s", kind)
	}

	// Wait for dependent channels to finish processing engineEvents before closing.
	<-displayDone
	cancellationScope.Close() // Don't take any cancellations anymore, we're shutting down.
	close(engineEvents)
	contract.IgnoreClose(snapshotManager)

	// Make sure that the goroutine writing to displayEvents and callerEventsOpt
	// has exited before proceeding
	<-eventsDone
	close(displayEvents)

	// Mark the update as complete.
	status := apitype.UpdateStatusSucceeded
	if res != nil {
		status = apitype.UpdateStatusFailed
	}
	completeErr := u.Complete(status)
	if completeErr != nil {
		res = result.Merge(res, result.FromError(completeErr))
	}

	return changes, res
}

func (b *cloudBackend) CancelCurrentUpdate(ctx context.Context, stackRef backend.StackReference) error {
	stackID, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return err
	}
	stack, err := b.client.GetStack(ctx, stackID)
	if err != nil {
		return err
	}

	if stack.ActiveUpdate == "" {
		return errors.Errorf("stack %v has never been updated", stackRef)
	}

	// Compute the update identifier and attempt to cancel the update.
	//
	// NOTE: the update kind is not relevant; the same endpoint will work for updates of all kinds.
	updateID := client.UpdateIdentifier{
		StackIdentifier: stackID,
		UpdateKind:      apitype.UpdateUpdate,
		UpdateID:        stack.ActiveUpdate,
	}
	return b.client.CancelUpdate(ctx, updateID)
}

func (b *cloudBackend) GetHistory(ctx context.Context, stackRef backend.StackReference) ([]backend.UpdateInfo, error) {
	stack, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return nil, err
	}

	updates, err := b.client.GetStackUpdates(ctx, stack)
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
			Kind:            update.Kind,
			Message:         update.Message,
			Environment:     update.Environment,
			Config:          cfg,
			Result:          backend.UpdateResult(update.Result),
			StartTime:       update.StartTime,
			EndTime:         update.EndTime,
			ResourceChanges: convertResourceChanges(update.ResourceChanges),
		})
	}

	return beUpdates, nil
}

func (b *cloudBackend) GetLatestConfiguration(ctx context.Context,
	stackRef backend.StackReference) (config.Map, error) {

	stackID, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return nil, err
	}

	return b.client.GetLatestConfiguration(ctx, stackID)
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

func (b *cloudBackend) GetLogs(ctx context.Context, stackRef backend.StackReference,
	logQuery operations.LogQuery) ([]operations.LogEntry, error) {

	stack, err := b.GetStack(ctx, stackRef)
	if err != nil {
		return nil, err
	}
	if stack == nil {
		return nil, errors.New("stack not found")
	}

	target, targetErr := b.getTarget(ctx, stackRef)
	if targetErr != nil {
		return nil, targetErr
	}
	return filestate.GetLogsForTarget(target, logQuery)
}

func (b *cloudBackend) ExportDeployment(ctx context.Context,
	stackRef backend.StackReference) (*apitype.UntypedDeployment, error) {

	stack, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return nil, err
	}

	deployment, err := b.client.ExportStackDeployment(ctx, stack)
	if err != nil {
		return nil, err
	}

	return &deployment, nil
}

func (b *cloudBackend) ImportDeployment(ctx context.Context, stackRef backend.StackReference,
	deployment *apitype.UntypedDeployment) error {

	stack, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return err
	}

	update, err := b.client.ImportStackDeployment(ctx, stack, deployment)
	if err != nil {
		return err
	}

	// Wait for the import to complete, which also polls and renders event output to STDOUT.
	status, err := b.waitForUpdate(
		ctx, backend.ActionLabel(apitype.ImportUpdate, false /*dryRun*/), update,
		display.Options{Color: colors.Always})
	if err != nil {
		return errors.Wrap(err, "waiting for import")
	} else if status != apitype.StatusSucceeded {
		return errors.Errorf("import unsuccessful: status %v", status)
	}
	return nil
}

var (
	projectNameCleanRegexp = regexp.MustCompile("[^a-zA-Z0-9-_.]")
)

// cleanProjectName replaces undesirable characters in project names with hyphens. At some point, these restrictions
// will be further enforced by the service, but for now we need to ensure that if we are making a rest call, we
// do this cleaning on our end.
func cleanProjectName(projectName string) string {
	return projectNameCleanRegexp.ReplaceAllString(projectName, "-")
}

// getCloudStackIdentifier converts a backend.StackReference to a client.StackIdentifier for the same logical stack
func (b *cloudBackend) getCloudStackIdentifier(stackRef backend.StackReference) (client.StackIdentifier, error) {
	cloudBackendStackRef, ok := stackRef.(cloudBackendReference)
	if !ok {
		return client.StackIdentifier{}, errors.New("bad stack reference type")
	}

	return client.StackIdentifier{
		Owner:   cloudBackendStackRef.owner,
		Project: cleanProjectName(cloudBackendStackRef.project),
		Stack:   string(cloudBackendStackRef.name),
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
func (b *cloudBackend) waitForUpdate(ctx context.Context, actionLabel string, update client.UpdateIdentifier,
	displayOpts display.Options) (apitype.UpdateStatus, error) {

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
				return b.tryNextUpdate(ctx, update, continuationToken, try, nextRetryTime)
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

func displayEvents(action string, events <-chan displayEvent, done chan<- bool, opts display.Options) {
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
func (b *cloudBackend) tryNextUpdate(ctx context.Context, update client.UpdateIdentifier, continuationToken *string,
	try int, nextRetryTime time.Duration) (bool, interface{}, error) {

	// If there is no error, we're done.
	results, err := b.client.GetUpdateEvents(ctx, update, continuationToken)
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
			if try < 10 {
				warn = false
			}
			logging.V(3).Infof("Expected %s HTTP %d error after %d retries (retrying): %v",
				b.CloudURL(), errResp.Code, try, err)
		} else {
			// Otherwise, we will issue an error.
			logging.V(3).Infof("Unexpected %s HTTP %d error after %d retries (erroring): %v",
				b.CloudURL(), errResp.Code, try, err)
			return false, nil, err
		}
	} else {
		logging.V(3).Infof("Unexpected %s error after %d retries (retrying): %v", b.CloudURL(), try, err)
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
func IsValidAccessToken(ctx context.Context, cloudURL, accessToken string) (bool, error) {
	// Make a request to get the authenticated user. If it returns a successful response,
	// we know the access token is legit. We also parse the response as JSON and confirm
	// it has a githubLogin field that is non-empty (like the Pulumi Service would return).
	_, err := client.NewClient(cloudURL, accessToken, cmdutil.Diag()).GetPulumiAccountName(ctx)
	if err != nil {
		if errResp, ok := err.(*apitype.ErrorResponse); ok && errResp.Code == 401 {
			return false, nil
		}
		return false, errors.Wrapf(err, "getting user info from %v", cloudURL)
	}

	return true, nil
}

// GetStackTags fetches the stack's existing tags.
func (b *cloudBackend) GetStackTags(ctx context.Context,
	stackRef backend.StackReference) (map[apitype.StackTagName]string, error) {

	stack, err := b.GetStack(ctx, stackRef)
	if err != nil {
		return nil, err
	}
	if stack == nil {
		return nil, errors.New("stack not found")
	}

	return stack.(Stack).Tags(), nil
}

// UpdateStackTags updates the stacks's tags, replacing all existing tags.
func (b *cloudBackend) UpdateStackTags(ctx context.Context,
	stackRef backend.StackReference, tags map[apitype.StackTagName]string) error {

	stack, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return err
	}

	return b.client.UpdateStackTags(ctx, stack, tags)
}

type httpstateBackendClient struct {
	backend Backend
}

func (c httpstateBackendClient) GetStackOutputs(ctx context.Context, name string) (resource.PropertyMap, error) {
	// When using the cloud backend, require that stack references are fully qualified so they
	// look like "<org>/<project>/<stack>"
	if strings.Count(name, "/") != 2 {
		return nil, errors.Errorf("a stack reference's name should be of the form " +
			"'<organization>/<project>/<stack>'. See https://pulumi.io/help/stack-reference for more information.")
	}

	return backend.NewBackendClient(c.backend).GetStackOutputs(ctx, name)
}

func (c httpstateBackendClient) DownloadPlugin(ctx context.Context, plug workspace.PluginInfo) (io.ReadCloser, error) {
	return c.backend.DownloadPlugin(ctx, plug, false, display.Options{})
}
