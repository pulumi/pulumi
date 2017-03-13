// Copyright 2016 Pulumi, Inc. All rights reserved.

package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/pulumi/coconut/pkg/compiler"
	"github.com/pulumi/coconut/pkg/compiler/core"
	"github.com/pulumi/coconut/pkg/compiler/errors"
	"github.com/pulumi/coconut/pkg/compiler/symbols"
	"github.com/pulumi/coconut/pkg/diag"
	"github.com/pulumi/coconut/pkg/diag/colors"
	"github.com/pulumi/coconut/pkg/encoding"
	"github.com/pulumi/coconut/pkg/eval/heapstate"
	"github.com/pulumi/coconut/pkg/eval/rt"
	"github.com/pulumi/coconut/pkg/pack"
	"github.com/pulumi/coconut/pkg/resource"
	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/pkg/util/cmdutil"
	"github.com/pulumi/coconut/pkg/util/contract"
	"github.com/pulumi/coconut/pkg/util/mapper"
	"github.com/pulumi/coconut/pkg/workspace"
)

func newEnvCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "env",
		Short: "Manage target environments",
		Long: "Manage target environments\n" +
			"\n" +
			"An environment is a named deployment target, and a single project may have many of them.\n" +
			"Each environment has a configuration and deployment history associated with it, stored in\n" +
			"the workspace, in addition to a full checkpoint of the last known good deployment.\n",
	}

	cmd.AddCommand(newEnvConfigCmd())
	cmd.AddCommand(newEnvDestroyCmd())
	cmd.AddCommand(newEnvInitCmd())
	cmd.AddCommand(newEnvLsCmd())
	cmd.AddCommand(newEnvRmCmd())

	return cmd
}

func initEnvCmd(cmd *cobra.Command, args []string) (*envCmdInfo, error) {
	// Read in the name of the environment to use.
	if len(args) == 0 {
		return nil, fmt.Errorf("missing required environment name")
	}
	return initEnvCmdName(tokens.QName(args[0]), args[1:])
}

func initEnvCmdName(name tokens.QName, args []string) (*envCmdInfo, error) {
	// Read in the deployment information, bailing if an IO error occurs.
	ctx := resource.NewContext(sink())
	envfile, env, old := readEnv(ctx, name)
	if env == nil {
		contract.Assert(!ctx.Diag.Success())
		ctx.Close()                                                          // close now, since we are exiting.
		return nil, fmt.Errorf("could not read envfile required to proceed") // failure reading the env information.
	}
	return &envCmdInfo{
		Ctx:     ctx,
		Env:     env,
		Envfile: envfile,
		Old:     old,
		Args:    args,
	}, nil
}

type envCmdInfo struct {
	Ctx     *resource.Context // the resulting context
	Env     *resource.Env     // the environment information
	Envfile *resource.Envfile // the full serialized envfile from which this came.
	Old     resource.Snapshot // the environment's latest deployment snapshot
	Args    []string          // the args after extracting the environment name
}

func (eci *envCmdInfo) Close() error {
	return eci.Ctx.Close()
}

func confirmPrompt(msg string, args ...interface{}) bool {
	prompt := fmt.Sprintf(msg, args...)
	fmt.Printf(
		colors.ColorizeText(fmt.Sprintf("%v%v%v\n", colors.SpecAttention, prompt, colors.Reset)))
	fmt.Printf("Please confirm that this is what you'd like to do by typing (\"yes\"): ")
	reader := bufio.NewReader(os.Stdin)
	if line, _ := reader.ReadString('\n'); line != "yes\n" {
		fmt.Fprintf(os.Stderr, "Confirmation declined -- exiting without doing anything\n")
		return false
	}
	return true
}

// create just creates a new empty environment without deploying anything into it.
// TODO: add the ability to configure the environment at the command line.
func create(name tokens.QName) {
	env := &resource.Env{Name: name}
	if success := saveEnv(env, nil, "", false); success {
		fmt.Printf("Environment '%v' initialized; see `coco env deploy` to deploy into it\n", name)
	}
}

