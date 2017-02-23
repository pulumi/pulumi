// Copyright 2016 Marapongo, Inc. All rights reserved.

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

	"github.com/marapongo/mu/pkg/compiler"
	"github.com/marapongo/mu/pkg/compiler/core"
	"github.com/marapongo/mu/pkg/compiler/errors"
	"github.com/marapongo/mu/pkg/compiler/symbols"
	"github.com/marapongo/mu/pkg/diag"
	"github.com/marapongo/mu/pkg/diag/colors"
	"github.com/marapongo/mu/pkg/encoding"
	"github.com/marapongo/mu/pkg/eval/rt"
	"github.com/marapongo/mu/pkg/graph"
	"github.com/marapongo/mu/pkg/pack"
	"github.com/marapongo/mu/pkg/resource"
	"github.com/marapongo/mu/pkg/util/cmdutil"
	"github.com/marapongo/mu/pkg/util/contract"
	"github.com/marapongo/mu/pkg/workspace"
)

var snk diag.Sink

// sink lazily allocates a sink to be used if we can't create a compiler.
func sink() diag.Sink {
	if snk == nil {
		snk = core.DefaultSink("")
	}
	return snk
}

// compile just uses the standard logic to parse arguments, options, and to locate/compile a package.  It returns the
// MuGL graph that is produced, or nil if an error occurred (in which case, we would expect non-0 errors).
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
	var pkg *pack.Package
	var g graph.Graph
	if len(args) == 0 {
		var err error
		comp, err = compiler.Newwd(opts)
		if err != nil {
			// Create a temporary diagnostics sink so that we can issue an error and bail out.
			sink().Errorf(errors.ErrorCantCreateCompiler, err)
			return nil
		}
		pkg, g = comp.Compile()
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
				return nil
			}
			g = comp.CompilePackage(pkg)
		}
	}
	return &compileResult{comp, pkg, g}
}

type compileResult struct {
	C   compiler.Compiler
	Pkg *pack.Package
	G   graph.Graph
}

// plan just uses the standard logic to parse arguments, options, and to create a snapshot and plan.
func plan(cmd *cobra.Command, args []string, existfn string, delete bool) *planResult {
	// Create a new context for the plan operations.
	ctx := resource.NewContext()

	// If we are using an existing snapshot, read in that file (bailing if an IO error occurs).
	var existing resource.Snapshot
	if existfn != "" {
		if existing = readSnapshot(ctx, existfn); existing == nil {
			return nil
		}
	}

	// If deleting, there is no need to create a new snapshot; otherwise, we will need to compile the package.
	if delete {
		return &planResult{
			compileResult: nil,
			Ctx:           ctx,
			Mugfile:       existfn,
			Existing:      existing,
			Snap:          nil,
			Plan:          resource.NewDeletePlan(ctx, existing),
		}
	} else if result := compile(cmd, args); result != nil && result.G != nil {
		// Create a resource snapshot from the compiled/evaluated object graph.
		snap, err := resource.NewGraphSnapshot(ctx, result.Pkg.Name, result.C.Ctx().Opts.Args, result.G)
		if err != nil {
			result.C.Diag().Errorf(errors.ErrorCantCreateSnapshot, err)
			return nil
		}

		var plan resource.Plan
		if existing == nil {
			// Generate a plan for creating the resources from scratch.
			plan = resource.NewCreatePlan(ctx, snap)
		} else {
			// Generate a plan for updating existing resources to the new snapshot.
			plan = resource.NewUpdatePlan(ctx, existing, snap)
		}
		return &planResult{
			compileResult: result,
			Ctx:           ctx,
			Mugfile:       existfn,
			Existing:      existing,
			Snap:          snap,
			Plan:          plan,
		}
	}

	return nil
}

type planResult struct {
	*compileResult
	Ctx      *resource.Context
	Mugfile  string            // the file from which the existing snapshot was loaded (if any).
	Existing resource.Snapshot // the existing snapshot (if any).
	Snap     resource.Snapshot // the new snapshot for this plan (if any).
	Plan     resource.Plan
}

func apply(cmd *cobra.Command, args []string, existing string, opts applyOptions) {
	if result := plan(cmd, args, existing, opts.Delete); result != nil {
		if opts.DryRun {
			// If no output file was requested, or "-", print to stdout; else write to that file.
			if opts.Output == "" || opts.Output == "-" {
				printPlan(result.Plan, opts.Detailed)
			} else {
				saveSnapshot(result.Snap, opts.Output)
			}
		} else {
			// Create an object to track progress and perform the actual operations.
			start := time.Now()
			progress := newProgress(opts.Detailed)
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
			// TODO: perform partial updates if we weren't able to perform the entire planned set of operations.
			if opts.Delete {
				contract.Assert(result.Mugfile != "")
				deleteSnapshot(result.Mugfile)
			} else {
				out := opts.Output
				if out == "" {
					out = result.Mugfile // try overwriting the existing file.
				}
				if out == "" {
					out = workspace.Mugfile // use the default file name.
				}
				contract.Assert(result.Snap != nil)
				saveSnapshot(result.Snap, out)
			}
		}
	}
}

func applyExisting(cmd *cobra.Command, args []string, opts applyOptions) {
	// Read in the snapshot argument.
	// TODO: if not supplied, auto-detect the current one.
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "fatal: missing required snapshot argument\n")
		os.Exit(-1)
	}

	apply(cmd, args[1:], args[0], opts)
}

