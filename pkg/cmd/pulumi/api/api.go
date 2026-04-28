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

package api

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

	"github.com/spf13/cobra"

	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate"
	"github.com/pulumi/pulumi/pkg/v3/backend/httpstate/client"
	pkgWorkspace "github.com/pulumi/pulumi/pkg/v3/workspace"
	"github.com/pulumi/pulumi/sdk/v3/go/common/util/cmdutil"
)

// apiFlags holds all flags accepted by `pulumi cloud api <path>`.
// Defined as a struct so the flag contract lives in one place.
type apiFlags struct {
	method        string
	fields        []string
	rawFields     []string
	headers       []string
	input         string
	body          string
	jq            string
	paginate      bool
	emitEvents    bool
	include       bool
	silent        bool
	verbose       bool
	dryRun        bool
	interactive   bool
	noInteractive bool
	output        string
	org           string
	project       string
	stack         string
	schemaVersion int
}

// bindAPIFlags attaches the full flag set to cmd and returns a pointer to the
// parsed values. Flag names are a stable contract matching `gh api`.
func bindAPIFlags(cmd *cobra.Command) *apiFlags {
	f := &apiFlags{schemaVersion: SchemaVersion}
	pf := cmd.Flags()

	pf.StringVarP(&f.method, "method", "X", "",
		"HTTP method (default GET, POST when body fields are present)")
	pf.StringArrayVarP(&f.fields, "field", "F", nil,
		"Typed key=value; numbers/bools/null auto-detected; JSON object/array "+
			"literals parsed; @file reads file; @- reads stdin. "+
			"Sent as query params on GET/HEAD, JSON body fields otherwise")
	pf.StringArrayVarP(&f.rawFields, "raw-field", "f", nil,
		"String key=value with no type coercion. "+
			"Sent as query params on GET/HEAD, JSON body fields otherwise")
	pf.StringArrayVarP(&f.headers, "header", "H", nil,
		"Custom HTTP header `Key: Value` (repeatable)")
	pf.StringVar(&f.input, "input", "",
		"Read request body from file; `-` reads stdin")
	pf.StringVar(&f.body, "body", "",
		"Inline request body sent verbatim (default Content-Type: application/json). "+
			"Mutually exclusive with --input")
	pf.StringVar(&f.jq, "jq", "",
		"Filter JSON response with a jq expression")
	pf.BoolVar(&f.paginate, "paginate", false,
		"Follow pagination cursors and emit the combined result")
	pf.BoolVar(&f.emitEvents, "emit-events", false,
		"Emit JSONL progress events to stderr during --paginate runs "+
			"(page, complete, partial_failure, truncated, cancelled). Hidden by default "+
			"so 2>&1 piped to jq doesn't mix progress lines with response data.")
	pf.BoolVarP(&f.include, "include", "i", false,
		"Include HTTP status line and response headers in output")
	pf.BoolVar(&f.silent, "silent", false,
		"Suppress body output; exit code carries success")
	pf.BoolVar(&f.verbose, "verbose", false,
		"Dump full request and response to stderr")
	pf.BoolVar(&f.dryRun, "dry-run", false,
		"Print the resolved request without sending it")
	pf.BoolVar(&f.interactive, "interactive", false,
		"Force the interactive endpoint picker even when stdin/stdout are not a TTY")
	pf.BoolVar(&f.noInteractive, "no-interactive", false,
		"Never prompt; error when required input is missing")
	pf.StringVar(&f.output, "output", "",
		"Drive content negotiation and rendering. Default uses the op's primary "+
			"response content type (usually JSON). `json` or `markdown` request that "+
			"format via the Accept header — rejected if the op's spec doesn't declare it. "+
			"`raw` keeps the op's default Accept and writes the body through unchanged.")
	pf.StringVar(&f.org, "org", "",
		"Override {orgName} / {org} template variable")
	pf.StringVar(&f.project, "project", "",
		"Override {projectName} / {project} template variable")
	pf.StringVar(&f.stack, "stack", "",
		"Override {stackName} / {stack} template variable")
	pf.IntVar(&f.schemaVersion, "schema-version", SchemaVersion,
		"Pin the envelope schema version the caller expects")

	return f
}

