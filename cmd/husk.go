// Copyright 2016 Pulumi, Inc. All rights reserved.

package cmd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
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
	"github.com/pulumi/coconut/pkg/resource"
	"github.com/pulumi/coconut/pkg/tokens"
	"github.com/pulumi/coconut/pkg/util/cmdutil"
	"github.com/pulumi/coconut/pkg/util/contract"
	"github.com/pulumi/coconut/pkg/util/mapper"
	"github.com/pulumi/coconut/pkg/workspace"
)

func newHuskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "husk",
		Short: "Manage or deploy into husks (deployment targets)",
		Long: "Manage or deploy into husks (deployment targets)\n" +
			"\n" +
			"A husk is a named deployment target, and a single nut have many of them.  Each husk\n" +
			"has a deployment history associated with it, stored in the workspace, in addition to\n" +
			"the last known good deployment.  A husk may also have unique configuration entries.\n",
	}

	cmd.AddCommand(newHuskDeployCmd())
	cmd.AddCommand(newHuskDestroyCmd())
	cmd.AddCommand(newHuskInitCmd())

	return cmd
}

var snk diag.Sink

// sink lazily allocates a sink to be used if we can't create a compiler.
func sink() diag.Sink {
	if snk == nil {
		snk = core.DefaultSink("")
	}
	return snk
}

// create just creates a new husk without deploying anything into it.
func create(cmd *cobra.Command, args []string) {
	// Read in the name of the husk to use.
	var husk tokens.QName
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "fatal: missing required husk name\n")
		os.Exit(-1)
	} else {
		husk = tokens.QName(args[0])
		args = args[1:]
	}

	if success := saveDeployment(husk, nil, "", false); success {
		fmt.Printf("Coconut husk '%v' initialized; ready for deployments (see `coco husk deploy`)\n", husk)
	}
}

// compile just uses the standard logic to parse arguments, options, and to locate/compile a package.  It returns the
// CocoGL graph that is produced, or nil if an error occurred (in which case, we would expect non-0 errors).
func compile(cmd *cobra.Command, args []string) *compileResult {
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
	var pkg *symbols.Package
	var heap *heapstate.Heap
	if len(args) == 0 {
		var err error
		comp, err = compiler.Newwd(opts)
		if err != nil {
			// Create a temporary diagnostics sink so that we can issue an error and bail out.
			sink().Errorf(errors.ErrorCantCreateCompiler, err)
			return nil
		}
		pkg, heap = comp.Compile()
	} else {
		fn := args[0]
		if pkgmeta := cmdutil.ReadPackageFromArg(fn); pkgmeta != nil {
			var err error
			if fn == "-" {
				comp, err = compiler.Newwd(opts)
			} else {
				comp, err = compiler.New(filepath.Dir(fn), opts)
			}
			if err != nil {
				sink().Errorf(errors.ErrorCantReadPackage, fn, err)
				return nil
			}
			pkg, heap = comp.CompilePackage(pkgmeta)
		}
	}
	return &compileResult{comp, pkg, heap}
}

type compileResult struct {
	C    compiler.Compiler
	Pkg  *symbols.Package
	Heap *heapstate.Heap
}

// plan just uses the standard logic to parse arguments, options, and to create a snapshot and plan.
func plan(cmd *cobra.Command, args []string, husk tokens.QName, delete bool) *planResult {
	// Create a new context for the plan operations.
	ctx := resource.NewContext(sink())

	// Read in the deployment information, bailing if an IO error occurs.
	dep, old := readDeployment(ctx, husk)
	if dep == nil {
		contract.Assert(!ctx.Diag.Success())
		return nil // failure reading the husk information.
	}

	// If deleting, there is no need to create a new snapshot; otherwise, we will need to compile the package.
	var new resource.Snapshot
	var result *compileResult
	if !delete {
		// First, compile; if that yields errors or an empty heap, exit early.
		if result = compile(cmd, args); result == nil || result.Heap == nil {
			return nil
		}

		// Create a resource snapshot from the compiled/evaluated object graph.
		var err error
		new, err = resource.NewGraphSnapshot(ctx, husk, result.Pkg.Tok, result.C.Ctx().Opts.Args, result.Heap)
		if err != nil {
			result.C.Diag().Errorf(errors.ErrorCantCreateSnapshot, err)
			return nil
		} else if !ctx.Diag.Success() {
			return nil
		}
	}

	// Generate a plan; this API handles all interesting cases (create, update, delete).
	plan := resource.NewPlan(ctx, old, new)
	return &planResult{
		compileResult: result,
		Ctx:           ctx,
		Husk:          husk,
		Old:           old,
		New:           new,
		Plan:          plan,
	}
}