// backupSnapshot makes a backup of an existing file, in preparation for writing a new one.  Instead of a copy, it
// simply renames the file, which is simpler, more efficient, etc.
func backupSnapshot(file string) {
	contract.Require(file != "", "file")
	// TODO: consider multiple backups (.bak.bak.bak...etc).
	os.Rename(file, file+".bak") // ignore errors.
}

// deleteSnapshot removes an existing snapshot file, leaving behind a backup.
func deleteSnapshot(file string) {
	contract.Require(file != "", "file")
	// Just make a backup of the file and don't write out anything new.
	backupSnapshot(file)
}

// readSnapshot reads in an existing snapshot file, issuing an error and returning nil if something goes awry.
func readSnapshot(ctx *resource.Context, file string) resource.Snapshot {
	m, ext := encoding.Detect(file)
	if m == nil {
		sink().Errorf(errors.ErrorIllegalMarkupExtension, ext)
		return nil
	}

	b, err := ioutil.ReadFile(file)
	if err != nil {
		sink().Errorf(errors.ErrorIO, err)
		return nil
	}

	var snap resource.MuglSnapshot
	if err = m.Unmarshal(b, &snap); err != nil {
		sink().Errorf(errors.ErrorCantReadSnapshot, file, err)
		return nil
	}
	return resource.DeserializeSnapshot(ctx, &snap)
}

// saveSnapshot saves a new MuGL snapshot at the given location, backing up any existing ones.
func saveSnapshot(snap resource.Snapshot, file string) {
	contract.Require(snap != nil, "snap")
	contract.Require(file != "", "file")

	// Make a serializable MuGL data structure and then use the encoder to encode it.
	m, ext := encoding.Detect(file)
	if m == nil {
		sink().Errorf(errors.ErrorIllegalMarkupExtension, ext)
	} else {
		if filepath.Ext(file) == "" {
			file = file + ext
		}
		ser := resource.SerializeSnapshot(snap, "")
		// TODO: this won't be a stable resource ordering; we need it to be in DAG order.
		b, err := m.Marshal(ser)
		if err != nil {
			sink().Errorf(errors.ErrorIO, err)
		} else {
			// Back up the existing file if it already exists.
			backupSnapshot(file)

			// And now write out the new snapshot file, overwriting that location.
			if err = ioutil.WriteFile(file, b, 0644); err != nil {
				sink().Errorf(errors.ErrorIO, err)
			}
		}
	}
}

type applyOptions struct {
	Delete   bool   // true if we are deleting resources.
	DryRun   bool   // true if we should just print the plan without performing it.
	Detailed bool   // true if we should print detailed information about resources and operations.
	Output   string // the place to store the output, if any.
}

// applyProgress pretty-prints the plan application process as it goes.
type applyProgress struct {
	Steps        int
	Ops          map[resource.StepOp]int
	MaybeCorrupt bool
	Detailed     bool
}

func newProgress(detailed bool) *applyProgress {
	return &applyProgress{
		Steps:    0,
		Ops:      make(map[resource.StepOp]int),
		Detailed: detailed,
	}
}

func (prog *applyProgress) Before(step resource.Step) {
	// Print the step.
	var b bytes.Buffer
	stepnum := prog.Steps + 1
	b.WriteString(fmt.Sprintf("Applying step #%v\n", stepnum))
	printStep(&b, step, !prog.Detailed, "    ")
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
		b.WriteString(fmt.Sprintf("Step #%v failed: ", stepnum))
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

func printPlan(plan resource.Plan, detailed bool) {
	// Now walk the plan's steps and and pretty-print them out.
	step := plan.Steps()
	for step != nil {
		var b bytes.Buffer

		// Print this step information (resource and all its properties).
		printStep(&b, step, detailed, "")

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
	default:
		return "  "
	}
}

func printStep(b *bytes.Buffer, step resource.Step, detailed bool, indent string) {
	// First print out the operation's prefix.
	b.WriteString(opPrefix(step.Op()))

	// Next print the resource moniker, properties, etc.
	printResource(b, step.Resource(), detailed, indent)

	// Finally make sure to reset the color.
	b.WriteString(colors.Reset)
}

func printResource(b *bytes.Buffer, res resource.Resource, detailed bool, indent string) {
	// First print out the resource type (since it is easy on the eyes).
	b.WriteString(fmt.Sprintf("%s:\n", string(res.Type())))

	// Now print out the moniker and, if present, the ID, as "pseudo-properties".
	indent += "    "
	b.WriteString(fmt.Sprintf("%s[m=%s]\n", indent, string(res.Moniker())))
	if id := res.ID(); id != "" {
		b.WriteString(fmt.Sprintf("%s[id=%s]\n", indent, string(id)))
	}

	if detailed {
		// Print all of the properties associated with this resource.
		printObject(b, res.Properties(), indent)
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
		b.WriteString(fmt.Sprintf("%s%-"+strconv.Itoa(maxkey)+"s: ", indent, k))
		printProperty(b, props[k], indent)
	}
}

func printProperty(b *bytes.Buffer, v resource.PropertyValue, indent string) {
	if v.IsNull() {
		b.WriteString("<nil>")
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
			prefix := fmt.Sprintf("%s    [%d]: ", indent, i)
			b.WriteString(prefix)
			printProperty(b, elem, fmt.Sprintf("%-"+strconv.Itoa(len(prefix))+"s", ""))
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

func isPrintableProperty(prop *rt.Pointer) bool {
	_, isfunc := prop.Obj().Type().(*symbols.FunctionType)
	return !isfunc
}
