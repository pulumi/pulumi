// Copyright 2016-2022, Pulumi Corporation.
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
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/browser"

	esc_client "github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/filestate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	sdkDisplay "github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/operations"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/util/validation"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource/config"
	"github.com/pulumi/pulumi/sdk/v3/go/common/slice"
	"github.com/pulumi/pulumi/sdk/v3/go/common/tokens"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/contract"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/logging"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/result"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/retry"
	"github.com/pulumi/pulumi/sdk/v3/go/common/workspace"
)

const (
	// defaultAPIEnvVar can be set to override the default cloud chosen, if `--cloud` is not present.
	defaultURLEnvVar = "PULUMI_API"
	// AccessTokenEnvVar is the environment variable used to bypass a prompt on login.
	AccessTokenEnvVar = "PULUMI_ACCESS_TOKEN"
)

// Name validation rules enforced by the Pulumi Service.
var stackOwnerRegexp = regexp.MustCompile("^[a-zA-Z0-9][a-zA-Z0-9-_]{1,38}[a-zA-Z0-9]$")

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
		return strings.TrimSuffix(cloudURL, "/")
	}

	// Otherwise, respect the PULUMI_API override.
	if cloudURL := os.Getenv(defaultURLEnvVar); cloudURL != "" {
		return cloudURL
	}

	// If that didn't work, see if we have a current cloud, and use that. Note we need to be careful
	// to ignore the local cloud.
	if creds, err := workspace.GetStoredCredentials(); err == nil {
		if creds.Current != "" && !filestate.IsFileStateBackendURL(creds.Current) {
			return creds.Current
		}
	}

	// If none of those led to a cloud URL, simply return the default.
	return PulumiCloudURL
}

// Backend extends the base backend interface with specific information about cloud backends.
type Backend interface {
	backend.Backend

	CloudURL() string

	StackConsoleURL(stackRef backend.StackReference) (string, error)
	Client() *client.Client

	RunDeployment(ctx context.Context, stackRef backend.StackReference, req apitype.CreateDeploymentRequest,
		opts display.Options) error

	// Queries the backend for resources based on the given query parameters.
	Search(
		ctx context.Context, orgName string, queryParams *apitype.PulumiQueryRequest,
	) (*apitype.ResourceSearchResponse, error)
	NaturalLanguageSearch(
		ctx context.Context, orgName string, query string,
	) (*apitype.ResourceSearchResponse, error)
}

type cloudBackend struct {
	d            diag.Sink
	url          string
	client       *client.Client
	escClient    esc_client.Client
	capabilities func(context.Context) capabilities

	// The current project, if any.
	currentProject *workspace.Project
}

// Assert we implement the backend.Backend and backend.SpecificDeploymentExporter interfaces.
var _ backend.SpecificDeploymentExporter = &cloudBackend{}

// New creates a new Pulumi backend for the given cloud API URL and token.
func New(d diag.Sink, cloudURL string, project *workspace.Project, insecure bool) (Backend, error) {
	cloudURL = ValueOrDefaultURL(cloudURL)
	account, err := workspace.GetAccount(cloudURL)
	if err != nil {
		return nil, fmt.Errorf("getting stored credentials: %w", err)
	}
	apiToken := account.AccessToken

	apiClient := client.NewClient(cloudURL, apiToken, insecure, d)
	escClient := esc_client.New(client.UserAgent(), cloudURL, apiToken, insecure)
	capabilities := detectCapabilities(d, apiClient)

	return &cloudBackend{
		d:              d,
		url:            cloudURL,
		client:         apiClient,
		escClient:      escClient,
		capabilities:   capabilities,
		currentProject: project,
	}, nil
}

// loginWithBrowser uses a web-browser to log into the cloud and returns the cloud backend for it.
func loginWithBrowser(
	ctx context.Context,
	cloudURL string,
	insecure bool,
	command string,
	welcome func(display.Options),
	current bool,
	opts display.Options,
) (*workspace.Account, error) {
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
		return nil, fmt.Errorf("could not start listener: %w", err)
	}

	// Extract the port
	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return nil, fmt.Errorf("could not determine port: %w", err)
	}

	// Generate a nonce we'll send with the request.
	nonceBytes := make([]byte, 32)
	_, err = cryptorand.Read(nonceBytes)
	contract.AssertNoErrorf(err, "could not get random bytes")
	nonce := hex.EncodeToString(nonceBytes)

	u, err := url.Parse(loginURL)
	contract.AssertNoErrorf(err, "error parsing login url: %s", loginURL)

	// Generate a description to associate with the access token we'll generate, for display on the Account Settings
	// page.
	var tokenDescription string
	if host, hostErr := os.Hostname(); hostErr == nil {
		tokenDescription = fmt.Sprintf("Generated by %s login on %s at %s", command, host, time.Now().Format(time.RFC822))
	} else {
		tokenDescription = fmt.Sprintf("Generated by %s login at %s", command, time.Now().Format(time.RFC822))
	}

	// Pass our state around as query parameters on the URL we'll open the user's preferred browser to
	q := u.Query()
	q.Add("cliSessionPort", port)
	q.Add("cliSessionNonce", nonce)
	q.Add("cliSessionDescription", tokenDescription)
	if command != "pulumi" {
		q.Add("cliCommand", command)
	}
	u.RawQuery = q.Encode()

	// Start the webserver to listen to handle the response
	go serveBrowserLoginServer(l, nonce, finalWelcomeURL, c)

	// Launch the web browser and navigate to the login URL.
	if openErr := browser.OpenURL(u.String()); openErr != nil {
		fmt.Printf("We couldn't launch your web browser for some reason. Please visit:\n\n%s\n\n"+
			"to finish the login process.", u)
	} else {
		fmt.Println("We've launched your web browser to complete the login process.")
	}

	fmt.Println("\nWaiting for login to complete...")

	accessToken := <-c

	username, organizations, tokenInfo, err := client.NewClient(
		cloudURL, accessToken, insecure, cmdutil.Diag()).GetPulumiAccountDetails(ctx)
	if err != nil {
		return nil, err
	}

	// Save the token and return the backend
	account := workspace.Account{
		AccessToken:      accessToken,
		Username:         username,
		Organizations:    organizations,
		LastValidatedAt:  time.Now(),
		Insecure:         insecure,
		TokenInformation: tokenInfo,
	}
	if err = workspace.StoreAccount(cloudURL, account, current); err != nil {
		return nil, err
	}

	// Welcome the user since this was an interactive login.
	if welcome != nil {
		welcome(opts)
	}

	return &account, nil
}

// LoginManager provides a slim wrapper around functions related to backend logins.
type LoginManager interface {
	// Current returns the current cloud backend if one is already logged in.
	Current(ctx context.Context, cloudURL string, insecure, setCurrent bool) (*workspace.Account, error)

	// Login logs into the target cloud URL and returns the cloud backend for it.
	Login(
		ctx context.Context,
		cloudURL string,
		insecure bool,
		command string,
		message string,
		welcome func(display.Options),
		current bool,
		opts display.Options,
	) (*workspace.Account, error)
}