type planResult struct {
	*compileResult
	Ctx  *resource.Context
	Husk tokens.QName      // the husk name.
	Old  resource.Snapshot // the existing snapshot (if any).
	New  resource.Snapshot // the new snapshot for this plan (if any).
	Plan resource.Plan
}

func apply(cmd *cobra.Command, args []string, opts applyOptions) {
	// Read in the name of the husk to use.
	var husk tokens.QName
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "fatal: missing required husk name\n")
		os.Exit(-1)
	} else {
		husk = tokens.QName(args[0])
		args = args[1:]
	}

	if result := plan(cmd, args, husk, opts.Delete); result != nil {
		// If we are doing an empty update, say so.
		if result.Plan.Empty() && !opts.Delete {
			sink().Infof(diag.Message("nothing to do -- resources are up to date"))
		}

		// Now based on whether a dry run was specified, or not, either print or perform the planned operations.
		if opts.DryRun {
			// If no output file was requested, or "-", print to stdout; else write to that file.
			if opts.Output == "" || opts.Output == "-" {
				printPlan(result.Plan, opts.Summary)
			} else {
				saveDeployment(husk, result.New, opts.Output, true /*overwrite*/)
			}
		} else {
			// Create an object to track progress and perform the actual operations.
			start := time.Now()
			progress := newProgress(opts.Summary)
			if err, _, _ := result.Plan.Apply(progress); err != nil {
				// TODO: we want richer diagnostics in the event that a plan apply fails.  For instance, we want to
				//     know precisely what step failed, we want to know whether it was catastrophic, etc.  We also
				//     probably want to plumb diag.Sink through apply so it can issue its own rich diagnostics.
				sink().Errorf(errors.ErrorPlanApplyFailed, err)
			}

			// Print out the total number of steps performed (and their kinds), if any succeeded.
			var b bytes.Buffer
			if progress.Steps > 0 {
				b.WriteString(fmt.Sprintf("%v total operations in %v:\n", progress.Steps, time.Since(start)))
				if c := progress.Ops[resource.OpCreate]; c > 0 {
					b.WriteString(fmt.Sprintf("    %v%v resources created%v\n",
						opPrefix(resource.OpCreate), c, colors.Reset))
				}
				if c := progress.Ops[resource.OpUpdate]; c > 0 {
					b.WriteString(fmt.Sprintf("    %v%v resources updated%v\n",
						opPrefix(resource.OpUpdate), c, colors.Reset))
				}
				if c := progress.Ops[resource.OpDelete]; c > 0 {
					b.WriteString(fmt.Sprintf("    %v%v resources deleted%v\n",
						opPrefix(resource.OpDelete), c, colors.Reset))
				}
			}
			if progress.MaybeCorrupt {
				b.WriteString(fmt.Sprintf(
					"%vfatal: A catastrophic error occurred; resources states may be unknown%v\n",
					colors.SpecFatal, colors.Reset))
			}
			s := b.String()
			fmt.Printf(colors.Colorize(s))

			// Now save the updated snapshot to the specified output file, if any, or the standard location otherwise.
			// TODO: save partial updates if we weren't able to perform the entire planned set of operations.
			if opts.Delete {
				deleteDeployment(result.Husk)
				fmt.Printf("Coconut husk '%v' has been destroyed!\n", result.Husk)
			} else {
				contract.Assert(result.New != nil)
				saveDeployment(result.Husk, result.New, opts.Output, true /*overwrite*/)
			}
		}
	}
}

// backupDeployment makes a backup of an existing file, in preparation for writing a new one.  Instead of a copy, it
// simply renames the file, which is simpler, more efficient, etc.
func backupDeployment(file string) {
	contract.Require(file != "", "file")
	os.Rename(file, file+".bak") // ignore errors.
	// TODO: consider multiple backups (.bak.bak.bak...etc).
}

// deleteDeployment removes an existing snapshot file, leaving behind a backup.
func deleteDeployment(husk tokens.QName) {
	contract.Require(husk != "", "husk")
	// Just make a backup of the file and don't write out anything new.
	file := workspace.HuskPath(husk)
	backupDeployment(file)
}

