// Copyright 2026, Pulumi Corporation.
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

package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	fxmaps "github.com/pgavlin/fx/v2/maps"
	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	"github.com/pulumi/pulumi/pkg/v3/cmd/pulumi/constrictor"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// apiCommand carries state and api for `pulumi cloud api` and the
// dispatcher path beneath it. refreshSpec is persistent and inherits
// into subcommands (list/describe); the rest are local to the dispatcher
// invocation (`pulumi cloud api <path>`).
type apiCommand struct {
	// Persistent flag inherited by subcommands.
	refreshSpec bool

	// Local api for the dispatcher path. Flag names are a stable contract
	// matching `gh api`.
	method          string
	fields          []string
	rawFields       []string
	headers         []string
	input           string
	body            string
	paginate        bool
	include         bool
	silent          bool
	verbose         bool
	dryRun          bool
	format          string
	envelopeVersion int
}

// bindFlags attaches the full flag set to cmd, binding each flag to a field
// on api. refreshSpec is persistent (inherited by subcommands); the rest
// are local to the dispatcher path.
func bindFlags(cmd *cobra.Command, api *apiCommand) {
	cmd.PersistentFlags().BoolVar(&api.refreshSpec, "refresh-spec", false,
		"Re-fetch the OpenAPI spec from Pulumi Cloud and overwrite the local cache")

	pf := cmd.Flags()
	pf.StringVarP(&api.method, "method", "X", "",
		"HTTP method (default GET, POST when body fields are present)")
	pf.StringArrayVarP(&api.fields, "field", "F", nil,
		"Typed key=value; numbers/bools/null auto-detected; JSON object/array "+
			"literals parsed; @file reads file; @- reads stdin. "+
			"Sent as query params on GET/HEAD, JSON body fields otherwise")
	pf.StringArrayVarP(&api.rawFields, "raw-field", "f", nil,
		"String key=value with no type coercion. "+
			"Sent as query params on GET/HEAD, JSON body fields otherwise")
	pf.StringArrayVarP(&api.headers, "header", "H", nil,
		"Custom HTTP header `Key: Value` (repeatable)")
	pf.StringVar(&api.input, "input", "",
		"Read request body from file; `-` reads stdin")
	pf.StringVar(&api.body, "body", "",
		"Inline request body sent verbatim (default Content-Type: application/json). "+
			"Mutually exclusive with --input")
	pf.BoolVar(&api.paginate, "paginate", false,
		"Follow pagination cursors and emit the combined result")
	pf.BoolVarP(&api.include, "include", "i", false,
		"Include HTTP status line and response headers in output")
	pf.BoolVar(&api.silent, "silent", false,
		"Do not print the response body on success; errors are still printed and exit non-zero")
	pf.BoolVar(&api.verbose, "verbose", false,
		"Dump full request and response to stderr")
	pf.BoolVar(&api.dryRun, "dry-run", false,
		"Print the resolved request without sending it")
	pf.StringVar(&api.format, "format", "",
		"Drive content negotiation and rendering. Default uses the op's primary "+
			"response content type (usually JSON). `json` or `markdown` request that "+
			"format via the Accept header — rejected if the op's spec doesn't declare it. "+
			"`raw` keeps the op's default Accept and writes the body through unchanged.")
	pf.IntVar(&api.envelopeVersion, "envelope-version", SchemaVersion,
		"Pin the JSON envelope version the caller expects")
}