// NewLoginManager returns a LoginManager for handling backend logins.
func NewLoginManager() LoginManager {
	return newLoginManager()
}

// newLoginManager creates a new LoginManager for handling logins. It is a variable instead of a regular
// function so it can be set to a different implementation at runtime, if necessary.
var newLoginManager = func() LoginManager {
	return defaultLoginManager{}
}

type defaultLoginManager struct{}

// Current returns the current cloud backend if one is already logged in.
func (m defaultLoginManager) Current(
	ctx context.Context,
	cloudURL string,
	insecure bool,
	setCurrent bool,
) (*workspace.Account, error) {
	cloudURL = ValueOrDefaultURL(cloudURL)

	// If we have a saved access token, and it is valid, use it.
	existingAccount, err := workspace.GetAccount(cloudURL)
	if err == nil && existingAccount.AccessToken != "" {
		// If the account was last verified less than an hour ago, assume the token is valid.
		valid := true
		username := existingAccount.Username
		organizations := existingAccount.Organizations
		tokenInfo := existingAccount.TokenInformation
		if username == "" || existingAccount.LastValidatedAt.Add(1*time.Hour).Before(time.Now()) {
			valid, username, organizations, tokenInfo, err = IsValidAccessToken(
				ctx, cloudURL, insecure, existingAccount.AccessToken)
			if err != nil {
				return nil, err
			}
			existingAccount.LastValidatedAt = time.Now()
		}

		if valid {
			// Save the token. While it hasn't changed this will update the current cloud we are logged into, as well.
			existingAccount.Username = username
			existingAccount.Organizations = organizations
			existingAccount.TokenInformation = tokenInfo
			existingAccount.Insecure = insecure
			if err = workspace.StoreAccount(cloudURL, existingAccount, setCurrent); err != nil {
				return nil, err
			}

			return &existingAccount, nil
		}
	}

	// We intentionally don't accept command-line args for the user's access token. Having it in
	// .bash_history is not great, and specifying it via flag isn't of much use.
	accessToken := os.Getenv(AccessTokenEnvVar)

	if accessToken == "" {
		// No access token available, this isn't an error per-se but we don't have a backend
		return nil, nil
	}

	// If there's already a token from the environment, use it.
	_, err = fmt.Fprintf(os.Stderr, "Logging in using access token from %s\n", AccessTokenEnvVar)
	contract.IgnoreError(err)

	// Try and use the credentials to see if they are valid.
	valid, username, organizations, tokenInfo, err := IsValidAccessToken(ctx, cloudURL, insecure, accessToken)
	if err != nil {
		return nil, err
	} else if !valid {
		return nil, fmt.Errorf("invalid access token")
	}

	// Save them.
	account := workspace.Account{
		AccessToken:      accessToken,
		Username:         username,
		Organizations:    organizations,
		TokenInformation: tokenInfo,
		LastValidatedAt:  time.Now(),
		Insecure:         insecure,
	}
	if err = workspace.StoreAccount(cloudURL, account, setCurrent); err != nil {
		return nil, err
	}

	return &account, nil
}

// Login logs into the target cloud URL and returns the cloud backend for it.
func (m defaultLoginManager) Login(
	ctx context.Context,
	cloudURL string,
	insecure bool,
	command string,
	message string,
	welcome func(display.Options),
	setCurrent bool,
	opts display.Options,
) (*workspace.Account, error) {
	current, err := m.Current(ctx, cloudURL, insecure, setCurrent)
	if err != nil {
		return nil, err
	}
	if current != nil {
		return current, nil
	}

	cloudURL = ValueOrDefaultURL(cloudURL)
	var accessToken string
	accountLink := cloudConsoleURL(cloudURL, "account", "tokens")

	if !cmdutil.Interactive() {
		// If interactive mode isn't enabled, the only way to specify a token is through the environment variable.
		// Fail the attempt to login.
		return nil, fmt.Errorf("%s must be set for login during non-interactive CLI sessions", AccessTokenEnvVar)
	}

	// If no access token is available from the environment, and we are interactive, prompt and offer to
	// open a browser to make it easy to generate and use a fresh token.
	line1 := "Manage your " + message + " by logging in."
	line1len := len(line1)
	line1 = colors.Highlight(line1, message, colors.Underline+colors.Bold)
	fmt.Printf(opts.Color.Colorize(line1) + "\n")
	maxlen := line1len

	line2 := fmt.Sprintf("Run `%s --help` for alternative login options.", command)
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
			return loginWithBrowser(ctx, cloudURL, insecure, command, welcome, setCurrent, opts)
		}

		// Welcome the user since this was an interactive login.
		if welcome != nil {
			welcome(opts)
		}
	}

	// Try and use the credentials to see if they are valid.
	valid, username, organizations, tokenInfo, err := IsValidAccessToken(ctx, cloudURL, insecure, accessToken)
	if err != nil {
		return nil, err
	} else if !valid {
		return nil, fmt.Errorf("invalid access token")
	}

	// Save them.
	account := workspace.Account{
		AccessToken:      accessToken,
		Username:         username,
		Organizations:    organizations,
		TokenInformation: tokenInfo,
		LastValidatedAt:  time.Now(),
		Insecure:         insecure,
	}
	if err = workspace.StoreAccount(cloudURL, account, setCurrent); err != nil {
		return nil, err
	}

	return &account, nil
}

// WelcomeUser prints a Welcome to Pulumi message.
func WelcomeUser(opts display.Options) {
	fmt.Printf(`

  %s

  Pulumi helps you create, deploy, and manage infrastructure on any cloud using
  your favorite language. You can get started today with Pulumi at:

      https://www.pulumi.com/docs/get-started/

  %s Resources you create with Pulumi are given unique names (a randomly
  generated suffix) by default. To learn more about auto-naming or customizing resource
  names see https://www.pulumi.com/docs/intro/concepts/resources/#autonaming.


`,
		opts.Color.Colorize(colors.SpecHeadline+"Welcome to Pulumi!"+colors.Reset),
		opts.Color.Colorize(colors.SpecSubHeadline+"Tip:"+colors.Reset))
}

func (b *cloudBackend) StackConsoleURL(stackRef backend.StackReference) (string, error) {
	stackID, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return "", err
	}

	path := b.cloudConsoleStackPath(stackID)

	url := b.CloudConsoleURL(path)
	if url == "" {
		return "", errors.New("could not determine cloud console URL")
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
	user, _, _, err := b.CurrentUser()
	if err != nil {
		return cloudConsoleURL(b.url)
	}
	return cloudConsoleURL(b.url, user)
}

func (b *cloudBackend) SetCurrentProject(project *workspace.Project) {
	b.currentProject = project
}

func (b *cloudBackend) CurrentUser() (string, []string, *workspace.TokenInformation, error) {
	return b.currentUser(context.Background())
}