// readDeployment reads in an existing snapshot file, issuing an error and returning nil if something goes awry.
func readDeployment(ctx *resource.Context, husk tokens.QName) (*resource.Deployment, resource.Snapshot) {
	contract.Require(husk != "", "husk")
	file := workspace.HuskPath(husk)

	// Detect the encoding of the file so we can do our initial unmarshaling.
	m, ext := encoding.Detect(file)
	if m == nil {
		sink().Errorf(errors.ErrorIllegalMarkupExtension, ext)
		return nil, nil
	}

	// Now read the whole file into a byte blob.
	b, err := ioutil.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			sink().Errorf(errors.ErrorInvalidHuskName, husk)
		} else {
			sink().Errorf(errors.ErrorIO, err)
		}
		return nil, nil
	}

	// Unmarshal the contents into a deployment structure.
	var dep resource.Deployment
	if err = m.Unmarshal(b, &dep); err != nil {
		sink().Errorf(errors.ErrorCantReadDeployment, file, err)
		return nil, nil
	}

	// Next, use the mapping infrastructure to validate the contents.
	var obj mapper.Object
	if err = m.Unmarshal(b, &obj); err != nil {
		sink().Errorf(errors.ErrorCantReadDeployment, file, err)
		return nil, nil
	} else {
		if obj["latest"] != nil {
			if latest, islatest := obj["latest"].(map[string]interface{}); islatest {
				delete(latest, "resources") // remove the resources, since they require custom marshaling.
			}
		}
		md := mapper.New(nil)
		if err = md.Decode(obj, &dep); err != nil {
			sink().Errorf(errors.ErrorCantReadDeployment, file, err)
			return nil, nil
		}
	}

	return &dep, resource.DeserializeDeployment(ctx, &dep)
}