func newAPICmd() *cobra.Command {
	api := &apiCommand{envelopeVersion: SchemaVersion}

	cmd := &cobra.Command{
		Use:   "api",
		Short: "Call any Pulumi Cloud API endpoint",
		Long: "Call any Pulumi Cloud API endpoint.\n" +
			"\n" +
			"The positional argument may be: a path (with optional {template} variables, e.g.\n" +
			"`/api/orgs/{orgName}/members`), an operation ID as shown in `list` (e.g.\n" +
			"`ListOrgMembers`), or a paste-friendly row (`GET /api/...`). Template variables\n" +
			"are resolved from the current Pulumi project / selected stack, or supplied\n" +
			"with -F (e.g. `-F orgName=acme`).\n" +
			"\n" +
			"Fields provided via -F/--field and -f/--raw-field are sent as query parameters\n" +
			"on GET/HEAD requests and as a JSON request body on POST/PUT/PATCH/DELETE. Method\n" +
			"defaults to GET, or POST when body fields, --body, or --input are present.\n" +
			"\n" +
			"Value forms accepted by -F:\n" +
			"  - scalars: `-F name=acme`, `-F count=3`, `-F enabled=true`, `-F note=null`\n" +
			"  - nested JSON: `-F 'labels={\"env\":\"prod\"}'`, `-F 'tags=[\"a\",\"b\"]'`\n" +
			"  - file / stdin: `-F body=@./payload.json`, `-F note=@-`\n" +
			"Use -f/--raw-field to suppress coercion and send the value verbatim as a string.\n" +
			"\n" +
			"For an entire request body, pass --body '<json>' inline, or --input <file> (use\n" +
			"`-` for stdin) to stream from a file.\n" +
			"\n" +
			"A field whose key matches a path template variable (e.g. `-F poolId=123` against\n" +
			"`{poolId}`) fills that variable and is not forwarded as a query or body parameter.\n" +
			"This is the way to supply non-context path parameters when calling by operation\n" +
			"ID.\n" +
			"\n" +
			"When a path template and the request body share a parameter name, the first\n" +
			"matching -F is consumed for the path and any subsequent -F with the same key is\n" +
			"sent in the body. Pass -F twice to fill both, or use --body / --input to supply\n" +
			"the body separately.\n" +
			"\n" +
			"The OpenAPI spec is cached locally for 24 hours; pass --refresh-spec on any\n" +
			"subcommand to force a re-fetch.\n" +
			"\n" +
			"Exit codes: 0 success; 1 caller error; 2 invalid flags; 3 auth; 8 cancelled;\n" +
			"255 internal.",
		Example: "  # Inspect the currently authenticated user.\n" +
			"  pulumi cloud api /api/user\n\n" +
			"  # Call by raw path with template variables filled from -F.\n" +
			"  pulumi cloud api /api/orgs/{orgName}/members -F orgName=acme\n\n" +
			"  # Multiple template variables in the path are filled the same way.\n" +
			"  pulumi cloud api /api/stacks/{orgName}/{projectName}/{stackName} \\\n" +
			"    -F orgName=acme -F projectName=web -F stackName=prod\n\n" +
			"  # Call by operation ID — orgName is taken from the current Pulumi project.\n" +
			"  pulumi cloud api ListOrgMembers\n\n" +
			"  # Pass path variables explicitly when no project context is available.\n" +
			"  pulumi cloud api GetStack -F orgName=acme -F projectName=web -F stackName=prod\n\n" +
			"  # Create a resource via POST; body fields go into a JSON body automatically.\n" +
			"  pulumi cloud api CreateStackTag -F orgName=acme -F projectName=web \\\n" +
			"    -F stackName=prod -F name=env -F value=prod\n\n" +
			"  # Build a nested body by mixing scalar fields with an inline JSON object.\n" +
			"  pulumi cloud api CreateStack -F orgName=acme -F projectName=web \\\n" +
			"    -F stackName=prod -F 'tags={\"env\":\"prod\",\"team\":\"platform\"}'\n\n" +
			"  # Send an array of items using a JSON literal.\n" +
			"  pulumi cloud api AddTeamMembers -F orgName=acme -F teamName=eng \\\n" +
			"    -F 'members=[\"alice\",\"bob\",\"carol\"]'\n\n" +
			"  # Pass the entire request body inline with --body.\n" +
			"  pulumi cloud api UpdateStack -F orgName=acme -F projectName=web -F stackName=prod \\\n" +
			"    --body '{\"description\":\"managed by agent\"}'\n\n" +
			"  # Read a JSON body from a file, or from stdin with `-`.\n" +
			"  pulumi cloud api UpdateStackTag --input ./tag.json\n" +
			"  cat tag.json | pulumi cloud api UpdateStackTag --input -\n\n" +
			"  # Filter the JSON response with jq.\n" +
			"  pulumi cloud api /api/user --format=json | jq '.githubLogin'\n\n" +
			"  # Follow pagination cursors and stream the combined result to jq.\n" +
			"  pulumi cloud api ListStacks --paginate | jq '.stacks[].name'\n\n" +
			"  # Extract just the status line + headers without the body.\n" +
			"  pulumi cloud api /api/user --include --silent\n\n" +
			"  # Preview the resolved request without sending it.\n" +
			"  pulumi cloud api CreateStackTag -F orgName=acme --dry-run",
	}
	constrictor.AttachArguments(cmd, &constrictor.Arguments{
		Arguments: []constrictor.Argument{{Name: "path-or-operation-id"}},
		Required:  0,
	})

	bindFlags(cmd, api)

	cmd.RunE = runWithEnvelope(func(cmd *cobra.Command, args []string) error {
		return runAPI(cmd, args, api)
	})

	cmd.AddCommand(newLsCmd(api))
	cmd.AddCommand(newDescribeCmd(api))

	return cmd
}