func (b *cloudBackend) currentUser(ctx context.Context) (string, []string, *workspace.TokenInformation, error) {
	account, err := workspace.GetAccount(b.CloudURL())
	if err != nil {
		return "", nil, nil, err
	}
	if account.Username != "" {
		logging.V(1).Infof("found username for access token")
		return account.Username, account.Organizations, account.TokenInformation, nil
	}
	logging.V(1).Infof("no username for access token")
	return b.client.GetPulumiAccountDetails(ctx)
}

func (b *cloudBackend) CloudURL() string { return b.url }

func (b *cloudBackend) parsePolicyPackReference(s string) (backend.PolicyPackReference, error) {
	split := strings.Split(s, "/")
	var orgName string
	var policyPackName string

	switch len(split) {
	case 2:
		orgName = split[0]
		policyPackName = split[1]
	default:
		return nil, fmt.Errorf("could not parse policy pack name '%s'; must be of the form "+
			"<org-name>/<policy-pack-name>", s)
	}

	if orgName == "" {
		currentUser, _, _, userErr := b.CurrentUser()
		if userErr != nil {
			return nil, userErr
		}
		orgName = currentUser
	}

	return newCloudBackendPolicyPackReference(b.CloudConsoleURL(), orgName, tokens.QName(policyPackName)), nil
}

func (b *cloudBackend) GetPolicyPack(ctx context.Context, policyPack string,
	d diag.Sink,
) (backend.PolicyPack, error) {
	policyPackRef, err := b.parsePolicyPackReference(policyPack)
	if err != nil {
		return nil, err
	}

	return &cloudPolicyPack{
		ref: newCloudBackendPolicyPackReference(b.CloudConsoleURL(),
			policyPackRef.OrgName(), policyPackRef.Name()),
		b:  b,
		cl: b.client,
	}, nil
}

func (b *cloudBackend) ListPolicyGroups(ctx context.Context, orgName string, inContToken backend.ContinuationToken) (
	apitype.ListPolicyGroupsResponse, backend.ContinuationToken, error,
) {
	return b.client.ListPolicyGroups(ctx, orgName, inContToken)
}

func (b *cloudBackend) ListPolicyPacks(ctx context.Context, orgName string, inContToken backend.ContinuationToken) (
	apitype.ListPolicyPacksResponse, backend.ContinuationToken, error,
) {
	return b.client.ListPolicyPacks(ctx, orgName, inContToken)
}

func (b *cloudBackend) SupportsTags() bool {
	return true
}

func (b *cloudBackend) SupportsOrganizations() bool {
	return true
}

func (b *cloudBackend) SupportsProgress() bool {
	return true
}

// qualifiedStackReference describes a qualified stack on the Pulumi Service. The Owner or Project
// may be "" if unspecified, e.g. "pulumi/production" specifies the Owner and Name, but not the
// Project. We infer the missing data and try to make things work as best we can in ParseStackReference.
type qualifiedStackReference struct {
	Owner   string
	Project string
	Name    string
}

// parseStackName parses the stack name into a potentially qualifiedStackReference. Any omitted
// portions will be left as "". For example:
//
// "alpha"            - will just set the Name, but ignore Owner and Project.
// "alpha/beta"       - will set the Owner and Name, but not Project.
// "alpha/beta/gamma" - will set Owner, Name, and Project.
func (b *cloudBackend) parseStackName(s string) (qualifiedStackReference, error) {
	var q qualifiedStackReference

	split := strings.Split(s, "/")
	switch len(split) {
	case 1:
		q.Name = split[0]
	case 2:
		q.Owner = split[0]
		q.Name = split[1]
	case 3:
		q.Owner = split[0]
		q.Project = split[1]
		q.Name = split[2]
	default:
		return qualifiedStackReference{}, fmt.Errorf("could not parse stack name '%s'", s)
	}

	return q, nil
}

func (b *cloudBackend) ParseStackReference(s string) (backend.StackReference, error) {
	// Parse the input as a qualified stack name.
	qualifiedName, err := b.parseStackName(s)
	if err != nil {
		return nil, err
	}

	// If the provided stack name didn't include the Owner or Project, infer them from the
	// local environment.
	if qualifiedName.Owner == "" {
		// if the qualifiedName doesn't include an owner then let's check to see if there is a default org which *will*
		// be the stack owner. If there is no defaultOrg, then we revert to checking the CurrentUser
		defaultOrg, err := workspace.GetBackendConfigDefaultOrg(b.currentProject)
		if err != nil {
			return nil, err
		}

		if defaultOrg != "" {
			qualifiedName.Owner = defaultOrg
		} else {
			currentUser, _, _, userErr := b.CurrentUser()
			if userErr != nil {
				return nil, userErr
			}
			qualifiedName.Owner = currentUser
		}
	}

	if qualifiedName.Project == "" {
		if b.currentProject == nil {
			return nil, fmt.Errorf("If you're using the --stack flag, " +
				"pass the fully qualified name (org/project/stack)")
		}

		qualifiedName.Project = b.currentProject.Name.String()
	}

	if err := validation.ValidateStackName(qualifiedName.Name); err != nil {
		return nil, err
	}
	contract.Assertf(tokens.IsName(qualifiedName.Name),
		"qualifiedName.Name must be a valid name because it is a valid stack name")

	return cloudBackendReference{
		owner:   qualifiedName.Owner,
		project: tokens.Name(qualifiedName.Project),
		name:    tokens.Name(qualifiedName.Name),
		b:       b,
	}, nil
}

func (b *cloudBackend) ValidateStackName(s string) error {
	qualifiedName, err := b.parseStackName(s)
	if err != nil {
		return err
	}

	// The Pulumi Service enforces specific naming restrictions for organizations,
	// projects, and stacks. Though ignore any values that need to be inferred later.
	if qualifiedName.Owner != "" {
		if err := validateOwnerName(qualifiedName.Owner); err != nil {
			return err
		}
	}

	if qualifiedName.Project != "" {
		if err := validateProjectName(qualifiedName.Project); err != nil {
			return err
		}
	}

	return validation.ValidateStackName(qualifiedName.Name)
}

// validateOwnerName checks if a stack owner name is valid. An "owner" is simply the namespace
// a stack may exist within, which for the Pulumi Service is the user account or organization.
func validateOwnerName(s string) error {
	if !stackOwnerRegexp.MatchString(s) {
		return errors.New("invalid stack owner")
	}
	return nil
}

// validateProjectName checks if a project name is valid, returning a user-suitable error if needed.
//
// NOTE: Be careful when requiring a project name be valid. The Pulumi.yaml file may contain
// an invalid project name like "r@bid^W0MBAT!!", but we try to err on the side of flexibility by
// implicitly "cleaning" the project name before we send it to the Pulumi Service. So when we go
// to make HTTP requests, we use a more palitable name like "r_bid_W0MBAT__".
//
// The projects canonical name will be the sanitized "r_bid_W0MBAT__" form, but we do not require the
// Pulumi.yaml file be updated.
//
// So we should only call validateProject name when creating _new_ stacks or creating _new_ projects.
// We should not require that project names be valid when reading what is in the current workspace.
func validateProjectName(s string) error {
	return tokens.ValidateProjectName(s)
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
	contract.IgnoreError(http.Serve(l, mux)) //nolint:gosec
}