// saveDeployment saves a new snapshot at the given location, backing up any existing ones.
func saveDeployment(husk tokens.QName, snap resource.Snapshot, file string, existok bool) bool {
	contract.Require(husk != "", "husk")
	if file == "" {
		file = workspace.HuskPath(husk)
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
	dep := resource.SerializeDeployment(husk, snap, "")
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
	backupDeployment(file)

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
	Create  bool   // true if we are creating resources.
	Delete  bool   // true if we are deleting resources.
	DryRun  bool   // true if we should just print the plan without performing it.
	Summary bool   // true if we should only summarize resources and operations.
	Output  string // the place to store the output, if any.
}

// applyProgress pretty-prints the plan application process as it goes.
type applyProgress struct {
	Steps        int
	Ops          map[resource.StepOp]int
	MaybeCorrupt bool
	Summary      bool
}

func newProgress(summary bool) *applyProgress {
	return &applyProgress{
		Steps:   0,
		Ops:     make(map[resource.StepOp]int),
		Summary: summary,
	}
}

func (prog *applyProgress) Before(step resource.Step) {
	// Print the step.
	var b bytes.Buffer
	stepnum := prog.Steps + 1
	b.WriteString(fmt.Sprintf("Applying step #%v [%v]\n", stepnum, step.Op()))
	printStep(&b, step, prog.Summary, "    ")
	s := colors.Colorize(b.String())
	fmt.Printf(s)
}

func (prog *applyProgress) After(step resource.Step, err error, state resource.ResourceState) {
	if err == nil {
		// Increment the counters.
		prog.Steps++
		prog.Ops[step.Op()]++
	} else {
		var b bytes.Buffer
		// Print the state of the resource; we don't issue the error, because the apply above will do that.
		stepnum := prog.Steps + 1
		b.WriteString(fmt.Sprintf("Step #%v failed [%v]: ", stepnum, step.Op()))
		switch state {
		case resource.StateOK:
			b.WriteString(colors.SpecNote)
			b.WriteString("provider successfully recovered from this failure")
		case resource.StateUnknown:
			b.WriteString(colors.SpecFatal)
			b.WriteString("this failure was catastrophic and the provider cannot guarantee recovery")
			prog.MaybeCorrupt = true
		default:
			contract.Failf("Unrecognized resource state: %v", state)
		}
		b.WriteString(colors.Reset)
		b.WriteString("\n")
		s := colors.Colorize(b.String())
		fmt.Printf(s)
	}
}

func printPlan(plan resource.Plan, summary bool) {
	// Now walk the plan's steps and and pretty-print them out.
	step := plan.Steps()
	for step != nil {
		var b bytes.Buffer

		// Print this step information (resource and all its properties).
		printStep(&b, step, summary, "")

		// Now go ahead and emit the output to the console, and move on to the next step in the plan.
		// TODO: it would be nice if, in the output, we showed the dependencies a la `git log --graph`.
		s := colors.Colorize(b.String())
		fmt.Printf(s)

		step = step.Next()
	}
}

func opPrefix(op resource.StepOp) string {
	switch op {
	case resource.OpCreate:
		return colors.SpecAdded + "+ "
	case resource.OpDelete:
		return colors.SpecDeleted + "- "
	case resource.OpUpdate:
		return colors.SpecChanged + "  "
	default:
		contract.Failf("Unrecognized resource step op: %v", op)
		return ""
	}
}

func opSuffix(op resource.StepOp) string {
	if op == resource.OpUpdate {
		return colors.Reset // updates colorize individual lines
	}
	return ""
}

const resourceDetailsIndent = "      " // 4 spaces, plus space for "+ ", "- ", and " " leaders

func printStep(b *bytes.Buffer, step resource.Step, summary bool, indent string) {
	// First print out the operation's prefix.
	b.WriteString(opPrefix(step.Op()))

	// Next print the resource moniker, properties, etc.
	printResourceHeader(b, step.Old(), step.New(), indent)
	b.WriteString(opSuffix(step.Op()))
	printResourceProperties(b, step.Old(), step.New(), summary, indent)

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
	summary bool, indent string) {
	indent += resourceDetailsIndent

	// Print out the moniker and, if present, the ID, as "pseudo-properties".
	var id resource.ID
	var moniker resource.Moniker
	if old == nil {
		id = new.ID()
		moniker = new.Moniker()
	} else {
		id = old.ID()
		moniker = old.Moniker()
	}
	if id != "" {
		b.WriteString(fmt.Sprintf("%s[id=%s]\n", indent, string(id)))
	}
	b.WriteString(fmt.Sprintf("%s[mk=%s]\n", indent, string(moniker)))

	if !summary {
		// Print all of the properties associated with this resource.
		if old == nil && new != nil {
			printObject(b, new.Properties(), indent)
		} else if new == nil && old != nil {
			printObject(b, old.Properties(), indent)
		} else {
			printOldNewDiffs(b, old.Properties(), new.Properties(), indent)
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
	prefix := fmt.Sprintf("%s    [%d]: ", indent, i)
	return prefix, fmt.Sprintf("%-"+strconv.Itoa(len(prefix))+"s", "")
}

func printArrayElemHeader(b *bytes.Buffer, i int, indent string) string {
	prefix, newIndent := getArrayElemHeader(b, i, indent)
	b.WriteString(prefix)
	return newIndent
}

func printOldNewDiffs(b *bytes.Buffer, olds resource.PropertyMap, news resource.PropertyMap, indent string) {
	// Get the full diff structure between the two, and print it (recursively).
	if diff := olds.Diff(news); diff != nil {
		printObjectDiff(b, *diff, indent)
	} else {
		printObject(b, news, indent)
	}
}

func printObjectDiff(b *bytes.Buffer, diff resource.ObjectDiff, indent string) {
	contract.Assert(len(indent) > 2)

	// Compute the maximum with of property keys so we can justify everything.
	keys := diff.Keys()
	maxkey := 0
	for _, k := range keys {
		if len(k) > maxkey {
			maxkey = len(k)
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
			printPropertyValueDiff(b, title, update, indent)
		} else if same := diff.Sames[k]; shouldPrintPropertyValue(same) {
			title(indent)
			printPropertyValue(b, diff.Sames[k], indent)
		}
	}
}

func printPropertyValueDiff(b *bytes.Buffer, title func(string), diff resource.ValueDiff, indent string) {
	contract.Assert(len(indent) > 2)

	if diff.Array != nil {
		title(indent)
		a := diff.Array
		b.WriteString("[\n")
		for i := 0; i < a.Len(); i++ {
			_, newIndent := getArrayElemHeader(b, i, indent)
			title := func(id string) { printArrayElemHeader(b, i, indent) }
			if add, isadd := a.Adds[i]; isadd {
				b.WriteString(colors.SpecAdded)
				title(addIndent(newIndent))
				printPropertyValue(b, add, addIndent(newIndent))
				b.WriteString(colors.Reset)
			} else if delete, isdelete := a.Deletes[i]; isdelete {
				b.WriteString(colors.SpecDeleted)
				title(deleteIndent(newIndent))
				printPropertyValue(b, delete, deleteIndent(indent))
				b.WriteString(colors.Reset)
			} else if update, isupdate := a.Updates[i]; isupdate {
				printPropertyValueDiff(b, title, update, newIndent)
			} else {
				title(newIndent)
				printPropertyValue(b, a.Sames[i], newIndent)
			}
		}
		b.WriteString(fmt.Sprintf("%s]\n", indent))
	} else if diff.Object != nil {
		title(indent)
		b.WriteString("{\n")
		printObjectDiff(b, *diff.Object, indent+"    ")
		b.WriteString(fmt.Sprintf("%s}\n", indent))
	} else {
		// If we ended up here, the two values either differ by type, or they have different primitive values.  We will
		// simply emit a deletion line followed by an addition line.
		if shouldPrintPropertyValue(diff.Old) {
			b.WriteString(colors.SpecChanged)
			title(deleteIndent(indent))
			printPropertyValue(b, diff.Old, deleteIndent(indent))
			b.WriteString(fmt.Sprintf("%s", colors.Reset))
		}
		if shouldPrintPropertyValue(diff.New) {
			b.WriteString(colors.SpecChanged)
			title(addIndent(indent))
			printPropertyValue(b, diff.New, addIndent(indent))
			b.WriteString(fmt.Sprintf("%s", colors.Reset))
		}
	}
}

func addIndent(indent string) string    { return indent[:len(indent)-2] + "+ " }
func deleteIndent(indent string) string { return indent[:len(indent)-2] + "- " }
