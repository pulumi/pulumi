package httpstate

import httpstate "github.com/pulumi/pulumi/sdk/v3/pkg/backend/httpstate"

type PulumiAILanguage = httpstate.PulumiAILanguage

type AIPromptRequestBody = httpstate.AIPromptRequestBody

// Backend extends the base backend interface with specific information about cloud backends.
type Backend = httpstate.Backend

// LoginManager provides a slim wrapper around functions related to backend logins.
type LoginManager = httpstate.LoginManager

type DisplayEventType = httpstate.DisplayEventType

const PulumiAILanguageTypeScript = httpstate.PulumiAILanguageTypeScript

const PulumiAILanguageJavaScript = httpstate.PulumiAILanguageJavaScript

const PulumiAILanguagePython = httpstate.PulumiAILanguagePython

const PulumiAILanguageGo = httpstate.PulumiAILanguageGo

const PulumiAILanguageCSharp = httpstate.PulumiAILanguageCSharp

const PulumiAILanguageJava = httpstate.PulumiAILanguageJava

const PulumiAILanguageYAML = httpstate.PulumiAILanguageYAML

// A natural language list of languages supported by Pulumi AI.
const PulumiAILanguagesClause = httpstate.PulumiAILanguagesClause

const UpdateEvent = httpstate.UpdateEvent

const ShutdownEvent = httpstate.ShutdownEvent

// All of the languages supported by Pulumi AI.
var PulumiAILanguageOptions = httpstate.PulumiAILanguageOptions

// DefaultURL returns the default cloud URL.  This may be overridden using the PULUMI_API environment
// variable.  If no override is found, and we are authenticated with a cloud, choose that.  Otherwise,
// we will default to the https://api.pulumi.com/ endpoint.
func DefaultURL(ws pkgWorkspace.Context) string {
	return httpstate.DefaultURL(ws)
}

// ValueOrDefaultURL returns the value if specified, or the default cloud URL otherwise.
func ValueOrDefaultURL(ws pkgWorkspace.Context, cloudURL string) string {
	return httpstate.ValueOrDefaultURL(ws, cloudURL)
}

// New creates a new Pulumi backend for the given cloud API URL and token.
func New(ctx context.Context, d diag.Sink, cloudURL string, project *workspace.Project, insecure bool) (Backend, error) {
	return httpstate.New(ctx, d, cloudURL, project, insecure)
}

// NewLoginManager returns a LoginManager for handling backend logins.
func NewLoginManager() LoginManager {
	return httpstate.NewLoginManager()
}

// WelcomeUser prints a Welcome to Pulumi message.
func WelcomeUser(opts display.Options) {
	httpstate.WelcomeUser(opts)
}

// IsValidAccessToken tries to use the provided Pulumi access token and returns if it is accepted
// or not. Returns error on any unexpected error.
func IsValidAccessToken(ctx context.Context, cloudURL string, insecure bool, accessToken string) (bool, string, []string, *workspace.TokenInformation, error) {
	return httpstate.IsValidAccessToken(ctx, cloudURL, insecure, accessToken)
}