// remove permanently deletes the environment's information from the local workstation.
func remove(env *resource.Env) {
	deleteEnv(env)
	msg := fmt.Sprintf("%sEnvironment '%s' has been removed!%s\n",
		colors.SpecAttention, env.Name, colors.Reset)
	fmt.Printf(colors.ColorizeText(msg))
}

func prepareCompiler(cmd *cobra.Command, args []string) (compiler.Compiler, *pack.Package) {
	// If there's a --, we need to separate out the command args from the stack args.
	flags := cmd.Flags()
	dashdash := flags.ArgsLenAtDash()
	var packArgs []string
	if dashdash != -1 {
		packArgs = args[dashdash:]
		args = args[0:dashdash]
	}

	// Create a compiler options object and map any flags and arguments to settings on it.
	opts := core.DefaultOptions()
	opts.Args = dashdashArgsToMap(packArgs)

	// In the case of an argument, load that specific package and new up a compiler based on its base path.
	// Otherwise, use the default workspace and package logic (which consults the current working directory).
	var comp compiler.Compiler
	var pkg *pack.Package
	if len(args) == 0 {
		var err error
		comp, err = compiler.Newwd(opts)
		if err != nil {
			// Create a temporary diagnostics sink so that we can issue an error and bail out.
			sink().Errorf(errors.ErrorCantCreateCompiler, err)
		}
	} else {
		fn := args[0]
		if pkg = cmdutil.ReadPackageFromArg(fn); pkg != nil {
			var err error
			if fn == "-" {
				comp, err = compiler.Newwd(opts)
			} else {
				comp, err = compiler.New(filepath.Dir(fn), opts)
			}
			if err != nil {
				sink().Errorf(errors.ErrorCantReadPackage, fn, err)
			}
		}
	}

	return comp, pkg
}

// compile just uses the standard logic to parse arguments, options, and to locate/compile a package.  It returns the
// CocoGL graph that is produced, or nil if an error occurred (in which case, we would expect non-0 errors).
func compile(cmd *cobra.Command, args []string, config resource.ConfigMap) *compileResult {
	// Prepare the compiler info and, provided it succeeds, perform the compilation.
	if comp, pkg := prepareCompiler(cmd, args); comp != nil {
		// Create the preexec hook if the config map is non-nil.
		var preexec compiler.Preexec
		configVars := make(map[tokens.Token]*rt.Object)
		if config != nil {
			preexec = config.ConfigApplier(configVars)
		}

		// Now perform the compilation and extract the heap snapshot.
		var heap *heapstate.Heap
		var pkgsym *symbols.Package
		if pkg == nil {
			pkgsym, heap = comp.Compile(preexec)
		} else {
			pkgsym, heap = comp.CompilePackage(pkg, preexec)
		}

		return &compileResult{
			C:          comp,
			Pkg:        pkgsym,
			Heap:       heap,
			ConfigVars: configVars,
		}
	}

	return nil
}

type compileResult struct {
	C          compiler.Compiler
	Pkg        *symbols.Package
	Heap       *heapstate.Heap
	ConfigVars map[tokens.Token]*rt.Object
}

// verify creates a compiler, much like compile, but only performs binding and verification on it.  If verification
// succeeds, the return value is true; if verification fails, errors will have been output, and the return is false.
func verify(cmd *cobra.Command, args []string) bool {
	// Prepare the compiler info and, provided it succeeds, perform the verification.
	if comp, pkg := prepareCompiler(cmd, args); comp != nil {
		// Now perform the compilation and extract the heap snapshot.
		if pkg == nil {
			return comp.Verify()
		}
		return comp.VerifyPackage(pkg)
	}

	return false
}