func newAPICmd() *cobra.Command {
	var flags *apiFlags

	cmd := &cobra.Command{
		Use:   "api [<path-or-operation-id>]",
		Short: "Call any Pulumi Cloud API endpoint",
		Long: "Call any Pulumi Cloud API endpoint.\n\n" +
			"The positional argument may be: a path (with optional {template}\n" +
			"variables, e.g. `/api/orgs/{orgName}/members`), an operation ID as\n" +
			"shown in `ls` (e.g. `ListOrgMembers`), or a paste-friendly row\n" +
			"(`GET /api/...`). Template variables are resolved from the current\n" +
			"Pulumi project or from --org / --project / --stack.\n\n" +
			"Fields provided via -F/--field and -f/--raw-field are sent as query\n" +
			"parameters on GET/HEAD requests and as a JSON request body on POST/PUT/\n" +
			"PATCH/DELETE. Method defaults to GET, or POST when body fields, --body,\n" +
			"or --input are present.\n\n" +
			"Value forms accepted by -F:\n" +
			"  - scalars: `-F name=acme`, `-F count=3`, `-F enabled=true`, `-F note=null`\n" +
			"  - nested JSON: `-F 'labels={\"env\":\"prod\"}'`, `-F 'tags=[\"a\",\"b\"]'`\n" +
			"  - file / stdin: `-F body=@./payload.json`, `-F note=@-`\n" +
			"Use -f/--raw-field to suppress coercion and send the value verbatim as a string.\n\n" +
			"For an entire request body, pass --body '<json>' inline, or --input <file>\n" +
			"(use `-` for stdin) to stream from a file.\n\n" +
			"A field whose key matches a path template variable (e.g. `-F poolId=123`\n" +
			"against `{poolId}`) fills that variable and is not forwarded as a query\n" +
			"or body parameter. This is the way to supply non-context path parameters\n" +
			"when calling by operation ID.\n\n" +
			"Run `pulumi cloud api ls` to list available endpoints and\n" +
			"`pulumi cloud api describe <path-or-operation-id>` to inspect one.\n\n" +
			"Exit codes: 0 success; 1 caller error; 2 invalid flags; 3 auth; 8 cancelled;\n" +
			"255 internal.",
		Example: "  # Inspect the currently authenticated user.\n" +
			"  pulumi cloud api /api/user\n\n" +
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
			"  # Follow pagination cursors and stream the combined result to jq.\n" +
			"  pulumi cloud api ListStacks --paginate | jq '.stacks[].name'\n\n" +
			"  # Filter the JSON response without leaving the command.\n" +
			"  pulumi cloud api /api/user --jq '.githubLogin'\n\n" +
			"  # Extract just the status line + headers without the body.\n" +
			"  pulumi cloud api /api/user --include --silent\n\n" +
			"  # Preview the resolved request without sending it.\n" +
			"  pulumi cloud api CreateStackTag -F orgName=acme --dry-run",
		Args:          cobra.MaximumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	flags = bindAPIFlags(cmd)

	cmd.RunE = runWithEnvelope(func(cmd *cobra.Command, args []string) error {
		return runAPI(cmd, args, flags)
	})

	cmd.PersistentFlags().Bool(refreshSpecFlagName, false,
		"Re-fetch the OpenAPI spec from Pulumi Cloud and overwrite the local cache")

	cmd.AddCommand(newLsCmd())
	cmd.AddCommand(newDescribeCmd())

	return cmd
}

func runAPI(cmd *cobra.Command, args []string, flags *apiFlags) error {
	if err := validateFlagCombos(flags); err != nil {
		return err
	}

	if len(args) == 0 {
		if !resolveInteractivity(flags.interactive, flags.noInteractive) {
			return NewAPIError(cmdutil.ExitCodeError, ErrRequiredInputMissing,
				"no endpoint provided").
				WithField("path").
				WithSuggestions(
					"pass a path, e.g. `pulumi cloud api /api/user`",
					"or an operation ID, e.g. `pulumi cloud api ListAccounts`",
					"use --interactive to launch the endpoint picker",
					"run `pulumi cloud api ls` to see available endpoints",
					"run `pulumi cloud api describe <path-or-operation-id>` to inspect one",
				)
		}
		return runInteractive(cmd, flags)
	}
	userArg := strings.TrimSpace(args[0])

	idx, err := LoadIndex(cmd.Context(), refreshSpecFlag(cmd))
	if err != nil {
		return err
	}

	methodExplicit := cmd.Flags().Changed("method")
	method, mr, rawQuery, err := resolveAPIArg(idx, userArg, flags, methodExplicit)
	if err != nil {
		return err
	}

	fields, err := parseFields(flags.fields, flags.rawFields, os.Stdin)
	if err != nil {
		return err
	}

	resolved, fields, err := resolveBindings(mr, flags, fields)
	if err != nil {
		return err
	}

	concretePath := buildConcretePath(mr.Op, resolved)

	hdrs, err := parseHeaders(flags.headers)
	if err != nil {
		return err
	}

	bodyBytes, queryExtras, contentType, err := encodeFields(method, fields, flags, mr.Op)
	if err != nil {
		return err
	}

	query := mergeQuery(rawQuery, queryExtras)

	accept, err := negotiateAccept(mr.Op, flags.output)
	if err != nil {
		return err
	}

	if flags.dryRun {
		cloudURL := httpstate.ValueOrDefaultURL(pkgWorkspace.Instance, "")
		fullURL := strings.TrimRight(cloudURL, "/") + concretePath
		if query != "" {
			fullURL += "?" + query
		}
		return emitDryRun(method, fullURL, hdrs, contentType, accept, bodyBytes)
	}

	return executeLive(cmd.Context(), mr.Op, method, concretePath, query, hdrs,
		contentType, accept, bodyBytes, flags)
}

// executeLive resolves auth, issues the HTTP request, and post-processes the
// response according to the flag set (--include, --silent, --jq, --verbose).
// accept is the Accept header value chosen by negotiateAccept upstream.
func executeLive(
	ctx context.Context,
	op *Operation,
	method, concretePath, query string,
	hdrs []ParsedHeader,
	contentType, accept string,
	body []byte,
	flags *apiFlags,
) error {
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	resolved, err := ResolveContext(ctx, flags.org, false)
	if err != nil {
		return NewAPIError(cmdutil.ExitAuthenticationError, ErrNotLoggedIn, err.Error()).
			WithSuggestions("run `pulumi login` first")
	}

	apiClient := NewAPIClient(resolved.CloudURL, resolved.Token)

	if flags.verbose {
		dumpRequestVerbose(method, strings.TrimRight(resolved.CloudURL, "/")+concretePath+queryFragment(query),
			hdrs, contentType, accept, body)
	}

	baseQuery, err := url.ParseQuery(query)
	if err != nil {
		return NewAPIError(cmdutil.ExitCodeError, ErrInvalidFlags,
			fmt.Sprintf("parsing query string %q: %v", query, err)).WithField("path")
	}

	if flags.paginate {
		return runPaginate(ctx, apiClient, paginateRequest{
			Method:      method,
			Path:        concretePath,
			BaseQuery:   baseQuery,
			Body:        body,
			ContentType: contentType,
			Accept:      accept,
			Headers:     hdrs,
			Op:          op,
		}, flags)
	}

	var reqBody io.Reader
	if len(body) > 0 {
		reqBody = bytes.NewReader(body)
	}

	resp, err := apiClient.Do(ctx, method, concretePath, baseQuery, reqBody, contentType, accept, hdrs)
	if err != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			ev := NewEvent("cancelled")
			ev.Phase = "request"
			maybeWriteStderrEvent(flags.emitEvents, ev)
			return &APIError{ExitCode: cmdutil.ExitCancelled, Silent: true}
		}
		return NewAPIError(cmdutil.ExitCodeError, ErrNetwork,
			fmt.Sprintf("HTTP %s %s: %v", method, concretePath, err))
	}
	defer resp.Body.Close()

	return handleResponse(resp, flags)
}