func runAPI(cmd *cobra.Command, args []string, api *apiCommand) error {
	if err := validateFlagCombos(api); err != nil {
		return err
	}

	if len(args) == 0 {
		return NewAPIError(cmdutil.ExitCodeError, ErrRequiredInputMissing,
			"no endpoint provided").
			WithField("path").
			WithSuggestions(
				"pass a path, e.g. `pulumi cloud api /api/user`",
				"or an operation ID, e.g. `pulumi cloud api ListAccounts`",
				"run `pulumi cloud api list` to see available endpoints",
				"run `pulumi cloud api describe <path-or-operation-id>` to inspect one",
			)
	}
	userArg := strings.TrimSpace(args[0])

	idx, err := LoadIndex(cmd.Context(), cmd.ErrOrStderr(), api.refreshSpec)
	if err != nil {
		return err
	}

	methodExplicit := cmd.Flags().Changed("method")
	method, mr, rawQuery, err := resolveAPIArg(idx, userArg, api, methodExplicit)
	if err != nil {
		return err
	}

	fields, err := parseFields(api.fields, api.rawFields, os.Stdin)
	if err != nil {
		return err
	}

	// Resolve cloud context up front so the backend-aware default org is
	// available to template-var resolution. ResolveContext is non-interactive
	// and returns a usable (anonymous) context when the user isn't logged in.
	resolvedCtx, err := ResolveContext(cmd.Context())
	if err != nil {
		return NewAPIError(cmdutil.ExitInternalError, ErrToolError,
			fmt.Sprintf("resolving cloud context: %v", err))
	}

	bindings, fields, err := resolveBindings(mr, fields, resolvedCtx)
	if err != nil {
		return err
	}

	concretePath := buildConcretePath(mr.Op, bindings)

	hdrs, err := parseHeaders(api.headers)
	if err != nil {
		return err
	}

	bodyBytes, queryExtras, contentType, err := encodeFields(method, fields, api, mr.Op)
	if err != nil {
		return err
	}

	query := mergeQuery(rawQuery, queryExtras)

	accept, err := negotiateAccept(mr.Op, api.format)
	if err != nil {
		return err
	}

	if api.dryRun {
		fullURL := strings.TrimRight(resolvedCtx.CloudURL, "/") + concretePath
		if query != "" {
			fullURL += "?" + query
		}
		return emitDryRun(cmd.OutOrStdout(), method, fullURL, hdrs, contentType, accept, bodyBytes)
	}

	return executeLive(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), mr.Op, method, concretePath, query, hdrs,
		contentType, accept, bodyBytes, api, resolvedCtx)
}