// plan just uses the standard logic to parse arguments, options, and to create a snapshot and plan.
func plan(cmd *cobra.Command, info *envCmdInfo, opts applyOptions) *planResult {
	// If deleting, there is no need to create a new snapshot; otherwise, we will need to compile the package.
	var new resource.Snapshot
	var result *compileResult
	var analyzers []tokens.QName
	if !opts.Delete {
		// First, compile; if that yields errors or an empty heap, exit early.
		if result = compile(cmd, info.Args, info.Env.Config); result == nil || result.Heap == nil {
			return nil
		}

		// Create a resource snapshot from the compiled/evaluated object graph.
		var err error
		new, err = resource.NewGraphSnapshot(
			info.Ctx, info.Env.Name, result.Pkg.Tok, result.C.Ctx().Opts.Args, result.Heap, info.Old)
		if err != nil {
			result.C.Diag().Errorf(errors.ErrorCantCreateSnapshot, err)
			return nil
		} else if !info.Ctx.Diag.Success() {
			return nil
		}

		// If there are any analyzers to run, queue them up.
		for _, a := range opts.Analyzers {
			analyzers = append(analyzers, tokens.QName(a)) // from the command line.
		}
		if as := result.Pkg.Node.Analyzers; as != nil {
			for _, a := range *as {
				analyzers = append(analyzers, a) // from the project file.
			}
		}
	}

	// Generate a plan; this API handles all interesting cases (create, update, delete).
	plan, err := resource.NewPlan(info.Ctx, info.Old, new, analyzers)
	if err != nil {
		result.C.Diag().Errorf(errors.ErrorCantCreateSnapshot, err)
		return nil
	}
	if !info.Ctx.Diag.Success() {
		return nil
	}
	return &planResult{
		compileResult: result,
		Info:          info,
		New:           new,
		Plan:          plan,
	}
}

type planResult struct {
	*compileResult
	Info *envCmdInfo       // plan command information.
	Old  resource.Snapshot // the existing snapshot (if any).
	New  resource.Snapshot // the new snapshot for this plan (if any).
	Plan resource.Plan     // the plan created by this command.
}

func apply(cmd *cobra.Command, info *envCmdInfo, opts applyOptions) {
	if result := plan(cmd, info, opts); result != nil {
		// Now based on whether a dry run was specified, or not, either print or perform the planned operations.
		if opts.DryRun {
			// If no output file was requested, or "-", print to stdout; else write to that file.
			if opts.Output == "" || opts.Output == "-" {
				printPlan(info.Ctx.Diag, result, opts)
			} else {
				saveEnv(info.Env, result.New, opts.Output, true /*overwrite*/)
			}
		} else {
			// If show unchanged was requested, print them first, along with a header.
			var header bytes.Buffer
			printPrelude(&header, result, opts)
			header.WriteString(fmt.Sprintf("%vDeploying changes:%v\n", colors.SpecUnimportant, colors.Reset))
			fmt.Printf(colors.Colorize(&header))

			// Print a nice message if the update is an empty one.
			empty := checkEmpty(info.Ctx.Diag, result.Plan)

			// Create an object to track progress and perform the actual operations.
			start := time.Now()
			progress := newProgress(info.Ctx, opts.Summary)
			checkpoint, err, _, _ := result.Plan.Apply(progress)
			if err != nil {
				contract.Assert(!info.Ctx.Diag.Success()) // an error should have been emitted.
			}

			var summary bytes.Buffer
			if !empty {
				// Print out the total number of steps performed (and their kinds), the duration, and any summary info.
				printSummary(&summary, progress.Ops, opts.ShowReplaceSteps, false)
				summary.WriteString(fmt.Sprintf("%vDeployment duration: %v%v\n",
					colors.SpecUnimportant, time.Since(start), colors.Reset))
			}

			if progress.MaybeCorrupt {
				summary.WriteString(fmt.Sprintf(
					"%vfatal: A catastrophic error occurred; resources states may be unknown%v\n",
					colors.SpecAttention, colors.Reset))
			}

			// Now save the updated snapshot to the specified output file, if any, or the standard location otherwise.
			// Note that if a failure has occurred, the Apply routine above will have returned a safe checkpoint.
			env := result.Info.Env
			saveEnv(env, checkpoint, opts.Output, true /*overwrite*/)

			fmt.Printf(colors.Colorize(&summary))
		}
	}
}

func checkEmpty(d diag.Sink, plan resource.Plan) bool {
	// If we are doing an empty update, say so.
	if plan.Empty() {
		d.Infof(diag.Message("no resources need to be updated"))
		return true
	}
	return false
}

// backupEnv makes a backup of an existing file, in preparation for writing a new one.  Instead of a copy, it
// simply renames the file, which is simpler, more efficient, etc.
func backupEnv(file string) {
	contract.Require(file != "", "file")
	os.Rename(file, file+".bak") // ignore errors.
	// TODO: consider multiple backups (.bak.bak.bak...etc).
}