// CloudConsoleStackPath returns the stack path components for getting to a stack in the cloud console.  This path
// must, of course, be combined with the actual console base URL by way of the CloudConsoleURL function above.
func (b *cloudBackend) cloudConsoleStackPath(stackID client.StackIdentifier) string {
	return path.Join(stackID.Owner, stackID.Project, stackID.Stack)
}

func inferOrg(ctx context.Context,
	getDefaultOrg func() (string, error),
	getUserOrg func() (string, error),
) (string, error) {
	orgName, err := getDefaultOrg()
	if err != nil || orgName == "" {
		// Fallback to using the current user.
		orgName, err = getUserOrg()
		if err != nil || orgName == "" {
			return "", errors.New("could not determine organization name")
		}
	}
	return orgName, nil
}

// DoesProjectExist returns true if a project with the given name exists in this backend, or false otherwise.
func (b *cloudBackend) DoesProjectExist(ctx context.Context, orgName string, projectName string) (bool, error) {
	if orgName != "" {
		return b.client.DoesProjectExist(ctx, orgName, projectName)
	}

	getDefaultOrg := func() (string, error) {
		return workspace.GetBackendConfigDefaultOrg(nil)
	}
	getUserOrg := func() (string, error) {
		orgName, _, _, err := b.currentUser(ctx)
		return orgName, err
	}
	orgName, err := inferOrg(ctx, getDefaultOrg, getUserOrg)
	if err != nil {
		return false, err
	}

	return b.client.DoesProjectExist(ctx, orgName, projectName)
}

func (b *cloudBackend) GetStack(ctx context.Context, stackRef backend.StackReference) (backend.Stack, error) {
	stackID, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return nil, err
	}

	// GetStack is typically the initial call to a series of calls to the backend. Although logically unrelated,
	// this is a good time to start detecting capabilities so that capability request is not on the critical path.
	go b.capabilities(ctx)

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

// Confirm the specified stack's project doesn't contradict the Pulumi.yaml of the current project.
// if the CWD is not in a Pulumi project,
//
//	does not contradict
//
// if the project name in Pulumi.yaml is "foo".
//
//	a stack with a name of foo/bar/foo should not work.
func currentProjectContradictsWorkspace(project *workspace.Project, stack client.StackIdentifier) bool {
	if project == nil {
		return false
	}

	return project.Name.String() != stack.Project
}

func (b *cloudBackend) CreateStack(
	ctx context.Context, stackRef backend.StackReference, root string,
	opts *backend.CreateStackOptions,
) (
	backend.Stack, error,
) {
	if opts == nil {
		opts = &backend.CreateStackOptions{}
	}

	stackID, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return nil, err
	}

	if currentProjectContradictsWorkspace(b.currentProject, stackID) {
		return nil, fmt.Errorf("provided project name %q doesn't match Pulumi.yaml", stackID.Project)
	}

	// TODO: This should load project config and pass it as the last parameter to GetEnvironmentTagsForCurrentStack.
	tags, err := backend.GetEnvironmentTagsForCurrentStack(root, b.currentProject, nil)
	if err != nil {
		return nil, fmt.Errorf("getting stack tags: %w", err)
	}

	apistack, err := b.client.CreateStack(ctx, stackID, tags, opts.Teams)
	if err != nil {
		// Wire through well-known error types.
		if errResp, ok := err.(*apitype.ErrorResponse); ok && errResp.Code == http.StatusConflict {
			// A 409 error response is returned when per-stack organizations are over their limit,
			// so we need to look at the message to differentiate.
			if strings.Contains(errResp.Message, "already exists") {
				return nil, &backend.StackAlreadyExistsError{StackName: stackID.String()}
			}
			if strings.Contains(errResp.Message, "you are using") {
				return nil, &backend.OverStackLimitError{Message: errResp.Message}
			}
		}
		return nil, err
	}

	stack := newStack(apistack, b)
	fmt.Printf("Created stack '%s'\n", stack.Ref())

	return stack, nil
}

func (b *cloudBackend) ListStacks(
	ctx context.Context, filter backend.ListStacksFilter, inContToken backend.ContinuationToken) (
	[]backend.StackSummary, backend.ContinuationToken, error,
) {
	// Sanitize the project name as needed, so when communicating with the Pulumi Service we
	// always use the name the service expects. (So that a similar, but not technically valid
	// name may be put in Pulumi.yaml without causing problems.)
	if filter.Project != nil {
		cleanedProj := cleanProjectName(*filter.Project)
		filter.Project = &cleanedProj
	}

	// Duplicate type to avoid circular dependency.
	clientFilter := client.ListStacksFilter{
		Organization: filter.Organization,
		Project:      filter.Project,
		TagName:      filter.TagName,
		TagValue:     filter.TagValue,
	}

	apiSummaries, outContToken, err := b.client.ListStacks(ctx, clientFilter, inContToken)
	if err != nil {
		return nil, nil, err
	}

	// Convert []apitype.StackSummary into []backend.StackSummary.
	backendSummaries := slice.Prealloc[backend.StackSummary](len(apiSummaries))
	for _, apiSummary := range apiSummaries {
		backendSummary := cloudStackSummary{
			summary: apiSummary,
			b:       b,
		}
		backendSummaries = append(backendSummaries, backendSummary)
	}

	return backendSummaries, outContToken, nil
}

func (b *cloudBackend) RemoveStack(ctx context.Context, stack backend.Stack, force bool) (bool, error) {
	stackID, err := b.getCloudStackIdentifier(stack.Ref())
	if err != nil {
		return false, err
	}

	return b.client.DeleteStack(ctx, stackID, force)
}

func (b *cloudBackend) RenameStack(ctx context.Context, stack backend.Stack,
	newName tokens.QName,
) (backend.StackReference, error) {
	stackID, err := b.getCloudStackIdentifier(stack.Ref())
	if err != nil {
		return nil, err
	}

	// Support a qualified stack name, which would also rename the stack's project too.
	// e.g. if you want to change the project name on the Pulumi Console to reflect a
	// new value in Pulumi.yaml.
	newRef, err := b.ParseStackReference(string(newName))
	if err != nil {
		return nil, err
	}
	newIdentity, err := b.getCloudStackIdentifier(newRef)
	if err != nil {
		return nil, err
	}

	if stackID.Owner != newIdentity.Owner {
		errMsg := fmt.Sprintf(
			"New stack owner, %s, does not match existing owner, %s.\n\n",
			stackID.Owner, newIdentity.Owner)

		// Re-parse the name using the parseStackName function to avoid the logic in ParseStackReference
		// that auto-populates the owner property with the currently logged in account. We actually want to
		// give a different error message if the raw stack name itself didn't include an owner part.
		parsedName, err := b.parseStackName(string(newName))
		contract.IgnoreError(err)
		if parsedName.Owner == "" {
			errMsg += fmt.Sprintf(
				"       Did you forget to include the owner name? If yes, rerun the command as follows:\n\n"+
					"           $ pulumi stack rename %s/%s\n\n",
				stackID.Owner, newName)
		}

		errMsgSuffix := "."
		if consoleURL, err := b.StackConsoleURL(stack.Ref()); err == nil {
			errMsgSuffix = ":\n\n           " + consoleURL + "/settings/options"
		}
		errMsg += "       You cannot transfer stack ownership via a rename. If you wish to transfer ownership\n" +
			"       of a stack to another organization, you can do so in the Pulumi Console by going to the\n" +
			"       \"Settings\" page of the stack and then clicking the \"Transfer Stack\" button"

		return nil, errors.New(errMsg + errMsgSuffix)
	}

	if err = b.client.RenameStack(ctx, stackID, newIdentity); err != nil {
		return nil, err
	}
	return newRef, nil
}