// executeLive issues the HTTP request and post-processes the response
// according to the flag set (--include, --silent, --verbose). resolved is
// the cloud context produced by runAPI; this layer only enforces that the
// caller is authenticated. accept is the Accept header value chosen by
// negotiateAccept upstream.
func executeLive(
	ctx context.Context,
	w, errW io.Writer,
	op *Operation,
	method, concretePath, query string,
	hdrs []ParsedHeader,
	contentType, accept string,
	body []byte,
	api *apiCommand,
	resolved *ResolvedContext,
) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if !resolved.LoggedIn {
		return NewAPIError(cmdutil.ExitAuthenticationError, ErrNotLoggedIn,
			"not logged in to Pulumi Cloud").
			WithSuggestions("run `pulumi login` first")
	}

	apiClient := resolved.Client

	if api.verbose {
		dumpRequestVerbose(errW, method, strings.TrimRight(resolved.CloudURL, "/")+concretePath+queryFragment(query),
			hdrs, contentType, accept, body)
	}

	baseQuery, err := url.ParseQuery(query)
	if err != nil {
		return NewAPIError(cmdutil.ExitCodeError, ErrInvalidFlags,
			fmt.Sprintf("parsing query string %q: %v", query, err)).WithField("path")
	}

	if api.paginate {
		return runPaginate(ctx, w, apiClient, paginateRequest{
			Method:      method,
			Path:        concretePath,
			BaseQuery:   baseQuery,
			Body:        body,
			ContentType: contentType,
			Accept:      accept,
			Headers:     hdrs,
			Op:          op,
		}, api)
	}

	var bodyReader io.Reader
	if len(body) > 0 {
		bodyReader = bytes.NewReader(body)
	}
	resp, err := apiClient.RawCall(ctx, method, concretePath, baseQuery, bodyReader,
		buildAPIHeaders(contentType, accept, hdrs), len(body) > 0)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return &APIError{ExitCode: cmdutil.ExitCancelled, Silent: true}
		}
		return NewAPIError(cmdutil.ExitCodeError, ErrNetwork,
			fmt.Sprintf("HTTP %s %s: %v", method, concretePath, err))
	}
	defer resp.Body.Close()

	return handleResponse(w, errW, resp, api)
}

// queryFragment returns "?<query>" when query is non-empty, else "".
func queryFragment(query string) string {
	if query == "" {
		return ""
	}
	return "?" + query
}

// dumpRequestVerbose writes the full outgoing request to errW for --verbose.
// Authorization is redacted; user-supplied Authorization overrides are dropped
// to match apiClient.Do, which also drops them.
func dumpRequestVerbose(
	errW io.Writer,
	method, fullURL string, hdrs []ParsedHeader,
	contentType, accept string, body []byte,
) {
	fmt.Fprintf(errW, "> %s %s\n", method, fullURL)
	fmt.Fprintf(errW, "> Authorization: token ***\n")
	fmt.Fprintf(errW, "> Accept: %s, application/vnd.pulumi+8\n", accept)
	if contentType != "" {
		fmt.Fprintf(errW, "> Content-Type: %s\n", contentType)
	}
	for _, h := range hdrs {
		if strings.EqualFold(h.Name, "Authorization") {
			continue
		}
		fmt.Fprintf(errW, "> %s: %s\n", h.Name, h.Value)
	}
	if len(body) > 0 {
		fmt.Fprintln(errW, ">")
		_, _ = errW.Write(body)
		fmt.Fprintln(errW)
	}
}

// handleResponse applies post-processing api (--include, --silent, --verbose)
// and renders the body via the content-type router.
func handleResponse(w, errW io.Writer, resp *http.Response, api *apiCommand) error {
	if api.include || api.verbose {
		writeStatusAndHeaders(w, resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return NewAPIError(cmdutil.ExitCodeError, ErrNetwork,
			fmt.Sprintf("reading response body: %v", err))
	}

	if api.verbose {
		if len(body) > 0 {
			fmt.Fprintln(errW, "<")
			_, _ = errW.Write(body)
			fmt.Fprintln(errW)
		}
	}

	if resp.StatusCode >= 400 {
		return httpErrorEnvelopeBytes(resp, body)
	}

	if api.silent {
		return nil
	}
	if len(body) == 0 {
		return nil
	}

	return renderBody(w, resp.Header.Get("Content-Type"), body)
}

// writeStatusAndHeaders writes the status line + sorted headers to w.
// Headers are sorted lexicographically for deterministic output.
func writeStatusAndHeaders(w io.Writer, resp *http.Response) {
	fmt.Fprintf(w, "HTTP/%d.%d %s\n", resp.ProtoMajor, resp.ProtoMinor, resp.Status)
	keys := make([]string, 0, len(resp.Header))
	for k := range resp.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range resp.Header.Values(k) {
			fmt.Fprintf(w, "%s: %s\n", k, v)
		}
	}
	fmt.Fprintln(w)
}