// deleteEnv removes an existing snapshot file, leaving behind a backup.
func deleteEnv(env *resource.Env) {
	contract.Require(env != nil, "env")
	// Just make a backup of the file and don't write out anything new.
	file := workspace.EnvPath(env.Name)
	backupEnv(file)
}

// readEnv reads in an existing snapshot file, issuing an error and returning nil if something goes awry.
func readEnv(ctx *resource.Context, name tokens.QName) (*resource.Envfile, *resource.Env, resource.Snapshot) {
	contract.Require(name != "", "name")
	file := workspace.EnvPath(name)

	// Detect the encoding of the file so we can do our initial unmarshaling.
	m, ext := encoding.Detect(file)
	if m == nil {
		ctx.Diag.Errorf(errors.ErrorIllegalMarkupExtension, ext)
		return nil, nil, nil
	}

	// Now read the whole file into a byte blob.
	b, err := ioutil.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			ctx.Diag.Errorf(errors.ErrorInvalidEnvName, name)
		} else {
			ctx.Diag.Errorf(errors.ErrorIO, err)
		}
		return nil, nil, nil
	}

	// Unmarshal the contents into a envfile deployment structure.
	var envfile resource.Envfile
	if err = m.Unmarshal(b, &envfile); err != nil {
		ctx.Diag.Errorf(errors.ErrorCantReadDeployment, file, err)
		return nil, nil, nil
	}

	// Next, use the mapping infrastructure to validate the contents.
	// TODO: we can eliminate this redundant unmarshaling once Go supports strict unmarshaling.
	var obj mapper.Object
	if err = m.Unmarshal(b, &obj); err != nil {
		ctx.Diag.Errorf(errors.ErrorCantReadDeployment, file, err)
		return nil, nil, nil
	} else {
		if obj["latest"] != nil {
			if latest, islatest := obj["latest"].(map[string]interface{}); islatest {
				delete(latest, "resources") // remove the resources, since they require custom marshaling.
			}
		}
		md := mapper.New(nil)
		var ignore resource.Envfile // just for errors.
		if err = md.Decode(obj, &ignore); err != nil {
			ctx.Diag.Errorf(errors.ErrorCantReadDeployment, file, err)
			return nil, nil, nil
		}
	}

	env, snap := resource.DeserializeEnvfile(ctx, &envfile)
	contract.Assert(env != nil)
	return &envfile, env, snap
}

// saveEnv saves a new snapshot at the given location, backing up any existing ones.
func saveEnv(env *resource.Env, snap resource.Snapshot, file string, existok bool) bool {
	contract.Require(env != nil, "env")
	if file == "" {
		file = workspace.EnvPath(env.Name)
	}

	// Make a serializable CocoGL data structure and then use the encoder to encode it.
	m, ext := encoding.Detect(file)
	if m == nil {
		sink().Errorf(errors.ErrorIllegalMarkupExtension, ext)
		return false
	}
	if filepath.Ext(file) == "" {
		file = file + ext
	}
	dep := resource.SerializeEnvfile(env, snap, "")
	b, err := m.Marshal(dep)
	if err != nil {
		sink().Errorf(errors.ErrorIO, err)
		return false
	}

	// If it's not ok for the file to already exist, ensure that it doesn't.
	if !existok {
		if _, err := os.Stat(file); err == nil {
			sink().Errorf(errors.ErrorIO, fmt.Errorf("file '%v' already exists", file))
			return false
		}
	}

	// Back up the existing file if it already exists.
	backupEnv(file)

	// Ensure the directory exists.
	if err = os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		sink().Errorf(errors.ErrorIO, err)
		return false
	}

	// And now write out the new snapshot file, overwriting that location.
	if err = ioutil.WriteFile(file, b, 0644); err != nil {
		sink().Errorf(errors.ErrorIO, err)
		return false
	}

	return true
}