func (b *cloudBackend) Preview(ctx context.Context, stack backend.Stack,
	op backend.UpdateOperation,
) (*deploy.Plan, sdkDisplay.ResourceChanges, result.Result) {
	// We can skip PreviewtThenPromptThenExecute, and just go straight to Execute.
	opts := backend.ApplierOptions{
		DryRun:   true,
		ShowLink: true,
	}
	return b.apply(
		ctx, apitype.PreviewUpdate, stack, op, opts, nil /*events*/)
}

func (b *cloudBackend) Update(ctx context.Context, stack backend.Stack,
	op backend.UpdateOperation,
) (sdkDisplay.ResourceChanges, result.Result) {
	return backend.PreviewThenPromptThenExecute(ctx, apitype.UpdateUpdate, stack, op, b.apply)
}

func (b *cloudBackend) Import(ctx context.Context, stack backend.Stack,
	op backend.UpdateOperation, imports []deploy.Import,
) (sdkDisplay.ResourceChanges, result.Result) {
	op.Imports = imports
	return backend.PreviewThenPromptThenExecute(ctx, apitype.ResourceImportUpdate, stack, op, b.apply)
}

func (b *cloudBackend) Refresh(ctx context.Context, stack backend.Stack,
	op backend.UpdateOperation,
) (sdkDisplay.ResourceChanges, result.Result) {
	return backend.PreviewThenPromptThenExecute(ctx, apitype.RefreshUpdate, stack, op, b.apply)
}

func (b *cloudBackend) Destroy(ctx context.Context, stack backend.Stack,
	op backend.UpdateOperation,
) (sdkDisplay.ResourceChanges, result.Result) {
	return backend.PreviewThenPromptThenExecute(ctx, apitype.DestroyUpdate, stack, op, b.apply)
}

func (b *cloudBackend) Watch(ctx context.Context, stk backend.Stack,
	op backend.UpdateOperation, paths []string,
) result.Result {
	return backend.Watch(ctx, b, stk, op, b.apply, paths)
}

func (b *cloudBackend) Query(ctx context.Context, op backend.QueryOperation) error {
	return b.query(ctx, op, nil /*events*/)
}

func (b *cloudBackend) Search(
	ctx context.Context, orgName string, queryParams *apitype.PulumiQueryRequest,
) (*apitype.ResourceSearchResponse, error) {
	results, err := b.Client().GetSearchQueryResults(ctx, orgName, queryParams, b.CloudConsoleURL())
	if err != nil {
		return nil, err
	}
	results.Query = queryParams.Query
	return results, nil
}

func (b *cloudBackend) NaturalLanguageSearch(
	ctx context.Context, orgName string, queryString string,
) (*apitype.ResourceSearchResponse, error) {
	parsedResults, err := b.Client().GetNaturalLanguageQueryResults(ctx, orgName, queryString)
	if err != nil {
		return nil, err
	}
	requestBody := apitype.PulumiQueryRequest{Query: parsedResults.Query}
	results, err := b.Client().GetSearchQueryResults(ctx, orgName, &requestBody, b.CloudConsoleURL())
	results.Query = parsedResults.Query
	if err != nil {
		return nil, err
	}
	return results, err
}

type updateMetadata struct {
	version    int
	leaseToken string
	messages   []apitype.Message
}

func (b *cloudBackend) createAndStartUpdate(
	ctx context.Context, action apitype.UpdateKind, stack backend.Stack,
	op *backend.UpdateOperation, dryRun bool,
) (client.UpdateIdentifier, updateMetadata, error) {
	stackRef := stack.Ref()

	stackID, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return client.UpdateIdentifier{}, updateMetadata{}, err
	}
	if currentProjectContradictsWorkspace(op.Proj, stackID) {
		return client.UpdateIdentifier{}, updateMetadata{}, fmt.Errorf(
			"provided project name %q doesn't match Pulumi.yaml", stackID.Project)
	}
	metadata := apitype.UpdateMetadata{
		Message:     op.M.Message,
		Environment: op.M.Environment,
	}
	update, updateDetails, err := b.client.CreateUpdate(
		ctx, action, stackID, op.Proj, op.StackConfiguration.Config, metadata, op.Opts.Engine, dryRun)
	if err != nil {
		return client.UpdateIdentifier{}, updateMetadata{}, err
	}

	//
	// TODO[pulumi-service#3745]: Move this to the plugin-gathering routine when we have a dedicated
	// service API when for getting a list of the required policies to run.
	//
	// For now, this list is given to us when we start an update; yet, the list of analyzers to boot
	// is given to us by CLI flag, and passed to the step generator (which lazily instantiates the
	// plugins) via `op.Opts.Engine.Analyzers`. Since the "start update" API request is sent well
	// after this field is populated, we instead populate the `RequiredPlugins` field here.
	//
	// Once this API is implemented, we can safely move these lines to the plugin-gathering code,
	// which is much closer to being the "correct" place for this stuff.
	//
	for _, policy := range updateDetails.RequiredPolicies {
		op.Opts.Engine.RequiredPolicies = append(
			op.Opts.Engine.RequiredPolicies, newCloudRequiredPolicy(b.client, policy, update.Owner))
	}

	// Start the update. We use this opportunity to pass new tags to the service, to pick up any
	// metadata changes.
	tags, err := backend.GetMergedStackTags(ctx, stack, op.Root, op.Proj, op.StackConfiguration.Config)
	if err != nil {
		return client.UpdateIdentifier{}, updateMetadata{}, fmt.Errorf("getting stack tags: %w", err)
	}

	version, token, err := b.client.StartUpdate(ctx, update, tags)
	if err != nil {
		if err, ok := err.(*apitype.ErrorResponse); ok && err.Code == 409 {
			conflict := backend.ConflictingUpdateError{Err: err}
			return client.UpdateIdentifier{}, updateMetadata{}, conflict
		}
		return client.UpdateIdentifier{}, updateMetadata{}, err
	}
	// Any non-preview update will be considered part of the stack's update history.
	if action != apitype.PreviewUpdate {
		logging.V(7).Infof("Stack %s being updated to version %d", stackRef, version)
	}

	return update, updateMetadata{
		version:    version,
		leaseToken: token,
		messages:   updateDetails.Messages,
	}, nil
}