// negotiateAccept picks the Accept header value to send for op based on the
// user's --format flag. When --format is unset we use the op's primary
// response content type (historically JSON for Pulumi Cloud). When the user
// explicitly asks for JSON or markdown we validate against the op's declared
// response content types so the call fails fast with a helpful message
// instead of surprising the caller with a 406 from the server.
func negotiateAccept(op *Operation, format string) (string, error) {
	switch strings.ToLower(format) {
	case "", "raw", "auto", "default":
		if op.ResponseContentType != "" {
			return op.ResponseContentType, nil
		}
		return "application/json", nil
	case "json":
		for _, ct := range op.SuccessContentTypes {
			if isJSONContentType(ct) {
				return ct, nil
			}
		}
		return "", unsupportedFormatError(op, "json", "application/json")
	case "markdown", "md":
		for _, ct := range op.SuccessContentTypes {
			if strings.EqualFold(ct, "text/markdown") {
				return ct, nil
			}
		}
		return "", unsupportedFormatError(op, "markdown", "text/markdown")
	default:
		return "", NewAPIError(cmdutil.ExitCodeError, ErrInvalidFlags,
			"invalid --format value: "+format).
			WithField("format").
			WithSuggestions("--format=json", "--format=markdown", "--format=raw")
	}
}

// unsupportedFormatError returns a caller-actionable error when the user
// asks for a format the op doesn't declare. The suggestions surface the
// content types the op does declare so the user can pick one that works.
func unsupportedFormatError(op *Operation, want, mediaType string) error {
	msg := fmt.Sprintf("operation %s does not declare a %s response", op.OperationID, mediaType)
	suggestions := []string{"omit --format to use the op's default content type"}
	if len(op.SuccessContentTypes) > 0 {
		suggestions = append(suggestions,
			"declared response content types: "+strings.Join(op.SuccessContentTypes, ", "))
	}
	return NewAPIError(cmdutil.ExitCodeError, ErrInvalidFlags, msg).
		WithField("format").
		WithSuggestions(suggestions...)
}

// isJSONContentType loosely matches `application/json` plus any `+json`
// subtype (application/vnd.pulumi+json etc.).
func isJSONContentType(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(strings.SplitN(ct, ";", 2)[0]))
	if ct == "application/json" || ct == "text/json" {
		return true
	}
	return strings.HasSuffix(ct, "+json")
}

// renderBody writes body to w, pretty-printing JSON when the content type
// indicates JSON, passing through text/binary otherwise. A thin adapter
// over format.go's existing helpers.
func renderBody(w io.Writer, contentType string, body []byte) error {
	ct := strings.ToLower(contentType)
	switch {
	case isJSONContentType(contentType), ct == "":
		return formatJSON(w, body)
	case strings.Contains(ct, "application/x-yaml"),
		strings.Contains(ct, "text/plain"),
		strings.Contains(ct, "text/markdown"):
		return formatText(w, body)
	case strings.Contains(ct, "application/x-tar"),
		strings.Contains(ct, "application/octet-stream"):
		return formatBinary(w, body)
	default:
		return formatBinary(w, body)
	}
}

// validateFlagCombos enforces flag-set invariants before we do any work.
func validateFlagCombos(api *apiCommand) error {
	if api.envelopeVersion != SchemaVersion {
		return NewAPIError(cmdutil.ExitCodeError, ErrUnsupportedSchemaVersion,
			fmt.Sprintf("envelope version %d is not supported; this CLI speaks envelope version %d",
				api.envelopeVersion, SchemaVersion)).
			WithField("envelope-version").
			WithSuggestions("omit --envelope-version to use the current version")
	}
	if api.body != "" && api.input != "" {
		return NewAPIError(cmdutil.ExitConfigurationError, ErrInvalidFlags,
			"--body and --input are mutually exclusive").
			WithSuggestions(
				"use --body for inline JSON",
				"use --input to read the body from a file",
			)
	}
	if api.silent && api.verbose {
		return NewAPIError(cmdutil.ExitConfigurationError, ErrInvalidFlags,
			"--silent and --verbose are mutually exclusive").
			WithSuggestions(
				"use --silent to drop the response body",
				"use --verbose to dump the full request and response to stderr",
			)
	}
	return nil
}