// queryFragment returns "?<query>" when query is non-empty, else "".
func queryFragment(query string) string {
	if query == "" {
		return ""
	}
	return "?" + query
}

// dumpRequestVerbose writes the full outgoing request to stderr for --verbose.
// Authorization is redacted; user-supplied Authorization overrides are dropped
// to match apiClient.Do, which also drops them.
func dumpRequestVerbose(method, fullURL string, hdrs []ParsedHeader, contentType, accept string, body []byte) {
	fmt.Fprintf(os.Stderr, "> %s %s\n", method, fullURL)
	fmt.Fprintf(os.Stderr, "> Authorization: token ***\n")
	fmt.Fprintf(os.Stderr, "> Accept: %s, application/vnd.pulumi+8\n", accept)
	if contentType != "" {
		fmt.Fprintf(os.Stderr, "> Content-Type: %s\n", contentType)
	}
	for _, h := range hdrs {
		if strings.EqualFold(h.Name, "Authorization") {
			continue
		}
		fmt.Fprintf(os.Stderr, "> %s: %s\n", h.Name, h.Value)
	}
	if len(body) > 0 {
		fmt.Fprintln(os.Stderr, ">")
		os.Stderr.Write(body)
		fmt.Fprintln(os.Stderr)
	}
}

// handleResponse applies post-processing flags (--include, --silent, --jq)
// and renders the body via the content-type router.
func handleResponse(resp *http.Response, flags *apiFlags) error {
	if flags.include {
		writeStatusAndHeaders(os.Stdout, resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return NewAPIError(cmdutil.ExitCodeError, ErrNetwork,
			fmt.Sprintf("reading response body: %v", err))
	}

	if flags.verbose {
		writeStatusAndHeaders(os.Stderr, resp)
		if len(body) > 0 {
			fmt.Fprintln(os.Stderr, "<")
			os.Stderr.Write(body)
			fmt.Fprintln(os.Stderr)
		}
	}

	if resp.StatusCode >= 400 {
		return httpErrorEnvelopeBytes(resp, body)
	}

	if flags.silent {
		return nil
	}
	if len(body) == 0 {
		return nil
	}

	if flags.jq != "" {
		if !isJSONContentType(resp.Header.Get("Content-Type")) {
			fmt.Fprintln(os.Stderr, "warning: --jq on non-JSON response; passing through raw")
			_, err := os.Stdout.Write(body)
			return err
		}
		return ApplyJQ(os.Stdout, body, flags.jq)
	}

	return renderBody(resp.Header.Get("Content-Type"), body)
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
// user's --output flag. When --output is unset we use the op's primary
// response content type (historically JSON for Pulumi Cloud). When the user
// explicitly asks for JSON or markdown we validate against the op's declared
// response content types so the call fails fast with a helpful message
// instead of surprising the caller with a 406 from the server.
func negotiateAccept(op *Operation, output string) (string, error) {
	switch strings.ToLower(output) {
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
		return "", unsupportedOutputError(op, "json", "application/json")
	case "markdown", "md":
		for _, ct := range op.SuccessContentTypes {
			if strings.EqualFold(ct, "text/markdown") {
				return ct, nil
			}
		}
		return "", unsupportedOutputError(op, "markdown", "text/markdown")
	default:
		return "", NewAPIError(cmdutil.ExitCodeError, ErrInvalidFlags,
			"invalid --output value: "+output).
			WithField("output").
			WithSuggestions("--output=json", "--output=markdown", "--output=raw")
	}
}

// unsupportedOutputError returns a caller-actionable error when the user
// asks for a format the op doesn't declare. The suggestions surface the
// content types the op does declare so the user can pick one that works.
func unsupportedOutputError(op *Operation, want, mediaType string) error {
	msg := fmt.Sprintf("operation %s does not declare a %s response", op.OperationID, mediaType)
	suggestions := []string{"omit --output to use the op's default content type"}
	if len(op.SuccessContentTypes) > 0 {
		suggestions = append(suggestions,
			"declared response content types: "+strings.Join(op.SuccessContentTypes, ", "))
	}
	return NewAPIError(cmdutil.ExitCodeError, ErrInvalidFlags, msg).
		WithField("output").
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

// renderBody writes body to stdout, pretty-printing JSON when the content
// type indicates JSON, passing through text/binary otherwise. A thin
// adapter over format.go's existing helpers.
func renderBody(contentType string, body []byte) error {
	ct := strings.ToLower(contentType)
	switch {
	case isJSONContentType(contentType), ct == "":
		return formatJSON(body)
	case strings.Contains(ct, "application/x-yaml"),
		strings.Contains(ct, "text/plain"),
		strings.Contains(ct, "text/markdown"):
		return formatText(body)
	case strings.Contains(ct, "application/x-tar"),
		strings.Contains(ct, "application/octet-stream"):
		return formatBinary(body)
	default:
		return formatJSON(body)
	}
}

// validateFlagCombos enforces flag-set invariants before we do any work.
func validateFlagCombos(flags *apiFlags) error {
	if flags.schemaVersion != SchemaVersion {
		return NewAPIError(cmdutil.ExitCodeError, ErrUnsupportedSchemaVersion,
			fmt.Sprintf("schemaVersion %d is not supported; this CLI speaks schemaVersion %d",
				flags.schemaVersion, SchemaVersion)).
			WithField("schema-version").
			WithSuggestions("omit --schema-version to use the current version")
	}
	if flags.body != "" && flags.input != "" {
		return NewAPIError(cmdutil.ExitConfigurationError, ErrInvalidFlags,
			"--body and --input are mutually exclusive").
			WithField("body").
			WithSuggestions(
				"use --body for inline JSON",
				"use --input to read the body from a file",
			)
	}
	return nil
}

// resolveAPIArg maps the user's positional argument to a concrete operation.
// Accepts three forms: a path with optional {template} params, an operation
// ID (e.g. "ListAccounts"), or a paste-friendly "METHOD /path" row from
// `ls`. methodExplicit signals whether --method was set; when true and the
// argument pins a different method, a conflict error is returned.
func resolveAPIArg(idx *Index, arg string, flags *apiFlags, methodExplicit bool) (string, *MatchResult, string, error) {
	if verb, rest, ok := splitLeadingHTTPMethod(arg); ok {
		if methodExplicit && strings.ToUpper(flags.method) != verb {
			return "", nil, "", NewAPIError(cmdutil.ExitConfigurationError, ErrInvalidFlags,
				fmt.Sprintf("conflicting methods: argument has %q, --method is %q", verb, flags.method)).
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
		if methodExplicit && strings.ToUpper(flags.method) != mr.Op.Method {
			return "", nil, "", NewAPIError(cmdutil.ExitConfigurationError, ErrInvalidFlags,
				fmt.Sprintf("operation %s is %s; --method=%s disagrees",
					mr.Op.OperationID, mr.Op.Method, flags.method)).
				WithField("method")
		}
		mr.Bindings = bindingsForTemplate(mr.Op.Path)
		return mr.Op.Method, mr, rawQuery, nil
	}

	// Path form — defaults to GET, or POST when body fields are present
	// AND the spec has a POST variant. Some GET endpoints accept `-F` as
	// query params, so we only auto-switch when the spec actually pairs
	// this path with POST.
	method := strings.ToUpper(flags.method)
	if method == "" {
		if methodDefaultsToPost(len(flags.fields)+len(flags.rawFields), flags.input != "", flags.body != "") &&
			hasMethod(idx, rawPath, "POST") {
			method = "POST"
		} else {
			method = "GET"
		}
	}
	mr, err := MatchPath(idx, method, rawPath)
	if err != nil {
		return "", nil, "", err
	}
	return method, mr, rawQuery, nil
}

// bindingsForTemplate synthesizes placeholder-style bindings for each
// {param} in specPath — the form resolveBindings feeds into --org /
// context resolution. Used when an operation is looked up by ID rather
// than by a user-supplied concrete path.
func bindingsForTemplate(specPath string) map[string]Binding {
	bindings := map[string]Binding{}
	for _, seg := range splitSegments(specPath) {
		if isTemplateSegment(seg) {
			name := trimBraces(seg)
			bindings[name] = Binding{Placeholder: name}
		}
	}
	return bindings
}

// hasMethod reports whether the spec defines the given method for a path
// (literal or template match). Used to guard the "-F → POST" default so we
// don't silently route to a method the endpoint doesn't support.
func hasMethod(idx *Index, userPath, method string) bool {
	_, err := MatchPath(idx, method, userPath)
	return err == nil
}

// splitPathQuery separates the query string from a user-provided path.
func splitPathQuery(userPath string) (string, string) {
	if idx := strings.IndexByte(userPath, '?'); idx >= 0 {
		return userPath[:idx], userPath[idx+1:]
	}
	return userPath, ""
}

// resolveBindings resolves every path-parameter binding to a concrete string.
// Precedence per var: path literal > matching -F/-f field (consumed) > context
// flag (--org / --project / --stack) > context auto-resolution.
//
// A field whose key matches a path template variable is consumed — it is
// removed from the returned slice so encodeFields won't re-route it to the
// query string or request body.
//
// Supplying both a context flag and a -F field for the same path var is a
// user error and returns ErrInvalidFlags.
func resolveBindings(mr *MatchResult, flags *apiFlags, fields []ParsedField) (map[string]string, []ParsedField, error) {
	out := make(map[string]string, len(mr.Bindings))
	remaining := fields
	for name, b := range mr.Bindings {
		if b.Literal != "" {
			out[name] = b.Literal
			continue
		}
		idx := findFieldIndex(remaining, name)
		if idx >= 0 {
			if err := checkContextFlagFieldConflict(name, flags); err != nil {
				return nil, nil, err
			}
			val, err := stringifyFieldForPath(remaining[idx])
			if err != nil {
				return nil, nil, err
			}
			out[name] = val
			remaining = append(remaining[:idx], remaining[idx+1:]...)
			continue
		}
		val, err := resolveTemplateVar(name, b.Placeholder, flags)
		if err != nil {
			return nil, nil, err
		}
		out[name] = val
	}
	return out, remaining, nil
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

// checkContextFlagFieldConflict returns ErrInvalidFlags if name is a context-
// kind template var AND the caller supplied both the corresponding context
// flag and a -F field for it.
func checkContextFlagFieldConflict(name string, flags *apiFlags) error {
	var flagName, flagVal string
	switch templateVarKind(name) {
	case kindOrg:
		flagName, flagVal = "--org", flags.org
	case kindProject:
		flagName, flagVal = "--project", flags.project
	case kindStack:
		flagName, flagVal = "--stack", flags.stack
	case kindNone:
		return nil
	default:
		return nil
	}
	if flagVal == "" {
		return nil
	}
	return NewAPIError(cmdutil.ExitConfigurationError, ErrInvalidFlags,
		fmt.Sprintf("%s and -F %s were both supplied; remove one", flagName, name)).
		WithField(strings.TrimPrefix(flagName, "--"))
}

// stringifyFieldForPath converts a ParsedField value to the plain-text form
// used for path substitution. Null values are rejected — a null would either
// collapse the segment to the empty string (breaking routing) or serialize
// as the literal "null", neither of which is what the user meant.
func stringifyFieldForPath(f ParsedField) (string, error) {
	switch v := f.Value.(type) {
	case string:
		return v, nil
	case bool:
		if v {
			return "true", nil
		}
		return "false", nil
	case int64:
		return strconv.FormatInt(v, 10), nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	case nil:
		return "", NewAPIError(cmdutil.ExitCodeError, ErrInvalidFlags,
			fmt.Sprintf("field %q is null; path parameters cannot be null", f.Key)).
			WithField(f.Key)
	default:
		return fmt.Sprintf("%v", v), nil
	}
}

type templateKind int

const (
	kindNone templateKind = iota
	kindOrg
	kindProject
	kindStack
)

func templateVarKind(name string) templateKind {
	switch name {
	case "orgName", "org":
		return kindOrg
	case "projectName", "project":
		return kindProject
	case "stackName", "stack":
		return kindStack
	}
	return kindNone
}

// resolveTemplateVar resolves a single unbound template variable. Precedence:
// explicit flag > currently-selected stack's qualified ref > project file /
// default-org fallback.
func resolveTemplateVar(specName, alias string, flags *apiFlags) (string, error) {
	kind := templateVarKind(specName)
	if kind == kindNone {
		kind = templateVarKind(alias)
	}
	stackOrg, stackProj, stackName := currentStackSelection()

	switch kind {
	case kindOrg:
		if flags.org != "" {
			return flags.org, nil
		}
		if stackOrg != "" {
			return stackOrg, nil
		}
		if org, err := pkgWorkspace.GetBackendConfigDefaultOrg(nil); err == nil && org != "" {
			return org, nil
		}
		return "", NewAPIError(cmdutil.ExitCodeError, ErrMissingContext,
			"template var {"+specName+"} is unresolved").
			WithField(specName).
			WithSuggestions(
				"pass --org <name>",
				"select a stack with `pulumi stack select <org>/<project>/<stack>`",
				"set a default with `pulumi org set-default <name>`",
			)
	case kindProject:
		if flags.project != "" {
			return flags.project, nil
		}
		if proj, _, err := pkgWorkspace.Instance.ReadProject(); err == nil && proj != nil && proj.Name != "" {
			return string(proj.Name), nil
		}
		if stackProj != "" {
			return stackProj, nil
		}
		return "", NewAPIError(cmdutil.ExitCodeError, ErrMissingContext,
			"template var {"+specName+"} is unresolved").
			WithField(specName).
			WithSuggestions(
				"pass --project <name>",
				"run from a directory containing Pulumi.yaml",
			)
	case kindStack:
		if flags.stack != "" {
			return flags.stack, nil
		}
		if stackName != "" {
			return stackName, nil
		}
		return "", NewAPIError(cmdutil.ExitCodeError, ErrMissingContext,
			"template var {"+specName+"} is unresolved").
			WithField(specName).
			WithSuggestions(
				"pass --stack <name>",
				"select a stack with `pulumi stack select <name>` first",
			)
	case kindNone:
		fallthrough
	default:
		return "", NewAPIError(cmdutil.ExitCodeError, ErrMissingContext,
			"template var {"+specName+"} is unresolved").
			WithField(specName).
			WithSuggestions(
				"pass -F "+specName+"=<value>",
				"include the value literally in the path",
			)
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
// above at the transport layer (APIClient.Do applies user headers after
// encoder defaults).
func encodeFields(
	method string, fields []ParsedField, flags *apiFlags, op *Operation,
) ([]byte, url.Values, string, error) {
	extras := url.Values{}

	if flags.input != "" {
		raw, err := readAtSource(flags.input, os.Stdin)
		if err != nil {
			return nil, nil, "", NewAPIError(cmdutil.ExitCodeError, ErrInvalidFlags,
				fmt.Sprintf("reading --input: %v", err)).WithField("input")
		}
		// Matches gh: when --input supplies the body, fields (if any) become query params.
		for _, f := range fields {
			extras.Add(f.Key, fieldToQueryString(f))
		}
		return raw, extras, chooseInputContentType(flags.input, op), nil
	}

	if flags.body != "" {
		// Same rule as --input: fields alongside --body become query params.
		for _, f := range fields {
			extras.Add(f.Key, fieldToQueryString(f))
		}
		return []byte(flags.body), extras, "application/json", nil
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
// path (url.ParseQuery → url.Values.Encode in APIClient.Do) canonicalizes key
// order and percent-encoding on the wire; --dry-run prints the raw string
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
func emitDryRun(method, fullURL string, hdrs []ParsedHeader, contentType, accept string, body []byte) error {
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

	return WriteJSON(os.Stdout, env, stdoutIsTTY())
}