// apply actually performs the provided type of update on a stack hosted in the Pulumi Cloud.
func (b *cloudBackend) apply(
	ctx context.Context, kind apitype.UpdateKind, stack backend.Stack,
	op backend.UpdateOperation, opts backend.ApplierOptions,
	events chan<- engine.Event,
) (*deploy.Plan, sdkDisplay.ResourceChanges, result.Result) {
	actionLabel := backend.ActionLabel(kind, opts.DryRun)

	if !(op.Opts.Display.JSONDisplay || op.Opts.Display.Type == display.DisplayWatch) {
		// Print a banner so it's clear this is going to the cloud.
		fmt.Printf(op.Opts.Display.Color.Colorize(
			colors.SpecHeadline+"%s (%s)"+colors.Reset+"\n\n"), actionLabel, stack.Ref())
	}

	// Create an update object to persist results.
	update, updateMeta, err := b.createAndStartUpdate(ctx, kind, stack, &op, opts.DryRun)
	if err != nil {
		return nil, nil, result.FromError(err)
	}

	// Display messages from the backend if present.
	if len(updateMeta.messages) > 0 {
		for _, msg := range updateMeta.messages {
			m := diag.RawMessage("", msg.Message)
			switch msg.Severity {
			case apitype.MessageSeverityError:
				cmdutil.Diag().Errorf(m)
			case apitype.MessageSeverityWarning:
				cmdutil.Diag().Warningf(m)
			case apitype.MessageSeverityInfo:
				cmdutil.Diag().Infof(m)
			default:
				// Fallback on Info if we don't recognize the severity.
				cmdutil.Diag().Infof(m)
				logging.V(7).Infof("Unknown message severity: %s", msg.Severity)
			}
		}
		fmt.Print("\n")
	}

	permalink := b.getPermalink(update, updateMeta.version, opts.DryRun)
	return b.runEngineAction(ctx, kind, stack.Ref(), op, update, updateMeta.leaseToken, permalink, events, opts.DryRun)
}

// getPermalink returns a link to the update in the Pulumi Console.
func (b *cloudBackend) getPermalink(update client.UpdateIdentifier, version int, preview bool) string {
	base := b.cloudConsoleStackPath(update.StackIdentifier)
	if !preview {
		return b.CloudConsoleURL(base, "updates", strconv.Itoa(version))
	}
	return b.CloudConsoleURL(base, "previews", update.UpdateID)
}

// query executes a query program against the resource outputs of a stack hosted in the Pulumi
// Cloud.
func (b *cloudBackend) query(ctx context.Context, op backend.QueryOperation,
	callerEventsOpt chan<- engine.Event,
) error {
	return backend.RunQuery(ctx, b, op, callerEventsOpt, b.newQuery)
}