// resolveAPIArg maps the user's positional argument to a concrete operation.
// Accepts three forms: a path with optional {template} params, an operation
// ID (e.g. "ListAccounts"), or a paste-friendly "METHOD /path" row from
// `ls`. methodExplicit signals whether --method was set; when true and the
// argument pins a different method, a conflict error is returned.
func resolveAPIArg(idx *Index, arg string, api *apiCommand, methodExplicit bool) (string, *MatchResult, string, error) {
	if verb, rest, ok := splitLeadingHTTPMethod(arg); ok {
		if methodExplicit && strings.ToUpper(api.method) != verb {
			return "", nil, "", NewAPIError(cmdutil.ExitConfigurationError, ErrInvalidFlags,
				fmt.Sprintf("conflicting methods: argument has %q, --method is %q", verb, api.method)).
				WithField("method")
		}
		rawPath, rawQuery := splitPathQuery(rest)
		mr, err := MatchPath(idx, verb, rawPath)
		if err != nil {
			return "", nil, "", err
		}
		return verb, mr, rawQuery, nil
	}

	rawPath, rawQuery := splitPathQuery(arg)
	if looksLikeOperationID(rawPath) {
		mr, err := MatchByOperationID(idx, rawPath)
		if err != nil {
			return "", nil, "", err
		}
		if methodExplicit && strings.ToUpper(api.method) != mr.Op.Method {
			return "", nil, "", NewAPIError(cmdutil.ExitConfigurationError, ErrInvalidFlags,
				fmt.Sprintf("operation %s is %s; --method=%s disagrees",
					mr.Op.OperationID, mr.Op.Method, api.method)).
				WithField("method")
		}
		return mr.Op.Method, mr, rawQuery, nil
	}

	// Path form — defaults to GET, or POST when body fields are present
	// AND the spec has a POST variant. Some GET endpoints accept `-F` as
	// query params, so we only auto-switch when the spec actually pairs
	// this path with POST.
	method := strings.ToUpper(api.method)
	if method == "" &&
		methodDefaultsToPost(len(api.fields)+len(api.rawFields), api.input != "", api.body != "") {
		if mr, err := MatchPath(idx, "POST", rawPath); err == nil {
			return "POST", mr, rawQuery, nil
		}
		method = "GET"
	}
	if method == "" {
		method = "GET"
	}
	mr, err := MatchPath(idx, method, rawPath)
	if err != nil {
		return "", nil, "", err
	}
	return method, mr, rawQuery, nil
}

// resolveBindings resolves every path-parameter binding to a concrete string.
// Precedence per var: path literal > matching -F/-f field (consumed) > context
// auto-resolution from the current Pulumi project / selected stack.
//
// When the user wrote a placeholder alias in the path (e.g. `{foo}` for a
// spec param named `orgName`), the alias is the lookup key for the matching
// -F field — Otherwise the spec param name is the lookup key.
//
// A field whose key matches a path template variable is consumed — it is
// removed from the returned slice so encodeFields won't re-route it to the
// query string or request body.
func resolveBindings(
	mr *MatchResult, fields []ParsedField, rctx *ResolvedContext,
) (map[string]string, []ParsedField, error) {
	out := make(map[string]string, len(mr.Bindings))
	remaining := fields
	ctxValues := contextPathValues(rctx)
	// Iterate bindings in a deterministic order so error messages and the
	// remaining-fields slice stay stable across runs.
	for name, b := range fxmaps.Sorted(mr.Bindings) {
		if b.Literal != "" {
			out[name] = b.Literal
			continue
		}
		lookupKey := name
		if b.Placeholder != "" {
			lookupKey = b.Placeholder
		}
		idx := findFieldIndex(remaining, lookupKey)
		if idx >= 0 {
			val, err := stringifyFieldForPath(remaining[idx])
			if err != nil {
				return nil, nil, err
			}
			out[name] = val
			remaining = append(remaining[:idx], remaining[idx+1:]...)
			continue
		}
		if v, ok := ctxValues[lookupKey]; ok {
			out[name] = v
			continue
		}
		return nil, nil, NewAPIError(cmdutil.ExitCodeError, ErrMissingContext,
			"template var {"+name+"} is unresolved").
			WithField(name).
			WithSuggestions(
				"pass -F "+lookupKey+"=<value>",
				"or include the value literally in the path",
			)
	}
	return out, remaining, nil
}