type applyOptions struct {
	Create           bool     // true if we are creating resources.
	Delete           bool     // true if we are deleting resources.
	DryRun           bool     // true if we should just print the plan without performing it.
	Analyzers        []string // an optional set of analyzers to run as part of this deployment.
	ShowConfig       bool     // true to show the configuration variables being used.
	ShowReplaceSteps bool     // true to show the replacement steps in the plan.
	ShowUnchanged    bool     // true to show the resources that aren't updated, in addition to those that are.
	Summary          bool     // true if we should only summarize resources and operations.
	Output           string   // the place to store the output, if any.
}

// applyProgress pretty-prints the plan application process as it goes.
type applyProgress struct {
	Ctx          *resource.Context
	Steps        int
	Ops          map[resource.StepOp]int
	MaybeCorrupt bool
	Summary      bool
}

func newProgress(ctx *resource.Context, summary bool) *applyProgress {
	return &applyProgress{
		Ctx:     ctx,
		Steps:   0,
		Ops:     make(map[resource.StepOp]int),
		Summary: summary,
	}
}

func (prog *applyProgress) Before(step resource.Step) {
	// Print the step.
	stepop := step.Op()
	stepnum := prog.Steps + 1

	var extra string
	if stepop == resource.OpReplaceCreate || stepop == resource.OpReplaceDelete {
		extra = " (part of a replacement step)"
	}

	var b bytes.Buffer
	b.WriteString(fmt.Sprintf("Applying step #%v [%v]%v\n", stepnum, stepop, extra))
	printStep(&b, step, prog.Summary, "    ")
	fmt.Printf(colors.Colorize(&b))
}

func (prog *applyProgress) After(step resource.Step, err error, state resource.ResourceState) {
	if err == nil {
		// Increment the counters.
		prog.Steps++
		prog.Ops[step.Op()]++
	} else {
		// Issue a true, bonafide error.
		prog.Ctx.Diag.Errorf(errors.ErrorPlanApplyFailed, err)

		// Print the state of the resource; we don't issue the error, because the apply above will do that.
		var b bytes.Buffer
		stepnum := prog.Steps + 1
		b.WriteString(fmt.Sprintf("Step #%v failed [%v]: ", stepnum, step.Op()))
		switch state {
		case resource.StateOK:
			b.WriteString(colors.SpecNote)
			b.WriteString("provider successfully recovered from this failure")
		case resource.StateUnknown:
			b.WriteString(colors.SpecAttention)
			b.WriteString("this failure was catastrophic and the provider cannot guarantee recovery")
			prog.MaybeCorrupt = true
		default:
			contract.Failf("Unrecognized resource state: %v", state)
		}
		b.WriteString(colors.Reset)
		b.WriteString("\n")
		fmt.Printf(colors.Colorize(&b))
	}
}

func printPlan(d diag.Sink, result *planResult, opts applyOptions) {
	// First print config/unchanged/etc. if necessary.
	var prelude bytes.Buffer
	printPrelude(&prelude, result, opts)

	// Now walk the plan's steps and and pretty-print them out.
	prelude.WriteString(fmt.Sprintf("%vPlanned changes:%v\n", colors.SpecUnimportant, colors.Reset))
	fmt.Printf(colors.Colorize(&prelude))

	// Print a nice message if the update is an empty one.
	if empty := checkEmpty(d, result.Plan); !empty {
		var summary bytes.Buffer
		step := result.Plan.Steps()
		counts := make(map[resource.StepOp]int)
		for step != nil {
			op := step.Op()
			// Print this step information (resource and all its properties).
			// TODO: it would be nice if, in the output, we showed the dependencies a la `git log --graph`.
			if opts.ShowReplaceSteps || (op != resource.OpReplaceCreate && op != resource.OpReplaceDelete) {
				printStep(&summary, step, opts.Summary, "")
			}
			counts[step.Op()]++
			step = step.Next()
		}

		// Print a summary of operation counts.
		printSummary(&summary, counts, opts.ShowReplaceSteps, true)
		fmt.Printf(colors.Colorize(&summary))
	}
}

func printPrelude(b *bytes.Buffer, result *planResult, opts applyOptions) {
	// If there are configuration variables, show them.
	if opts.ShowConfig {
		printConfig(b, result.compileResult)
	}

	// If show-sames was requested, walk the sames and print them.
	if opts.ShowUnchanged {
		printUnchanged(b, result.Plan, opts.Summary)
	}
}