func (b *cloudBackend) runEngineAction(
	ctx context.Context, kind apitype.UpdateKind, stackRef backend.StackReference,
	op backend.UpdateOperation, update client.UpdateIdentifier, token, permalink string,
	callerEventsOpt chan<- engine.Event, dryRun bool,
) (*deploy.Plan, sdkDisplay.ResourceChanges, result.Result) {
	contract.Assertf(token != "", "persisted actions require a token")
	u, err := b.newUpdate(ctx, stackRef, op, update, token)
	if err != nil {
		return nil, nil, result.FromError(err)
	}

	// displayEvents renders the event to the console and Pulumi service. The processor for the
	// will signal all events have been proceed when a value is written to the displayDone channel.
	displayEvents := make(chan engine.Event)
	displayDone := make(chan bool)
	go u.RecordAndDisplayEvents(
		backend.ActionLabel(kind, dryRun), kind, stackRef, op, permalink,
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

	persister := b.newSnapshotPersister(ctx, u.update, u.tokenSource)
	snapshotManager := backend.NewSnapshotManager(persister, op.SecretsManager, u.GetTarget().Snapshot)

	// Depending on the action, kick off the relevant engine activity.  Note that we don't immediately check and
	// return error conditions, because we will do so below after waiting for the display channels to close.
	cancellationScope := op.Scopes.NewScope(engineEvents, dryRun)
	engineCtx := &engine.Context{
		Cancel:          cancellationScope.Context(),
		Events:          engineEvents,
		SnapshotManager: snapshotManager,
		BackendClient:   httpstateBackendClient{backend: backend.NewBackendClient(b, op.SecretsProvider)},
	}
	if parentSpan := opentracing.SpanFromContext(ctx); parentSpan != nil {
		engineCtx.ParentSpan = parentSpan.Context()
	}

	var plan *deploy.Plan
	var changes sdkDisplay.ResourceChanges
	var updateErr error
	switch kind {
	case apitype.PreviewUpdate:
		plan, changes, updateErr = engine.Update(u, engineCtx, op.Opts.Engine, true)
	case apitype.UpdateUpdate:
		plan, changes, updateErr = engine.Update(u, engineCtx, op.Opts.Engine, dryRun)
	case apitype.ResourceImportUpdate:
		_, changes, updateErr = engine.Import(u, engineCtx, op.Opts.Engine, op.Imports, dryRun)
	case apitype.RefreshUpdate:
		_, changes, updateErr = engine.Refresh(u, engineCtx, op.Opts.Engine, dryRun)
	case apitype.DestroyUpdate:
		_, changes, updateErr = engine.Destroy(u, engineCtx, op.Opts.Engine, dryRun)
	default:
		contract.Failf("Unrecognized update kind: %s", kind)
	}
	res := result.WrapIfNonNil(updateErr)

	// Wait for dependent channels to finish processing engineEvents before closing.
	<-displayDone
	cancellationScope.Close() // Don't take any cancellations anymore, we're shutting down.
	close(engineEvents)
	err = snapshotManager.Close()
	// Historically we ignored this error (using IgnoreClose so it would log to the V11 log).
	// To minimize the immediate blast radius of this to start with we're just going to write an error to the user.
	if err != nil {
		cmdutil.Diag().Errorf(diag.Message("", "Snapshot write failed: %v"), err)
	}

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
		res = result.Merge(res, result.FromError(fmt.Errorf("failed to complete update: %w", completeErr)))
	}

	return plan, changes, res
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
		return fmt.Errorf("stack %v has never been updated", stackRef)
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

func (b *cloudBackend) GetHistory(
	ctx context.Context,
	stackRef backend.StackReference,
	pageSize int,
	page int,
) ([]backend.UpdateInfo, error) {
	stack, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return nil, err
	}

	updates, err := b.client.GetStackUpdates(ctx, stack, pageSize, page)
	if err != nil {
		return nil, fmt.Errorf("failed to get stack updates: %w", err)
	}

	// Convert apitype.UpdateInfo objects to the backend type.
	beUpdates := slice.Prealloc[backend.UpdateInfo](len(updates))
	for _, update := range updates {
		// Convert types from the apitype package into their internal counterparts.
		cfg, err := convertConfig(update.Config)
		if err != nil {
			return nil, fmt.Errorf("converting configuration: %w", err)
		}

		beUpdates = append(beUpdates, backend.UpdateInfo{
			Version:         update.Version,
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
	stack backend.Stack,
) (config.Map, error) {
	stackID, err := b.getCloudStackIdentifier(stack.Ref())
	if err != nil {
		return nil, err
	}

	cfg, err := b.client.GetLatestConfiguration(ctx, stackID)
	switch {
	case err == client.ErrNoPreviousDeployment:
		return nil, backend.ErrNoPreviousDeployment
	case err != nil:
		return nil, err
	default:
		return cfg, nil
	}
}

// convertResourceChanges converts the apitype version of sdkDisplay.ResourceChanges into the internal version.
func convertResourceChanges(changes map[apitype.OpType]int) sdkDisplay.ResourceChanges {
	b := make(sdkDisplay.ResourceChanges)
	for k, v := range changes {
		b[sdkDisplay.StepOp(k)] = v
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
		if rawV.Object {
			if rawV.Secret {
				c[k] = config.NewSecureObjectValue(rawV.String)
			} else {
				c[k] = config.NewObjectValue(rawV.String)
			}
		} else {
			if rawV.Secret {
				c[k] = config.NewSecureValue(rawV.String)
			} else {
				c[k] = config.NewValue(rawV.String)
			}
		}
	}
	return c, nil
}

func (b *cloudBackend) GetLogs(ctx context.Context,
	secretsProvider secrets.Provider, stack backend.Stack, cfg backend.StackConfiguration,
	logQuery operations.LogQuery,
) ([]operations.LogEntry, error) {
	target, targetErr := b.getTarget(ctx, secretsProvider, stack.Ref(), cfg.Config, cfg.Decrypter)
	if targetErr != nil {
		return nil, targetErr
	}
	return filestate.GetLogsForTarget(target, logQuery)
}

// ExportDeployment exports a deployment _from_ the backend service.
// This will return the stack state that was being stored on the backend service.
func (b *cloudBackend) ExportDeployment(ctx context.Context,
	stack backend.Stack,
) (*apitype.UntypedDeployment, error) {
	return b.exportDeployment(ctx, stack.Ref(), nil /* latest */)
}

func (b *cloudBackend) ExportDeploymentForVersion(
	ctx context.Context, stack backend.Stack, version string,
) (*apitype.UntypedDeployment, error) {
	// The Pulumi Console defines versions as a positive integer. Parse the provided version string and
	// ensure it is valid.
	//
	// The first stack update version is 1, and monotonically increasing from there.
	versionNumber, err := strconv.Atoi(version)
	if err != nil || versionNumber <= 0 {
		return nil, fmt.Errorf(
			"%q is not a valid stack version. It should be a positive integer",
			version)
	}

	return b.exportDeployment(ctx, stack.Ref(), &versionNumber)
}

// exportDeployment exports the checkpoint file for a stack, optionally getting a previous version.
func (b *cloudBackend) exportDeployment(
	ctx context.Context, stackRef backend.StackReference, version *int,
) (*apitype.UntypedDeployment, error) {
	stack, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return nil, err
	}

	deployment, err := b.client.ExportStackDeployment(ctx, stack, version)
	if err != nil {
		return nil, err
	}

	return &deployment, nil
}

// ImportDeployment imports a deployment _into_ the backend. At the end of this operation,
// the deployment provided will be the current state stored on the backend service.
func (b *cloudBackend) ImportDeployment(ctx context.Context, stack backend.Stack,
	deployment *apitype.UntypedDeployment,
) error {
	stackID, err := b.getCloudStackIdentifier(stack.Ref())
	if err != nil {
		return err
	}

	update, err := b.client.ImportStackDeployment(ctx, stackID, deployment)
	if err != nil {
		return err
	}

	// Wait for the import to complete, which also polls and renders event output to STDOUT.
	status, err := b.waitForUpdate(
		ctx, backend.ActionLabel(apitype.StackImportUpdate, false /*dryRun*/), update,
		display.Options{Color: colors.Always})
	if err != nil {
		return fmt.Errorf("waiting for import: %w", err)
	} else if status != apitype.StatusSucceeded {
		return fmt.Errorf("import unsuccessful: status %v", status)
	}
	return nil
}

var projectNameCleanRegexp = regexp.MustCompile("[^a-zA-Z0-9-_.]")

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
		Project: cleanProjectName(string(cloudBackendStackRef.project)),
		Stack:   string(cloudBackendStackRef.name),
	}, nil
}

// Client returns a client object that may be used to interact with this backend.
func (b *cloudBackend) Client() *client.Client {
	return b.client
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
	displayOpts display.Options,
) (apitype.UpdateStatus, error) {
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
	spinner, ticker := cmdutil.NewSpinnerAndTicker(prefix, nil, opts.Color, 8 /*timesPerSecond*/)

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
	try int, nextRetryTime time.Duration,
) (bool, interface{}, error) {
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
func IsValidAccessToken(ctx context.Context, cloudURL string,
	insecure bool, accessToken string,
) (bool, string, []string, *workspace.TokenInformation, error) {
	// Make a request to get the authenticated user. If it returns a successful response,
	// we know the access token is legit. We also parse the response as JSON and confirm
	// it has a githubLogin field that is non-empty (like the Pulumi Service would return).
	username, organizations, tokenInfo, err := client.NewClient(cloudURL, accessToken,
		insecure, cmdutil.Diag()).GetPulumiAccountDetails(ctx)
	if err != nil {
		if errResp, ok := err.(*apitype.ErrorResponse); ok && errResp.Code == 401 {
			return false, "", nil, nil, nil
		}
		return false, "", nil, nil, fmt.Errorf("getting user info from %v: %w", cloudURL, err)
	}

	return true, username, organizations, tokenInfo, nil
}

// UpdateStackTags updates the stacks's tags, replacing all existing tags.
func (b *cloudBackend) UpdateStackTags(ctx context.Context,
	stack backend.Stack, tags map[apitype.StackTagName]string,
) error {
	stackID, err := b.getCloudStackIdentifier(stack.Ref())
	if err != nil {
		return err
	}

	return b.client.UpdateStackTags(ctx, stackID, tags)
}

const pulumiOperationHeader = "Pulumi operation"

func (b *cloudBackend) RunDeployment(ctx context.Context, stackRef backend.StackReference,
	req apitype.CreateDeploymentRequest, opts display.Options,
) error {
	stackID, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return err
	}

	resp, err := b.client.CreateDeployment(ctx, stackID, req)
	if err != nil {
		return err
	}
	id := resp.ID

	fmt.Print(opts.Color.Colorize(colors.SpecHeadline + "Preparing deployment..." + colors.Reset + "\n\n"))

	if !opts.SuppressPermalink && !opts.JSONDisplay && resp.ConsoleURL != "" {
		fmt.Printf(opts.Color.Colorize(
			colors.SpecHeadline+"View Live: "+
				colors.Underline+colors.BrightBlue+"%s"+colors.Reset+"\n"), resp.ConsoleURL)
	}

	token := ""
	for {
		logs, err := b.client.GetDeploymentLogs(ctx, stackID, id, token)
		if err != nil {
			return err
		}

		for _, l := range logs.Lines {
			if l.Header != "" {
				fmt.Print(opts.Color.Colorize(
					"\n" + colors.SpecHeadline + l.Header + ":" + colors.Reset + "\n"))

				// If we see it's a Pulumi operation, rather than outputting the deployment logs,
				// find the associated update and show the normal rendering of the operation's events.
				if l.Header == pulumiOperationHeader {
					fmt.Println()
					return b.showDeploymentEvents(ctx, stackID, apitype.UpdateKind(req.Operation.Operation), id, opts)
				}
			} else {
				fmt.Print(l.Line)
			}
		}

		// If there are no more logs for the deployment and the deployment has finished or we're not following,
		// then we're done.
		if logs.NextToken == "" {
			break
		}

		// Otherwise, update the token, sleep, and loop around.
		if logs.NextToken == token {
			time.Sleep(500 * time.Millisecond)
		}
		token = logs.NextToken
	}

	return nil
}