// contextPathValues pre-fills the path values that the dispatcher can derive
// from the current Pulumi context (selected stack, project file, default
// org). Keyed by the OpenAPI spec name only. Returns nil when rctx is nil.
func contextPathValues(rctx *ResolvedContext) map[string]string {
	if rctx == nil {
		return nil
	}
	out := make(map[string]string, 3)
	switch {
	case rctx.StackOrg != "":
		out["orgName"] = rctx.StackOrg
	case rctx.OrgName != "":
		out["orgName"] = rctx.OrgName
	}
	switch {
	case rctx.Project != nil && rctx.Project.Name != "":
		out["projectName"] = string(rctx.Project.Name)
	case rctx.StackProj != "":
		out["projectName"] = rctx.StackProj
	}
	if rctx.StackName != "" {
		out["stackName"] = rctx.StackName
	}
	return out
}

// findFieldIndex returns the index of the first field whose key matches name,
// or -1 if none.
func findFieldIndex(fields []ParsedField, name string) int {
	for i := range fields {
		if fields[i].Key == name {
			return i
		}
	}
	return -1
}

// stringifyFieldForPath converts a ParsedField value to the plain-text form
// used for path substitution. Null values are rejected — a null would either
// collapse the segment to the empty string (breaking routing) or serialize
// as the literal "null", neither of which is what the user meant. The
// Pulumi Cloud OpenAPI spec only declares string and integer path params;
// other types fall through the default and stringify with %v.
func stringifyFieldForPath(f ParsedField) (string, error) {
	switch v := f.Value.(type) {
	case string:
		return v, nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case nil:
		return "", NewAPIError(cmdutil.ExitCodeError, ErrInvalidFlags,
			fmt.Sprintf("field %q is null; path parameters cannot be null", f.Key)).
			WithField(f.Key)
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

// buildConcretePath substitutes resolved values into an OpenAPI path template,
// URL-escaping each parameter. A handful of param names (see encoding.go)
// require double-escaping because their server-side handlers decode twice.
func buildConcretePath(op *Operation, resolved map[string]string) string {
	segs := splitSegments(op.Path)
	for i, seg := range segs {
		if !isTemplateSegment(seg) {
			continue
		}
		name := trimBraces(seg)
		segs[i] = escapePathParam(resolved[name], requiresDoubleEncoding(name))
	}
	return strings.Join(segs, "/")
}

// encodeFields splits parsed -F / -f values into a body blob (JSON-encoded
// when fields present, file bytes when --input, raw when --body), plus extra
// query parameters for GET/HEAD. Returns body, queryExtras, contentType.
//
// For --input, the content type is chosen in order of precedence:
//  1. file extension (.yaml/.yml → application/x-yaml, .json → application/json)
//     so a user can force a shape when the spec accepts more than one.
//  2. op.BodyContentType when the spec pins one (e.g. ESC UpdateEnvironment is
//     application/x-yaml only).
//  3. application/json as the final default.
//
// --body sends the string verbatim with application/json as the default
// Content-Type. A user-supplied -H 'Content-Type: …' wins over all of the
// above at the transport layer (buildAPIHeaders applies user headers after
// encoder defaults).
func encodeFields(
	method string, fields []ParsedField, api *apiCommand, op *Operation,
) ([]byte, url.Values, string, error) {
	extras := url.Values{}

	if api.input != "" {
		raw, err := readAtSource(api.input, os.Stdin)
		if err != nil {
			return nil, nil, "", NewAPIError(cmdutil.ExitCodeError, ErrInvalidFlags,
				fmt.Sprintf("reading --input: %v", err)).WithField("input")
		}
		// Matches gh: when --input supplies the body, fields (if any) become query params.
		for _, f := range fields {
			extras.Add(f.Key, fieldToQueryString(f))
		}
		return raw, extras, chooseInputContentType(api.input, op), nil
	}

	if api.body != "" {
		// Same rule as --input: fields alongside --body become query params.
		for _, f := range fields {
			extras.Add(f.Key, fieldToQueryString(f))
		}
		return []byte(api.body), extras, "application/json", nil
	}

	isBodyless := method == "GET" || method == "HEAD"
	if isBodyless {
		for _, f := range fields {
			extras.Add(f.Key, fieldToQueryString(f))
		}
		return nil, extras, "", nil
	}

	if len(fields) == 0 {
		return nil, extras, "", nil
	}
	obj := make(map[string]any, len(fields))
	for _, f := range fields {
		if existing, ok := obj[f.Key]; ok {
			// Repeated key collapses into an array, preserving order.
			switch prev := existing.(type) {
			case []any:
				obj[f.Key] = append(prev, f.Value)
			default:
				obj[f.Key] = []any{prev, f.Value}
			}
		} else {
			obj[f.Key] = f.Value
		}
	}
	body, err := json.Marshal(obj)
	if err != nil {
		return nil, nil, "", NewAPIError(cmdutil.ExitInternalError, ErrToolError,
			fmt.Sprintf("marshaling body: %v", err))
	}
	return body, extras, "application/json", nil
}

// chooseInputContentType picks the Content-Type for --input data.
// Precedence: explicit file extension → op's declared BodyContentType → JSON.
// Stdin (@-) and @path sources without a recognised extension fall through.
func chooseInputContentType(input string, op *Operation) string {
	// @path syntax is handled by readAtSource; strip the @ for extension sniffing.
	source := strings.TrimPrefix(input, "@")
	switch strings.ToLower(filepath.Ext(source)) {
	case ".yaml", ".yml":
		return "application/x-yaml"
	case ".json":
		return "application/json"
	}
	if op != nil && op.BodyContentType != "" {
		return op.BodyContentType
	}
	return "application/json"
}

// fieldToQueryString serializes a ParsedField's value for use in a query string.
// Nested objects/arrays (from JSON-literal -F values) serialize back to JSON so
// the result is round-trippable on the server side.
func fieldToQueryString(f ParsedField) string {
	switch v := f.Value.(type) {
	case string:
		return v
	case bool:
		if v {
			return "true"
		}
		return "false"
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case nil:
		return ""
	default:
		if b, err := json.Marshal(v); err == nil {
			return string(b)
		}
		return fmt.Sprint(v)
	}
}

// mergeQuery combines a raw user-provided query string with additional values
// from -F / -f. The raw string is preserved verbatim here. The live-request
// path (url.ParseQuery → url.Values.Encode in Client.RawCall) canonicalizes
// key order and percent-encoding on the wire; --dry-run prints the raw string
// unchanged.
func mergeQuery(raw string, extras url.Values) string {
	extrasEnc := extras.Encode()
	switch {
	case raw == "" && extrasEnc == "":
		return ""
	case raw == "":
		return extrasEnc
	case extrasEnc == "":
		return raw
	default:
		return raw + "&" + extrasEnc
	}
}

// emitDryRun renders the resolved request as JSON on stdout without sending
// it. Template vars and field coercion are resolved, but no network call
// (or server-side validation) has happened.
func emitDryRun(
	w io.Writer, method, fullURL string,
	hdrs []ParsedHeader, contentType, accept string, body []byte,
) error {
	if accept == "" {
		accept = "application/json"
	}
	headers := map[string]string{
		"Authorization": "token ***",
		"Accept":        accept + ", application/vnd.pulumi+8",
		"User-Agent":    client.UserAgent(),
	}
	if contentType != "" {
		headers["Content-Type"] = contentType
	}
	for _, h := range hdrs {
		headers[h.Name] = h.Value
	}

	var bodyRaw json.RawMessage
	if len(body) > 0 {
		if json.Valid(body) {
			bodyRaw = json.RawMessage(body)
		} else {
			quoted, _ := json.Marshal(string(body))
			bodyRaw = json.RawMessage(quoted)
		}
	}

	env := dryRunEnvelope{SchemaVersion: SchemaVersion}
	env.DryRun.Plan = DryRunPlan{
		Method:  method,
		URL:     fullURL,
		Headers: headers,
		Body:    bodyRaw,
	}

	return WriteJSON(w, env, cmdutil.Interactive())
}
