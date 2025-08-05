// Copyright 2016-2025, Pulumi Corporation.
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
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/pkg/browser"

	esc_client "github.com/pulumi/esc/cmd/esc/cli/client"
	"github.com/pulumi/pulumi/pkg/v3/backend"
	"github.com/pulumi/pulumi/pkg/v3/backend/backenderr"
	"github.com/pulumi/pulumi/pkg/v3/backend/display"
	"github.com/pulumi/pulumi/pkg/v3/backend/diy"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	sdkDisplay "github.com/pulumi/pulumi/pkg/v3/display"
	"github.com/pulumi/pulumi/pkg/v3/engine"
	"github.com/pulumi/pulumi/pkg/v3/operations"
	"github.com/pulumi/pulumi/pkg/v3/resource/deploy"
	"github.com/pulumi/pulumi/pkg/v3/secrets"
	"github.com/pulumi/pulumi/pkg/v3/util/nosleep"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/apitype"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag"
	"github.com/pulumi/pulumi/sdk/v3/go/common/diag/colors"
	"github.com/pulumi/pulumi/sdk/v3/go/common/env"
	"github.com/pulumi/pulumi/sdk/v3/go/common/promise"
	"github.com/pulumi/pulumi/sdk/v3/go/common/registry"
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

type PulumiAILanguage string

const (
	PulumiAILanguageTypeScript PulumiAILanguage = "TypeScript"
	PulumiAILanguageJavaScript PulumiAILanguage = "JavaScript"
	PulumiAILanguagePython     PulumiAILanguage = "Python"
	PulumiAILanguageGo         PulumiAILanguage = "Go"
	PulumiAILanguageCSharp     PulumiAILanguage = "C#"
	PulumiAILanguageJava       PulumiAILanguage = "Java"
	PulumiAILanguageYAML       PulumiAILanguage = "YAML"
)

var pulumiAILanguageMap = map[string]PulumiAILanguage{
	"typescript": PulumiAILanguageTypeScript,
	"javascript": PulumiAILanguageJavaScript,
	"python":     PulumiAILanguagePython,
	"go":         PulumiAILanguageGo,
	"c#":         PulumiAILanguageCSharp,
	"java":       PulumiAILanguageJava,
	"yaml":       PulumiAILanguageYAML,
}

// All of the languages supported by Pulumi AI.
var PulumiAILanguageOptions = []PulumiAILanguage{
	PulumiAILanguageTypeScript,
	PulumiAILanguageJavaScript,
	PulumiAILanguagePython,
	PulumiAILanguageGo,
	PulumiAILanguageCSharp,
	PulumiAILanguageJava,
	PulumiAILanguageYAML,
}

// A natural language list of languages supported by Pulumi AI.
const PulumiAILanguagesClause = "TypeScript, JavaScript, Python, Go, C#, Java, or YAML"

func (e *PulumiAILanguage) String() string {
	return string(*e)
}

func (e *PulumiAILanguage) Set(v string) error {
	value, ok := pulumiAILanguageMap[strings.ToLower(v)]
	if !ok {
		return fmt.Errorf("must be one of %s", PulumiAILanguagesClause)
	}
	*e = value
	return nil
}

func (e *PulumiAILanguage) Type() string {
	return "pulumiAILanguage"
}

type AIPromptRequestBody struct {
	Language       PulumiAILanguage `json:"language"`
	Instructions   string           `json:"instructions"`
	ResponseMode   string           `json:"responseMode"`
	ConversationID string           `json:"conversationId"`
	ConnectionID   string           `json:"connectionId"`
}

// Name validation rules enforced by the Pulumi Service.
var stackOwnerRegexp = regexp.MustCompile("^[a-zA-Z0-9][a-zA-Z0-9-_]{1,38}[a-zA-Z0-9]$")

// DefaultURL returns the default cloud URL.  This may be overridden using the PULUMI_API environment
// variable.  If no override is found, and we are authenticated with a cloud, choose that.  Otherwise,
// we will default to the https://api.pulumi.com/ endpoint.
func DefaultURL(ws pkgWorkspace.Context) string {
	return ValueOrDefaultURL(ws, "")
}