func printConfig(b *bytes.Buffer, result *compileResult) {
	b.WriteString(fmt.Sprintf("%vConfiguration:%v\n", colors.SpecUnimportant, colors.Reset))
	if result != nil && result.ConfigVars != nil {
		var toks []string
		for tok := range result.ConfigVars {
			toks = append(toks, string(tok))
		}
		sort.Strings(toks)
		for _, tok := range toks {
			b.WriteString(fmt.Sprintf("%v%v: %v\n", detailsIndent, tok, result.ConfigVars[tokens.Token(tok)]))
		}
	}
}

func printSummary(b *bytes.Buffer, counts map[resource.StepOp]int, showReplaceSteps bool, plan bool) {
	total := 0
	for _, c := range counts {
		total += c
	}

	var planned string
	if plan {
		planned = "planned "
	}
	var colon string
	if total != 0 {
		colon = ":"
	}
	b.WriteString(fmt.Sprintf("%v total %v%v%v\n", total, planned, plural("change", total), colon))

	var planTo string
	var pastTense string
	if plan {
		planTo = "to "
	} else {
		pastTense = "d"
	}

	for _, op := range resource.StepOps() {
		if !showReplaceSteps && (op == resource.OpReplaceCreate || op == resource.OpReplaceDelete) {
			// Unless the user requested it, don't show the fine-grained replacement steps; just the logical ones.
			continue
		}
		if c := counts[op]; c > 0 {
			b.WriteString(fmt.Sprintf("    %v%v %v %v%v%v%v\n",
				op.Prefix(), c, plural("resource", c), planTo, op, pastTense, colors.Reset))
		}
	}
}

func plural(s string, c int) string {
	if c != 1 {
		s += "s"
	}
	return s
}

const detailsIndent = "      " // 4 spaces, plus 2 for "+ ", "- ", and " " leaders

func printUnchanged(b *bytes.Buffer, plan resource.Plan, summary bool) {
	b.WriteString(fmt.Sprintf("%vUnchanged resources:%v\n", colors.SpecUnimportant, colors.Reset))
	for _, res := range plan.Unchanged() {
		b.WriteString("  ") // simulate the 2 spaces for +, -, etc.
		printResourceHeader(b, res, nil, "")
		printResourceProperties(b, res, nil, nil, nil, summary, "")
	}
}

func printStep(b *bytes.Buffer, step resource.Step, summary bool, indent string) {
	// First print out the operation's prefix.
	b.WriteString(step.Op().Prefix())

	// Next print the resource URN, properties, etc.
	printResourceHeader(b, step.Old(), step.New(), indent)
	b.WriteString(step.Op().Suffix())
	var replaces []resource.PropertyKey
	if step.Old() != nil {
		m := step.Old().URN()
		replaceMap := step.Plan().Replaces()
		replaces = replaceMap[m]
	}
	printResourceProperties(b, step.Old(), step.New(), step.NewProps(), replaces, summary, indent)

	// Finally make sure to reset the color.
	b.WriteString(colors.Reset)
}

func printResourceHeader(b *bytes.Buffer, old resource.Resource, new resource.Resource, indent string) {
	var t tokens.Type
	if old == nil {
		t = new.Type()
	} else {
		t = old.Type()
	}

	// The primary header is the resource type (since it is easy on the eyes).
	b.WriteString(fmt.Sprintf("%s:\n", string(t)))
}

func printResourceProperties(b *bytes.Buffer, old resource.Resource, new resource.Resource,
	computed resource.PropertyMap, replaces []resource.PropertyKey, summary bool, indent string) {
	indent += detailsIndent

	// Print out the URN and, if present, the ID, as "pseudo-properties".
	var id resource.ID
	var URN resource.URN
	if old == nil {
		id = new.ID()
		URN = new.URN()
	} else {
		id = old.ID()
		URN = old.URN()
	}
	if id != "" {
		b.WriteString(fmt.Sprintf("%s[id=%s]\n", indent, string(id)))
	}
	b.WriteString(fmt.Sprintf("%s[urn=%s]\n", indent, URN.Name()))

	if !summary {
		// Print all of the properties associated with this resource.
		if old == nil && new != nil {
			printObject(b, new.Properties(), indent)
		} else if new == nil && old != nil {
			printObject(b, old.Properties(), indent)
		} else {
			contract.Assert(computed != nil) // use computed properties for diffs.
			printOldNewDiffs(b, old.Properties(), computed, replaces, indent)
		}
	}
}