func (b *cloudBackend) showDeploymentEvents(ctx context.Context, stackID client.StackIdentifier,
	kind apitype.UpdateKind, deploymentID string, opts display.Options,
) error {
	getUpdateID := func() (string, int, error) {
		for tries := 0; tries < 10; tries++ {
			updates, err := b.client.GetDeploymentUpdates(ctx, stackID, deploymentID)
			if err != nil {
				return "", 0, err
			}
			if len(updates) > 0 {
				return updates[0].UpdateID, updates[0].Version, nil
			}

			time.Sleep(500 * time.Millisecond)
		}
		return "", 0, fmt.Errorf("could not find update associated with deployment %s", deploymentID)
	}

	updateID, version, err := getUpdateID()
	if err != nil {
		return err
	}

	dryRun := kind == apitype.PreviewUpdate
	update := client.UpdateIdentifier{
		StackIdentifier: stackID,
		UpdateKind:      kind,
		UpdateID:        updateID,
	}

	events := make(chan engine.Event) // Note: unbuffered, but we assume it won't matter in practice.
	done := make(chan bool)

	// Timings do not display correctly when rendering remote events, so suppress showing them.
	opts.SuppressTimings = true

	permalink := b.getPermalink(update, version, dryRun)
	go display.ShowEvents(
		backend.ActionLabel(kind, dryRun), kind, tokens.Name(stackID.Stack), tokens.PackageName(stackID.Project),
		permalink, events, done, opts, dryRun)

	// The UpdateEvents API returns a continuation token to only get events after the previous call.
	var continuationToken *string
	var lastEvent engine.Event
	for {
		resp, err := b.client.GetUpdateEngineEvents(ctx, update, continuationToken)
		if err != nil {
			return err
		}
		for _, jsonEvent := range resp.Events {
			event, err := display.ConvertJSONEvent(jsonEvent)
			if err != nil {
				return err
			}
			lastEvent = event
			events <- event
		}

		continuationToken = resp.ContinuationToken
		// A nil continuation token means there are no more events to read and the update has finished.
		if continuationToken == nil {
			// If the event stream does not terminate with a cancel event, synthesize one here.
			if lastEvent.Type != engine.CancelEvent {
				events <- engine.NewEvent(engine.CancelEvent, nil)
			}

			close(events)
			<-done
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}
}

type httpstateBackendClient struct {
	backend deploy.BackendClient
}

func (c httpstateBackendClient) GetStackOutputs(ctx context.Context, name string) (resource.PropertyMap, error) {
	// When using the cloud backend, require that stack references are fully qualified so they
	// look like "<org>/<project>/<stack>"
	if strings.Count(name, "/") != 2 {
		return nil, fmt.Errorf("a stack reference's name should be of the form " +
			"'<organization>/<project>/<stack>'. See https://pulumi.io/help/stack-reference for more information.")
	}

	return c.backend.GetStackOutputs(ctx, name)
}

func (c httpstateBackendClient) GetStackResourceOutputs(
	ctx context.Context, name string,
) (resource.PropertyMap, error) {
	return c.backend.GetStackResourceOutputs(ctx, name)
}

// Represents feature-detected capabilities of the service the backend is connected to.
type capabilities struct {
	// If non-nil, indicates that delta checkpoint updates are supported.
	deltaCheckpointUpdates *apitype.DeltaCheckpointUploadsConfigV2
}

// Builds a lazy wrapper around doDetectCapabilities.
func detectCapabilities(d diag.Sink, client *client.Client) func(ctx context.Context) capabilities {
	var once sync.Once
	var caps capabilities
	done := make(chan struct{})
	get := func(ctx context.Context) capabilities {
		once.Do(func() {
			caps = doDetectCapabilities(ctx, d, client)
			close(done)
		})
		<-done
		return caps
	}
	return get
}

func doDetectCapabilities(ctx context.Context, d diag.Sink, client *client.Client) capabilities {
	resp, err := client.GetCapabilities(ctx)
	if err != nil {
		d.Warningf(diag.Message("" /*urn*/, "failed to get capabilities: %v"), err)
		return capabilities{}
	}
	caps, err := decodeCapabilities(resp.Capabilities)
	if err != nil {
		d.Warningf(diag.Message("" /*urn*/, "failed to decode capabilities: %v"), err)
		return capabilities{}
	}

	// Allow users to opt out of deltaCheckpointUpdates even if the backend indicates it should be used. This
	// remains necessary while PULUMI_OPTIMIZED_CHECKPOINT_PATCH has higher memory requirements on the client and
	// may cause out-of-memory issues in constrained environments.
	switch strings.ToLower(os.Getenv("PULUMI_OPTIMIZED_CHECKPOINT_PATCH")) {
	case "0", "false":
		caps.deltaCheckpointUpdates = nil
	}

	return caps
}

func decodeCapabilities(wireLevel []apitype.APICapabilityConfig) (capabilities, error) {
	var parsed capabilities
	for _, entry := range wireLevel {
		switch entry.Capability {
		case apitype.DeltaCheckpointUploads:
			var upcfg apitype.DeltaCheckpointUploadsConfigV1
			if err := json.Unmarshal(entry.Configuration, &upcfg); err != nil {
				msg := "decoding DeltaCheckpointUploadsConfig returned %w"
				return capabilities{}, fmt.Errorf(msg, err)
			}
			parsed.deltaCheckpointUpdates = &apitype.DeltaCheckpointUploadsConfigV2{
				CheckpointCutoffSizeBytes: upcfg.CheckpointCutoffSizeBytes,
			}
		case apitype.DeltaCheckpointUploadsV2:
			if entry.Version == 2 {
				var upcfg apitype.DeltaCheckpointUploadsConfigV2
				if err := json.Unmarshal(entry.Configuration, &upcfg); err != nil {
					msg := "decoding DeltaCheckpointUploadsConfigV2 returned %w"
					return capabilities{}, fmt.Errorf(msg, err)
				}
				parsed.deltaCheckpointUpdates = &upcfg
			}
		default:
			continue
		}
	}
	return parsed, nil
}