// ValueOrDefaultURL returns the value if specified, or the default cloud URL otherwise.
func ValueOrDefaultURL(ws pkgWorkspace.Context, cloudURL string) string {
	// If we have a cloud URL, just return it.
	if cloudURL != "" {
		return strings.TrimSuffix(cloudURL, "/")
	}

	// Otherwise, respect the PULUMI_API override.

	if cloudURL := env.APIURL.Value(); cloudURL != "" {
		return cloudURL
	}

	// If that didn't work, see if we have a current cloud, and use that. Note we need to be careful
	// to ignore the diy cloud.
	if creds, err := ws.GetStoredCredentials(); err == nil {
		if creds.Current != "" && !diy.IsDIYBackendURL(creds.Current) {
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
		opts display.Options, deploymentInitiator string, streamDeploymentLogs bool) error

	// Queries the backend for resources based on the given query parameters.
	Search(
		ctx context.Context, orgName string, queryParams *apitype.PulumiQueryRequest,
	) (*apitype.ResourceSearchResponse, error)
	NaturalLanguageSearch(
		ctx context.Context, orgName string, query string,
	) (*apitype.ResourceSearchResponse, error)
	PromptAI(ctx context.Context, requestBody AIPromptRequestBody) (*http.Response, error)
	// Capabilities returns the capabilities of the backend indicating what features are available.
	Capabilities(ctx context.Context) apitype.Capabilities
}

type cloudBackend struct {
	d            diag.Sink
	url          string
	client       *client.Client
	escClient    esc_client.Client
	capabilities *promise.Promise[apitype.Capabilities]

	// The current project, if any.
	currentProject                  *workspace.Project
	copilotEnabledForCurrentProject *bool
}

// Assert we implement the backend.Backend and backend.SpecificDeploymentExporter interfaces.
var _ backend.SpecificDeploymentExporter = &cloudBackend{}

// New creates a new Pulumi backend for the given cloud API URL and token.
func New(ctx context.Context, d diag.Sink,
	cloudURL string, project *workspace.Project, insecure bool,
) (Backend, error) {
	cloudURL = ValueOrDefaultURL(pkgWorkspace.Instance, cloudURL)
	account, err := workspace.GetAccount(cloudURL)
	if err != nil {
		return nil, fmt.Errorf("getting stored credentials: %w", err)
	}
	apiToken := account.AccessToken

	apiClient := client.NewClient(cloudURL, apiToken, insecure, d)
	escClient := esc_client.New(client.UserAgent(), cloudURL, apiToken, insecure)

	return &cloudBackend{
		d:              d,
		url:            cloudURL,
		client:         apiClient,
		escClient:      escClient,
		capabilities:   detectCapabilities(d, apiClient),
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
	cloudURL = ValueOrDefaultURL(pkgWorkspace.Instance, cloudURL)

	// We intentionally don't accept command-line args for the user's access token. Having it in
	// .bash_history is not great, and specifying it via flag isn't of much use.
	accessToken := env.AccessToken.Value()

	// If we have a saved access token, and it is valid, and it
	// either matches PULUMI_ACCESS_TOKEN or PULUMI_ACCESS_TOKEN
	// is not set use it.  If PULUMI_ACCESS_TOKEN does not match,
	// we prefer that.
	existingAccount, err := workspace.GetAccount(cloudURL)
	if err == nil && existingAccount.AccessToken != "" &&
		(accessToken == "" || existingAccount.AccessToken == accessToken) {
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

	// We either have no token saved, or PULUMI_ACCESS_TOKEN
	// doesn't match what we have saved.  Prefer the new
	// PULUMI_ACCESS_TOKEN.
	if accessToken == "" {
		// No access token available, this isn't an error per-se but we don't have a backend
		return nil, nil
	}

	// If there's already a token from the environment, use it.
	_, err = fmt.Fprintf(os.Stderr, "Logging in using access token from %s\n", env.AccessToken.Var().Name())
	contract.IgnoreError(err)

	// Try and use the credentials to see if they are valid.
	valid, username, organizations, tokenInfo, err := IsValidAccessToken(ctx, cloudURL, insecure, accessToken)
	if err != nil {
		return nil, err
	} else if !valid {
		return nil, errors.New("invalid access token")
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

	cloudURL = ValueOrDefaultURL(pkgWorkspace.Instance, cloudURL)
	var accessToken string
	accountLink := cloudConsoleURL(cloudURL, "account", "tokens")

	if !cmdutil.Interactive() {
		// If interactive mode isn't enabled, the only way to specify a token is through the environment variable.
		// Fail the attempt to login.
		return nil, backenderr.MissingEnvVarForNonInteractiveError{Var: env.AccessToken.Var()}
	}

	// If no access token is available from the environment, and we are interactive, prompt and offer to
	// open a browser to make it easy to generate and use a fresh token.
	line1 := "Manage your " + message + " by logging in."
	line1len := len(line1)
	line1 = colors.Highlight(line1, message, colors.Underline+colors.Bold)
	fmt.Println(opts.Color.Colorize(line1))
	maxlen := line1len

	line2 := fmt.Sprintf("Run `%s login --help` for alternative login options.", command)
	line2len := len(line2)
	fmt.Println(opts.Color.Colorize(line2))
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
		line3 := "Enter your access token from " + accountLink
		line3len := len(line3)
		line3 = colors.Highlight(line3, "access token", colors.BrightCyan+colors.Bold)
		line3 = colors.Highlight(line3, accountLink, colors.BrightBlue+colors.Underline+colors.Bold)
		fmt.Println(opts.Color.Colorize(line3))
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
		return nil, errors.New("invalid access token")
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
	b.copilotEnabledForCurrentProject = nil
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

func (b *cloudBackend) ListTemplates(ctx context.Context, orgName string) (apitype.ListOrgTemplatesResponse, error) {
	return b.client.ListOrgTemplates(ctx, orgName)
}

func (b *cloudBackend) DownloadTemplate(
	ctx context.Context, orgName, sourceURL string,
) (backend.TarReaderCloser, error) {
	t, err := b.client.DownloadOrgTemplate(ctx, orgName, sourceURL)
	if err != nil {
		return nil, err
	}
	return t, nil
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

func (b *cloudBackend) SupportsDeployments() bool {
	return true
}

func (b *cloudBackend) SupportsTemplates() bool {
	return true
}

func (b *cloudBackend) Capabilities(ctx context.Context) apitype.Capabilities {
	capabilities, err := b.capabilities.Result(ctx)
	if err != nil {
		return apitype.Capabilities{}
	}
	return capabilities
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

	defaultOrg, err := backend.GetDefaultOrg(context.TODO(), b, b.currentProject)
	if err != nil {
		return nil, err
	}

	// If the provided stack name didn't include the Owner or Project, infer them from the
	// local environment.
	if qualifiedName.Owner == "" {
		// if the qualifiedName doesn't include an owner then let's check to see if there is a default org which *will*
		// be the stack owner. If there is no defaultOrg, then we revert to checking the CurrentUser
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
			return nil, errors.New("no current project found, pass the fully qualified stack name (org/project/stack)")
		}

		qualifiedName.Project = b.currentProject.Name.String()
	}

	parsedName, err := tokens.ParseStackName(qualifiedName.Name)
	if err != nil {
		return nil, err
	}

	return cloudBackendReference{
		owner:      qualifiedName.Owner,
		defaultOrg: defaultOrg,
		project:    tokens.Name(qualifiedName.Project),
		name:       parsedName,
		b:          b,
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

	_, err = tokens.ParseStackName(qualifiedName.Name)
	return err
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
	return path.Join(stackID.Owner, stackID.Project, stackID.Stack.String())
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
		return backend.GetDefaultOrg(ctx, b, nil)
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

	stack, err := b.client.GetStack(ctx, stackID)
	if err != nil {
		// If this was a 404, return nil, nil as per this method's contract.
		if errResp, ok := err.(*apitype.ErrorResponse); ok && errResp.Code == http.StatusNotFound {
			return nil, nil
		}
		return nil, err
	}

	return newStack(ctx, stack, b)
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
	ctx context.Context,
	stackRef backend.StackReference,
	root string,
	initialState *apitype.UntypedDeployment,
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

	b.downgradeUntypedDeploymentVersionIfNeeded(ctx, initialState)

	apistack, err := b.client.CreateStack(ctx, stackID, tags, opts.Teams, initialState, opts.Config)
	if err != nil {
		// Wire through well-known error types.
		if errResp, ok := err.(*apitype.ErrorResponse); ok && errResp.Code == http.StatusConflict {
			// A 409 error response is returned when per-stack organizations are over their limit,
			// so we need to look at the message to differentiate.
			if strings.Contains(errResp.Message, "already exists") {
				return nil, &backenderr.StackAlreadyExistsError{StackName: stackID.String()}
			}
			if strings.Contains(errResp.Message, "you are using") {
				return nil, &backenderr.OverStackLimitError{Message: errResp.Message}
			}
		}
		return nil, err
	}

	stack, err := newStack(ctx, apistack, b)
	if err != nil {
		fmt.Printf("Created stack '%s'\n", stack.Ref())
	}

	return stack, err
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

	// Look up the default organization and persist it across each stack summary, in order to reduce
	// the number of lookups each stack summary would otherwise have to make to determine whether to
	// elide the organization name.
	// Since ListStacks is also a potentially long-running operation for power users with many stacks,
	// this has the added benefit of ensuring that the default org is consistent for the duration of the
	// operation, even if the user changes their default org mid-process.
	defaultOrg, err := backend.GetDefaultOrg(ctx, b, b.currentProject)
	if err != nil {
		return nil, nil, err
	}
	if defaultOrg == "" {
		defaultOrg, _, _, err = b.CurrentUser()
		if err != nil {
			return nil, nil, err
		}
	}

	// Convert []apitype.StackSummary into []backend.StackSummary.
	backendSummaries := slice.Prealloc[backend.StackSummary](len(apiSummaries))
	for _, apiSummary := range apiSummaries {
		backendSummary := cloudStackSummary{
			summary:    apiSummary,
			b:          b,
			defaultOrg: defaultOrg,
		}
		backendSummaries = append(backendSummaries, backendSummary)
	}

	return backendSummaries, outContToken, nil
}

func (b *cloudBackend) ListStackNames(
	ctx context.Context, filter backend.ListStackNamesFilter, inContToken backend.ContinuationToken) (
	[]backend.StackReference, backend.ContinuationToken, error,
) {
	// Convert ListStackNamesFilter to ListStacksFilter (without tag fields)
	stacksFilter := backend.ListStacksFilter{
		Organization: filter.Organization,
		Project:      filter.Project,
	}

	// For the cloud backend, we can reuse ListStacks since the API already returns data efficiently.
	// We just extract the stack references from the summaries.
	summaries, outContToken, err := b.ListStacks(ctx, stacksFilter, inContToken)
	if err != nil {
		return nil, nil, err
	}

	stackRefs := slice.Prealloc[backend.StackReference](len(summaries))
	for _, summary := range summaries {
		stackRefs = append(stackRefs, summary.Name())
	}

	return stackRefs, outContToken, nil
}

func (b *cloudBackend) RemoveStack(ctx context.Context, stack backend.Stack, force, removeBackups bool) (bool, error) {
	stackID, err := b.getCloudStackIdentifier(stack.Ref())
	if err != nil {
		return false, err
	}

	// Note: removeBackups is currently unused in the cloud backend.

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
	op backend.UpdateOperation, events chan<- engine.Event,
) (*deploy.Plan, sdkDisplay.ResourceChanges, error) {
	// We can skip PreviewThenPromptThenExecute, and just go straight to Execute.
	opts := backend.ApplierOptions{
		DryRun:   true,
		ShowLink: true,
	}
	return b.apply(
		ctx, apitype.PreviewUpdate, stack, op, opts, events)
}

func (b *cloudBackend) Update(ctx context.Context, stack backend.Stack,
	op backend.UpdateOperation, events chan<- engine.Event,
) (sdkDisplay.ResourceChanges, error) {
	return backend.PreviewThenPromptThenExecute(ctx, apitype.UpdateUpdate, stack, op, b.apply, b, events)
}

// IsExplainPreviewEnabled implements the "explainer" interface.
// Checks that the backend supports the CopilotExplainPreview capability and that the user has enabled
// the Copilot features.
func (b *cloudBackend) IsExplainPreviewEnabled(ctx context.Context, opts display.Options) bool {
	if !b.isCopilotFeaturesEnabled(opts) {
		return false
	}

	if !b.Capabilities(ctx).CopilotExplainPreviewV1 {
		logging.V(7).Infof("CopilotExplainPreviewV1 is not supported by the backend")
		return false
	}

	return true
}

func (b *cloudBackend) isCopilotFeaturesEnabled(opts display.Options) bool {
	// Have copilot features been requested by specifying the --copilot flag to the cli
	if !opts.ShowCopilotFeatures {
		return false
	}

	// Is copilot enabled this project in Pulumi Cloud
	if b.copilotEnabledForCurrentProject == nil {
		logging.V(3).Info(
			"error: copilotEnabledForCurrentProject has not been set. only available after an update has been started.")
		return false
	}

	return *b.copilotEnabledForCurrentProject
}

// explain takes engine events, renders them out to a buffer as something similar to what the user sees
// in the CLI, and then explains the output with Copilot.
func (b *cloudBackend) Explain(
	ctx context.Context,
	stackRef backend.StackReference,
	kind apitype.UpdateKind,
	op backend.UpdateOperation,
	events []engine.Event,
) (string, error) {
	renderer := display.NewCaptureProgressEvents(
		stackRef.Name(),
		op.Proj.Name,
		display.Options{
			ShowResourceChanges: true,
		},
		true, /* isPreview */
		kind,
	)
	renderer.ProcessEventSlice(events)
	output := renderer.Output()

	if output == "" {
		return "", errors.New("no output from preview")
	}

	stackID, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return "", err
	}

	displayOpts := op.Opts.Display
	display.RenderCopilotThinking(displayOpts)
	orgID := stackID.Owner
	summary, err := b.client.ExplainPreviewWithCopilot(ctx, orgID, string(kind), output)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// Format a better error message for the user
			return "", fmt.Errorf("request to %s timed out after %s", b.client.URL(), client.CopilotRequestTimeout.String())
		}
		return "", err
	}

	if summary == "" {
		summary = "No summary available"
	}

	formattedSummary := display.FormatCopilotSummary(summary, displayOpts)

	return formattedSummary, nil
}

func (b *cloudBackend) Import(ctx context.Context, stack backend.Stack,
	op backend.UpdateOperation, imports []deploy.Import,
) (sdkDisplay.ResourceChanges, error) {
	op.Imports = imports

	if op.Opts.PreviewOnly {
		// We can skip PreviewThenPromptThenExecute, and just go straight to Execute.
		opts := backend.ApplierOptions{
			DryRun:   true,
			ShowLink: true,
		}

		op.Opts.Engine.GeneratePlan = false
		_, changes, err := b.apply(
			ctx, apitype.ResourceImportUpdate, stack, op, opts, nil /*events*/)
		return changes, err
	}

	return backend.PreviewThenPromptThenExecute(ctx, apitype.ResourceImportUpdate, stack, op, b.apply, b, nil)
}

func (b *cloudBackend) Refresh(ctx context.Context, stack backend.Stack,
	op backend.UpdateOperation,
) (sdkDisplay.ResourceChanges, error) {
	if op.Opts.PreviewOnly {
		// We can skip PreviewThenPromptThenExecute, and just go straight to Execute.
		opts := backend.ApplierOptions{
			DryRun:   true,
			ShowLink: true,
		}

		op.Opts.Engine.GeneratePlan = false
		_, changes, err := b.apply(
			ctx, apitype.RefreshUpdate, stack, op, opts, nil /*events*/)
		return changes, err
	}
	return backend.PreviewThenPromptThenExecute(ctx, apitype.RefreshUpdate, stack, op, b.apply, b, nil)
}

func (b *cloudBackend) Destroy(ctx context.Context, stack backend.Stack,
	op backend.UpdateOperation,
) (sdkDisplay.ResourceChanges, error) {
	if op.Opts.PreviewOnly {
		// We can skip PreviewThenPromptThenExecute, and just go straight to Execute.
		opts := backend.ApplierOptions{
			DryRun:   true,
			ShowLink: true,
		}

		op.Opts.Engine.GeneratePlan = false
		_, changes, err := b.apply(
			ctx, apitype.DestroyUpdate, stack, op, opts, nil /*events*/)
		return changes, err
	}
	return backend.PreviewThenPromptThenExecute(ctx, apitype.DestroyUpdate, stack, op, b.apply, b, nil)
}

func (b *cloudBackend) Watch(ctx context.Context, stk backend.Stack,
	op backend.UpdateOperation, paths []string,
) error {
	return backend.Watch(ctx, b, stk, op, b.apply, paths)
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

func (b *cloudBackend) PromptAI(
	ctx context.Context, requestBody AIPromptRequestBody,
) (*http.Response, error) {
	res, err := b.client.SubmitAIPrompt(ctx, requestBody)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to submit AI prompt: %s", res.Status)
	}
	return res, nil
}

func (b *cloudBackend) renderAndSummarizeOutput(
	ctx context.Context, kind apitype.UpdateKind, stack backend.Stack, op backend.UpdateOperation,
	events []engine.Event, update client.UpdateIdentifier, updateMeta updateMetadata, dryRun bool,
) {
	renderer := display.NewCaptureProgressEvents(
		stack.Ref().Name(),
		op.Proj.Name,
		display.Options{
			ShowResourceChanges: true,
		},
		dryRun,
		kind,
	)
	renderer.ProcessEventSlice(events)

	permalink := b.getPermalink(update, updateMeta.version, dryRun)
	if renderer.OutputIncludesFailure() {
		summary, err := b.summarizeErrorWithCopilot(ctx, renderer.Output(), stack.Ref(), op.Opts.Display)
		// Pass the error into the renderer to ensure it's displayed. We don't want to fail the update/preview
		// if we can't generate a summary.
		display.RenderCopilotErrorSummary(summary, err, op.Opts.Display, permalink)
	}
}

func (b *cloudBackend) summarizeErrorWithCopilot(
	ctx context.Context, pulumiOutput string, stackRef backend.StackReference, opts display.Options,
) (*display.CopilotErrorSummaryMetadata, error) {
	if len(pulumiOutput) == 0 {
		return nil, nil
	}

	stackID, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return nil, err
	}
	orgName := stackID.Owner

	model := opts.CopilotSummaryModel
	maxSummaryLen := opts.CopilotSummaryMaxLen

	summary, err := b.client.SummarizeErrorWithCopilot(ctx, orgName, pulumiOutput, model, maxSummaryLen)
	if err != nil {
		return nil, err
	}

	if summary == "" {
		// Summarization did not return output, this is not an error.
		return nil, nil
	}

	return &display.CopilotErrorSummaryMetadata{
		Summary: summary,
	}, nil
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
	// Once we start an update we want to keep the machine from sleeping.
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
			conflict := backenderr.ConflictingUpdateError{Err: err}
			return client.UpdateIdentifier{}, updateMetadata{}, conflict
		}
		return client.UpdateIdentifier{}, updateMetadata{}, err
	}
	// Any non-preview update will be considered part of the stack's update history.
	if action != apitype.PreviewUpdate {
		logging.V(7).Infof("Stack %s being updated to version %d", stackRef, version)
	}

	userName, _, _, err := b.CurrentUser()
	if err != nil {
		userName = "unknown"
	}
	// Check if the user's org (stack's owner) has Copilot enabled. If not, we don't show the link to Copilot.
	isCopilotEnabled := updateDetails.IsCopilotIntegrationEnabled
	b.copilotEnabledForCurrentProject = &isCopilotEnabled
	copilotEnabledValueString := "is"
	continuationString := ""
	if isCopilotEnabled {
		if env.SuppressCopilotLink.Value() {
			// Copilot is enabled in user's org, but the environment variable to suppress the link to Copilot is set.
			op.Opts.Display.ShowLinkToCopilot = false
			continuationString = " but the environment variable PULUMI_SUPPRESS_COPILOT_LINK" +
				" suppresses the link to Copilot in diagnostics"
		}
	} else {
		op.Opts.Display.ShowLinkToCopilot = false
		op.Opts.Display.ShowCopilotFeatures = false
		copilotEnabledValueString = "is not"
	}
	logging.V(7).Infof("Copilot in org '%s' %s enabled for user '%s'%s",
		stackID.Owner, copilotEnabledValueString, userName, continuationString)

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
) (*deploy.Plan, sdkDisplay.ResourceChanges, error) {
	resetKeepRunning := nosleep.KeepRunning()
	defer resetKeepRunning()

	actionLabel := backend.ActionLabel(kind, opts.DryRun)

	if !op.Opts.Display.JSONDisplay && op.Opts.Display.Type != display.DisplayWatch {
		// Print a banner so it's clear this is going to the cloud.
		fmt.Printf(op.Opts.Display.Color.Colorize(
			colors.SpecHeadline+"%s (%s)"+colors.Reset+"\n\n"), actionLabel, stack.Ref())
	}

	// Create an update object to persist results.
	update, updateMeta, err := b.createAndStartUpdate(ctx, kind, stack, &op, opts.DryRun)
	if err != nil {
		return nil, nil, err
	}

	if b.isCopilotFeaturesEnabled(op.Opts.Display) {
		if !b.Capabilities(ctx).CopilotSummarizeErrorV1 {
			logging.V(7).Infof("CopilotSummarizeErrorV1 is not supported by the backend")
		} else {
			originalEvents := events
			// New var as we need a bidirectional channel type to be able to read from it.
			eventsChannel := make(chan engine.Event)
			events = eventsChannel

			var renderEvents []engine.Event
			done := make(chan bool)
			go func() {
				for e := range eventsChannel {
					// Forward all events from the engine to the original channel.
					// (e.g. PreviewThenPrompt also saves events to be able to generate a diff on request).
					if originalEvents != nil {
						originalEvents <- e
					}
					// Do not send internal events to the copilot summary as they are not displayed to the user either.
					// We can skip Ephemeral events as well as we want to display the "final" output.
					if e.Internal() || e.Ephemeral() {
						continue
					}
					renderEvents = append(renderEvents, e)
				}
				done <- true
			}()
			defer func() {
				close(eventsChannel)
				<-done
				b.renderAndSummarizeOutput(ctx, kind, stack, op, renderEvents, update, updateMeta, opts.DryRun)
			}()
		}
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

func (b *cloudBackend) runEngineAction(
	ctx context.Context, kind apitype.UpdateKind, stackRef backend.StackReference,
	op backend.UpdateOperation, update client.UpdateIdentifier, token, permalink string,
	callerEventsOpt chan<- engine.Event, dryRun bool,
) (*deploy.Plan, sdkDisplay.ResourceChanges, error) {
	contract.Assertf(token != "", "persisted actions require a token")
	u, tokenSource, err := b.newUpdate(ctx, stackRef, op, update, token)
	if err != nil {
		return nil, nil, err
	}

	// displayEvents renders the event to the console and Pulumi service. The processor for the
	// will signal all events have been proceed when a value is written to the displayDone channel.
	displayEvents := make(chan engine.Event)
	displayDone := make(chan bool)

	go b.recordAndDisplayEvents(
		ctx, tokenSource, update,
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

	// We only need a snapshot manager if we're doing an update.
	var snapshotManager *backend.SnapshotManager
	if kind != apitype.PreviewUpdate && !dryRun {
		persister := b.newSnapshotPersister(ctx, update, tokenSource)
		snapshotManager = backend.NewSnapshotManager(persister, op.SecretsManager, u.Target.Snapshot)
	}

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
		if op.Opts.Engine.RefreshProgram {
			_, changes, updateErr = engine.RefreshV2(u, engineCtx, op.Opts.Engine, dryRun)
		} else {
			_, changes, updateErr = engine.Refresh(u, engineCtx, op.Opts.Engine, dryRun)
		}
	case apitype.DestroyUpdate:
		if op.Opts.Engine.DestroyProgram {
			_, changes, updateErr = engine.DestroyV2(u, engineCtx, op.Opts.Engine, dryRun)
		} else {
			_, changes, updateErr = engine.Destroy(u, engineCtx, op.Opts.Engine, dryRun)
		}
	case apitype.StackImportUpdate, apitype.RenameUpdate:
		contract.Failf("unexpected %s event", kind)
	default:
		contract.Failf("Unrecognized update kind: %s", kind)
	}

	// Wait for dependent channels to finish processing engineEvents before closing.
	<-displayDone
	cancellationScope.Close() // Don't take any cancellations anymore, we're shutting down.
	close(engineEvents)
	if snapshotManager != nil {
		err = snapshotManager.Close()
		// If the snapshot manager failed to close, we should return that error.
		// Even though all the parts of the operation have potentially succeeded, a
		// snapshotting failure is likely to rear its head on the next
		// operation/invocation (e.g. an invalid snapshot that fails integrity
		// checks, or a failure to write that means the snapshot is incomplete).
		// Reporting now should make debugging and reporting easier.
		if err != nil {
			return plan, changes, fmt.Errorf("writing snapshot: %w", err)
		}
	}

	// Make sure that the goroutine writing to displayEvents and callerEventsOpt
	// has exited before proceeding
	<-eventsDone
	close(displayEvents)

	// Mark the update as complete.
	status := apitype.UpdateStatusSucceeded
	if updateErr != nil {
		status = apitype.UpdateStatusFailed
	}
	completeErr := b.completeUpdate(ctx, tokenSource, update, status)
	if completeErr != nil {
		updateErr = result.MergeBails(updateErr, fmt.Errorf("failed to complete update: %w", completeErr))
	}

	return plan, changes, updateErr
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
		return nil, backenderr.ErrNoPreviousDeployment
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

// convertConfig converts the apitype version of config.Map into the internal version.
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
	return diy.GetLogsForTarget(target, logQuery)
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

	b.downgradeUntypedDeploymentVersionIfNeeded(ctx, deployment)

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
		Stack:   cloudBackendStackRef.name,
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
	spinner, ticker := cmdutil.NewSpinnerAndTicker(prefix, nil, opts.Color, 8 /*timesPerSecond*/, opts.SuppressProgress)

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

func (b *cloudBackend) EncryptStackDeploymentSettingsSecret(ctx context.Context,
	stack backend.Stack, secret string,
) (*apitype.SecretValue, error) {
	stackID, err := b.getCloudStackIdentifier(stack.Ref())
	if err != nil {
		return nil, err
	}

	return b.client.EncryptStackDeploymentSettingsSecret(ctx, stackID, secret)
}

func (b *cloudBackend) UpdateStackDeploymentSettings(ctx context.Context, stack backend.Stack,
	deployment apitype.DeploymentSettings,
) error {
	stackID, err := b.getCloudStackIdentifier(stack.Ref())
	if err != nil {
		return err
	}

	return b.client.UpdateStackDeploymentSettings(ctx, stackID, deployment)
}

func (b *cloudBackend) DestroyStackDeploymentSettings(ctx context.Context, stack backend.Stack) error {
	stackID, err := b.getCloudStackIdentifier(stack.Ref())
	if err != nil {
		return err
	}

	return b.client.DestroyStackDeploymentSettings(ctx, stackID)
}

func (b *cloudBackend) GetGHAppIntegration(
	ctx context.Context, stack backend.Stack,
) (*apitype.GitHubAppIntegration, error) {
	stackID, err := b.getCloudStackIdentifier(stack.Ref())
	if err != nil {
		return nil, err
	}

	return b.client.GetGHAppIntegration(ctx, stackID)
}

func (b *cloudBackend) GetStackDeploymentSettings(ctx context.Context,
	stack backend.Stack,
) (*apitype.DeploymentSettings, error) {
	stackID, err := b.getCloudStackIdentifier(stack.Ref())
	if err != nil {
		return nil, err
	}

	return b.client.GetStackDeploymentSettings(ctx, stackID)
}

func (b *cloudBackend) RunDeployment(ctx context.Context, stackRef backend.StackReference,
	req apitype.CreateDeploymentRequest, opts display.Options, deploymentInitiator string,
	suppressStreamLogs bool,
) error {
	stackID, err := b.getCloudStackIdentifier(stackRef)
	if err != nil {
		return err
	}

	resp, err := b.client.CreateDeployment(ctx, stackID, req, deploymentInitiator)
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

	if suppressStreamLogs {
		return nil
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
				if l.Header == fmt.Sprintf("pulumi %v", req.Op) {
					fmt.Println()
					return b.showDeploymentEvents(ctx, stackID, apitype.UpdateKind(req.Op), id, opts)
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
		backend.ActionLabel(kind, dryRun), kind, stackID.Stack, tokens.PackageName(stackID.Project),
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
				events <- engine.NewCancelEvent()
			}

			close(events)
			<-done
			return nil
		}

		time.Sleep(500 * time.Millisecond)
	}
}

func (b *cloudBackend) GetDefaultOrg(ctx context.Context) (string, error) {
	resp, err := b.client.GetDefaultOrg(ctx)
	if err != nil {
		return "", err
	}
	return resp.GitHubLogin, nil
}

type httpstateBackendClient struct {
	backend deploy.BackendClient
}

func (c httpstateBackendClient) GetStackOutputs(
	ctx context.Context,
	name string,
	onDecryptError func(error) error,
) (resource.PropertyMap, error) {
	// When using the cloud backend, require that stack references are fully qualified so they
	// look like "<org>/<project>/<stack>"
	if strings.Count(name, "/") != 2 {
		return nil, errors.New("a stack reference's name should be of the form '<organization>/<project>/<stack>'. " +
			"See https://www.pulumi.com/docs/using-pulumi/stack-outputs-and-references/#using-stack-references " +
			"for more information.")
	}

	return c.backend.GetStackOutputs(ctx, name, onDecryptError)
}

func (c httpstateBackendClient) GetStackResourceOutputs(
	ctx context.Context, name string,
) (resource.PropertyMap, error) {
	return c.backend.GetStackResourceOutputs(ctx, name)
}

// Builds a lazy wrapper around doDetectCapabilities.
func detectCapabilities(d diag.Sink, client *client.Client) *promise.Promise[apitype.Capabilities] {
	return promise.Run(func() (apitype.Capabilities, error) {
		return doDetectCapabilities(context.Background(), d, client), nil
	})
}

func doDetectCapabilities(ctx context.Context, d diag.Sink, client *client.Client) apitype.Capabilities {
	resp, err := client.GetCapabilities(ctx)
	if err != nil {
		d.Warningf(diag.Message("" /*urn*/, "failed to get capabilities: %v"), err)
		return apitype.Capabilities{}
	}
	caps, err := resp.Parse()
	if err != nil {
		d.Warningf(diag.Message("" /*urn*/, "failed to decode capabilities: %v"), err)
		return apitype.Capabilities{}
	}

	// Allow users to opt out of deltaCheckpointUpdates even if the backend indicates it should be used. This
	// remains necessary while PULUMI_OPTIMIZED_CHECKPOINT_PATCH has higher memory requirements on the client and
	// may cause out-of-memory issues in constrained environments.
	switch strings.ToLower(os.Getenv("PULUMI_OPTIMIZED_CHECKPOINT_PATCH")) {
	case "0", "false":
		caps.DeltaCheckpointUpdates = nil
	}

	return caps
}

func (b *cloudBackend) DefaultSecretManager(*workspace.ProjectStack) (secrets.Manager, error) {
	// The default secrets manager for a cloud-backed stack is a cloud secrets manager, which is inherently
	// stack-specific. Thus at the backend level we return nil, deferring to Stack.DefaultSecretManager when the stack has
	// been created.
	return nil, nil
}

func (b *cloudBackend) GetCloudRegistry() (backend.CloudRegistry, error) {
	return newCloudRegistry(b.client), nil
}

func (b *cloudBackend) GetReadOnlyCloudRegistry() registry.Registry {
	return newCloudRegistry(b.client)
}

// downgradeDeploymentVersionIfNeeded downgrades the deployment schema version to 3 if the service does not
// support a higher version. This is necessary to ensure compatibility with versions of the service, such as
// the self-hosted service, that do not support the latest deployment schema version.
func (b *cloudBackend) downgradeDeploymentVersionIfNeeded(
	ctx context.Context, version int, features []string,
) (int, []string) {
	// Downgrade to v3 if the version is greater than 3 and the service does not support it.
	// Version 3 is supported by the service even if the version from capabilities isn't set.
	if version > 3 && b.Capabilities(ctx).DeploymentSchemaVersion <= 3 {
		logging.V(7).Infof("Downgrading deployment schema version %d to 3 for compatibility with backend", version)
		return 3, nil
	}
	return version, features
}

// downgradeUntypedDeploymentVersionIfNeeded downgrades the deployment schema version to 3 if the service does not
// support a higher version. This is necessary to ensure compatibility with versions of the service, such as
// the self-hosted service, that do not support the latest deployment schema version.
func (b *cloudBackend) downgradeUntypedDeploymentVersionIfNeeded(
	ctx context.Context, deployment *apitype.UntypedDeployment,
) {
	if deployment != nil {
		deployment.Version, deployment.Features = b.downgradeDeploymentVersionIfNeeded(
			ctx, deployment.Version, deployment.Features)
	}
}