func printObject(b *bytes.Buffer, props resource.PropertyMap, indent string) {
	// Compute the maximum with of property keys so we can justify everything.
	keys := resource.StablePropertyKeys(props)
	maxkey := 0
	for _, k := range keys {
		if len(k) > maxkey {
			maxkey = len(k)
		}
	}

	// Now print out the values intelligently based on the type.
	for _, k := range keys {
		if v := props[k]; shouldPrintPropertyValue(v) {
			printPropertyTitle(b, k, maxkey, indent)
			printPropertyValue(b, v, indent)
		}
	}
}

func shouldPrintPropertyValue(v resource.PropertyValue) bool {
	return !v.IsNull() // by default, don't print nulls (they just clutter up the output)
}

func printPropertyTitle(b *bytes.Buffer, k resource.PropertyKey, align int, indent string) {
	b.WriteString(fmt.Sprintf("%s%-"+strconv.Itoa(align)+"s: ", indent, k))
}

func printPropertyValue(b *bytes.Buffer, v resource.PropertyValue, indent string) {
	if v.IsNull() {
		b.WriteString("<null>")
	} else if v.IsBool() {
		b.WriteString(fmt.Sprintf("%t", v.BoolValue()))
	} else if v.IsNumber() {
		b.WriteString(fmt.Sprintf("%v", v.NumberValue()))
	} else if v.IsString() {
		b.WriteString(fmt.Sprintf("\"%s\"", v.StringValue()))
	} else if v.IsResource() {
		b.WriteString(fmt.Sprintf("-> *%s", v.ResourceValue()))
	} else if v.IsArray() {
		b.WriteString(fmt.Sprintf("[\n"))
		for i, elem := range v.ArrayValue() {
			newIndent := printArrayElemHeader(b, i, indent)
			printPropertyValue(b, elem, newIndent)
		}
		b.WriteString(fmt.Sprintf("%s]", indent))
	} else {
		contract.Assert(v.IsObject())
		b.WriteString("{\n")
		printObject(b, v.ObjectValue(), indent+"    ")
		b.WriteString(fmt.Sprintf("%s}", indent))
	}
	b.WriteString("\n")
}

func getArrayElemHeader(b *bytes.Buffer, i int, indent string) (string, string) {
	prefix := fmt.Sprintf("    %s[%d]: ", indent, i)
	return prefix, fmt.Sprintf("%-"+strconv.Itoa(len(prefix))+"s", "")
}

func printArrayElemHeader(b *bytes.Buffer, i int, indent string) string {
	prefix, newIndent := getArrayElemHeader(b, i, indent)
	b.WriteString(prefix)
	return newIndent
}

func printOldNewDiffs(b *bytes.Buffer, olds resource.PropertyMap, news resource.PropertyMap,
	replaces []resource.PropertyKey, indent string) {
	// Get the full diff structure between the two, and print it (recursively).
	if diff := olds.Diff(news); diff != nil {
		printObjectDiff(b, *diff, replaces, false, indent)
	} else {
		printObject(b, news, indent)
	}
}

func printObjectDiff(b *bytes.Buffer, diff resource.ObjectDiff,
	replaces []resource.PropertyKey, causedReplace bool, indent string) {
	contract.Assert(len(indent) > 2)

	// Compute the maximum with of property keys so we can justify everything.
	keys := diff.Keys()
	maxkey := 0
	for _, k := range keys {
		if len(k) > maxkey {
			maxkey = len(k)
		}
	}

	// If a list of what causes a resource to get replaced exist, create a handy map.
	var replaceMap map[resource.PropertyKey]bool
	if len(replaces) > 0 {
		replaceMap = make(map[resource.PropertyKey]bool)
		for _, k := range replaces {
			replaceMap[k] = true
		}
	}

	// To print an object diff, enumerate the keys in stable order, and print each property independently.
	for _, k := range keys {
		title := func(id string) { printPropertyTitle(b, k, maxkey, id) }
		if add, isadd := diff.Adds[k]; isadd {
			if shouldPrintPropertyValue(add) {
				b.WriteString(colors.SpecAdded)
				title(addIndent(indent))
				printPropertyValue(b, add, addIndent(indent))
				b.WriteString(colors.Reset)
			}
		} else if delete, isdelete := diff.Deletes[k]; isdelete {
			if shouldPrintPropertyValue(delete) {
				b.WriteString(colors.SpecDeleted)
				title(deleteIndent(indent))
				printPropertyValue(b, delete, deleteIndent(indent))
				b.WriteString(colors.Reset)
			}
		} else if update, isupdate := diff.Updates[k]; isupdate {
			if !causedReplace && replaceMap != nil {
				causedReplace = replaceMap[k]
			}
			printPropertyValueDiff(b, title, update, causedReplace, indent)
		} else if same := diff.Sames[k]; shouldPrintPropertyValue(same) {
			title(indent)
			printPropertyValue(b, diff.Sames[k], indent)
		}
	}
}

func printPropertyValueDiff(b *bytes.Buffer, title func(string), diff resource.ValueDiff,
	causedReplace bool, indent string) {
	contract.Assert(len(indent) > 2)

	if diff.Array != nil {
		title(indent)
		b.WriteString("[\n")

		a := diff.Array
		for i := 0; i < a.Len(); i++ {
			_, newIndent := getArrayElemHeader(b, i, indent)
			title := func(id string) { printArrayElemHeader(b, i, id) }
			if add, isadd := a.Adds[i]; isadd {
				b.WriteString(resource.OpCreate.Color())
				title(addIndent(indent))
				printPropertyValue(b, add, addIndent(newIndent))
				b.WriteString(colors.Reset)
			} else if delete, isdelete := a.Deletes[i]; isdelete {
				b.WriteString(resource.OpDelete.Color())
				title(deleteIndent(indent))
				printPropertyValue(b, delete, deleteIndent(newIndent))
				b.WriteString(colors.Reset)
			} else if update, isupdate := a.Updates[i]; isupdate {
				title(indent)
				printPropertyValueDiff(b, func(string) {}, update, causedReplace, newIndent)
			} else {
				title(indent)
				printPropertyValue(b, a.Sames[i], newIndent)
			}
		}
		b.WriteString(fmt.Sprintf("%s]\n", indent))
	} else if diff.Object != nil {
		title(indent)
		b.WriteString("{\n")
		printObjectDiff(b, *diff.Object, nil, causedReplace, indent+"    ")
		b.WriteString(fmt.Sprintf("%s}\n", indent))
	} else if diff.Old.IsResource() && diff.New.IsResource() && diff.New.ResourceValue().Replacement() {
		// If the old and new are both resources, and the new is a replacement, show this in a special way (+-).
		b.WriteString(resource.OpReplace.Color())
		title(updateIndent(indent))
		printPropertyValue(b, diff.Old, updateIndent(indent))
		b.WriteString(colors.Reset)
	} else {
		// If we ended up here, the two values either differ by type, or they have different primitive values.  We will
		// simply emit a deletion line followed by an addition line.
		if shouldPrintPropertyValue(diff.Old) {
			var color string
			if causedReplace {
				color = resource.OpDelete.Color() // this property triggered replacement; color as a delete
			} else {
				color = resource.OpUpdate.Color()
			}
			b.WriteString(color)
			title(deleteIndent(indent))
			printPropertyValue(b, diff.Old, deleteIndent(indent))
			b.WriteString(colors.Reset)
		}
		if shouldPrintPropertyValue(diff.New) {
			var color string
			if causedReplace {
				color = resource.OpCreate.Color() // this property triggered replacement; color as a create
			} else {
				color = resource.OpUpdate.Color()
			}
			b.WriteString(color)
			title(addIndent(indent))
			printPropertyValue(b, diff.New, addIndent(indent))
			b.WriteString(colors.Reset)
		}
	}
}

func addIndent(indent string) string    { return indent[:len(indent)-2] + "+ " }
func deleteIndent(indent string) string { return indent[:len(indent)-2] + "- " }
func updateIndent(indent string) string { return indent[:len(indent)-2] + "+-" }
