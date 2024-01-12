// Copyright 2016-2024, Pulumi Corporation.  All rights reserved.
//go:build !all
// +build !all

package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	cloud_workflows "github.com/Frassle/cloud_workflows/sdk/go"
	cf "github.com/pulumi/cloud/sdk/v3/go/workflow"
	wf "github.com/pulumi/pulumi/sdk/v3/go/pulumi/workflow"
)

// Welcome to Pulumi workflows!
//
// This is _a_ proposal for what I think workflows could look like. It's made a few choices about behavior and semantics
// that dictate the API shape which I think we could decided to choose differently, but it feels the closest to what
// Pulumi models is like today (imperative looking, constructor like declarations, ergonomic over totally type safe).
//
// The workflow system, like esc, is split in two parts. The Evaluator (OSS) and the Scheduler (Proprietary).
//
// Users write workflow programs very much like Pulumi program using the OSS libraries and tools (e.g. the evaluator
// lets you run a job in the workflow locally) and then pushes that program to Cloud (or more likely points Cloud to the
// repo ala deployments). Cloud then runs the scheduler which makes use of the Evaluator to build an understanding of
// the workflow and execute individual jobs, but Cloud has all the smarts of when a trigger (more on those soon) is to
// be fired, and will track all jobs complete and run their dependent jobs as possible till the whole workflow has run.
//
// I haven't built the whole Evaluator system for the below (clearly given it's only been like a day), but I have
// thought pretty hard that it should be possible to build the below with the semantics intended. As mentioned this is
// _very_ similar to my last job so I've got some experience to refer to here about what things did/didn't work.

func main() {
	// A workflow program runs in a context, very similar to a normal Pulumi program. The context defines a few things.
	// Firstly A connection back to the Evaluator which is sort of the equivalent of the resource monitor. Secondly the
	// id of the workflow. This is split into three, the first is the name of the workflow which is pulled from a
	// Pulumi.yaml file (same as project name in a normal Pulumi program pretty much). The second is an id for the
	// version of the workflow, when Cloud gets new source code it would change this id. The third is an id for the
	// execution instance of the workflow (probably a UUID) which cloud would set for every job running due to one
	// trigger. When running locally OSS those two later id's would default to blank and a random UUID but could be
	// explicitly set by the user if needed.
	wf.Run(func(ctx *wf.Context) error {
		// The first thing you need in a workflow is some sort of Trigger. Workflow execution _always_ starts from ONE
		// trigger, but the workflow code can define multiple triggers to wait on.
		//
		// A trigger must show up in the first preview of the workflow, it's a programming bug if you try and run a
		// trigger in an output context (i.e. inside the Apply equivalent). All their inputs are plain values, but their
		// outputs are Outputs.
		//
		// Triggers (like all the nodes in a workflow) have a unique name. These names can be nested via subgraphs (more
		// on that later) such that you get things like "/all my triggers/8am daily".
		cron := cf.OnCron("8am daily", &cf.CronArgs{
			// The schema of a trigger is defined by the scheduler. Notice that these types for cron are from the cloud
			// library not from the core pulumi library. As far as the evaluator is concerned all a trigger is is a blob
			// of plain input and output data. It's the scheduler which knows what triggers it can fire on and what
			// shape they are.
			Schedule: "0 8 * * *",
		})

		// The second thing you need in a workflow are Jobs. At it's a core a Job is just a function. It has a name just
		// like Triggers (the namespace is shared, so must be unique across the whole workflow definition). Unlike
		// triggers the whole point of a job is to do something given an Output that happened.
		print := wf.Job[time.Duration]("print time", wf.JobArgs[time.Duration]{
			// The input can be of any type, this is a fully generic feature. In this case we're just using on of the
			// output properties from the cron trigger to get the time it actually fired.
			Input: cron.Time,
			Step: func(ctx context.Context, t time.Duration) error {
				// The code for a job is just plain code, and importantly does not use Output inside. The output is
				// resolved for the Input field, passed to the Step function as plain and you shouldn't then do anything
				// with outputs again.
				fmt.Printf("The cron ran at: %s", t)
				return nil
			},
		})

		// A job can depend on another job.
		wf.Job[wf.JobStatus]("ok", wf.JobArgs[wf.JobStatus]{
			// The only output of a Job is it's status, a small struct of info the scheduler knows given it ran the job.
			// My working assumption is we're not building any sort of state into the workflow system, so if you want to
			// propagate values across jobs you need to solve state yourself. I think this is fine for MVP but something
			// we probably want to revisit.
			Input: print.Result,
			Step: func(ctx context.Context, arg wf.JobStatus) error {
				fmt.Printf("print was ok: %s", arg.Success)
				return nil
			},
		})

		// Result by default is for all results (job failures are included), if you only want to depend on the job if it
		// passed use SuccessResult.

		// A workflow can define a second trigger. Each execution only runs due to any one trigger. This is another
		// trigger type that cloud could provide. To run a workflow in response to a stack update happening.
		update := cf.OnStackUpdate("update", &cf.StackUpdateArgs{
			// Project could be optional to filter to a specific project, or to fire on all projects in the org.
			Project: "networking",
			// Likewise you could have a stack argument to filter to only fire on a given stack. If the built in
			// filtering isn't powerful enough the Evaluator can run a filter function allowing the user general code to
			// make a decision.
			Filter: func(ctx context.Context, state cf.StackUpdateState) (bool, error) {
				if strings.HasPrefix(state.Stack, "dev-") {
					// This filters out firing this workflow for any stack starting with "dev-".
					return false, nil
				}
				return true, nil
			},
		})

		// A job can depend on multiple events via a disjunctive select.
		cronOrUpdate := pulumi.Select(cron.Time, update.Stack)
		refresh := wf.Job[pulumi.Choice[time.Duration, string]](
			"refresh", &wf.JobArgs[pulumi.Choice[time.Duration, string]]{
				Input: cronOrUpdate,
				Step: func(ctx context.Context, choice pulumi.Choice[time.Duration, string]) error {
					stack := "default_prod_stack"
					if choice.IsFirst() {
						fmt.Printf("refresh fired because of cron at %s", choice.First())
					} else {
						stack := choice.Second()
						fmt.Printf("refresh fired because of stack update %s", stack)
					}

					// Imagine actually doing a stack refresh via automation api here

					return nil
				},
			})

		// A job can depend on multiple events via a conjunctive Map2/All.
		updateAndRefresh := pulumi.Map2(update.Stack, refresh.SuccessResult, func(s string, r wf.JobResult) {
			return fmt.Printf("%s:%b", s, r.Success)
		})
		// A job taking updateAndRefresh as an input won't run till both the update trigger has fired and the refresh
		// job has run. It is a user programming mistake to write jobs that depend on two trigger paths both being
		// fired, those jobs just won't ever run.

		// Given we're just dealing with generic Output values you could also write an output value that just transforms
		// a jobs JobResult into something more useful. E.g. assuming we have S3 state setup somewhere we could do
		// something like have a job write to S3 key'd by the current workflow ID then take the same ID and the
		// JobResult (to wait for the job to finish) to then read from S3.
		statefulJob := wf.Job[string]("stateful", wf.JobArgs[string]{
			Input: ctx.ExecutionId,
			Step: func(ctx context.Context, id string) error {
				// Write something to s3 key'd by id
				return nil
			},
		})
		statefulJobResult := pulumi.Map2(ctx.ExecutionId, statefulJob.SuccessResult, func(id string, _ wf.JobResult) string {
			// lookup the s3 data by id and read it in for other jobs to use
			return result
		})

		// The next major feature is subgraphs. Subgraphs allow you to build up child graphs where the shape doesn't
		// _have_ to be known at first preview. They look a lot like a job except inside their function it's fine to
		// make more calls to Job/Graph (and Step if the input would resolve at first preview).
		subgraph := wf.Graph[string, pulumi.Output[int]]("inner", &wf.GraphArgs[string, pulumi.Output[int]]{
			Input: statefulJobResult,
			Step: func(ctx *wf.Context, arg string) pulumi.Output[int] {
				// We can build another job now based on arg
				var result pulumi.Output[int]
				if arg == "small" {
					small := wf.Job[time.Duration]("small", wf.JobArgs[string]{
						Input: wf.ExecutionId,
						Step: func(ctx context.Context, id string) error {
							// Write some small calculation to S3
							return nil
						},
					})
					// map over small.Result and ExecutionId to read the result from S3
					result = pulumi.Map2(small.SuccessResult, wf.ExecutionId, func(_ wf.JobResult, id string) int {
						return 0
					})
				} else {
					count := scanf.Int(arg)
					// Different graph shape based on arg. Also showing that we can iterate.
					jobs := make([]wf.JobResult, 0, count)
					for i := 0; i < count; i += 1 {
						i := i
						job := wf.Job[time.Duration](fmt.Sprintf("big%d", i), wf.JobArgs[string]{
							Input: wf.ExecutionId,
							Step: func(ctx context.Context, id string) error {
								// Write some big calculation to S3 key'd by id and i
								return nil
							},
						})
						jobs = append(jobs, job.SuccessResult)
					}
					result = pulumi.All(jobs)
					// Again map over id and all the jobs to get the result from S3.
					result = pulumi.Map2(result, wf.ExecutionId, func(_ []wf.JobResult, id string) int {
						return 0
					})
				}
				return result
			},
		})

		// Finally we've got workflow plugins like MLCs (MLGs?) which let you share definitions for triggers, jobs,
		// invokes and graphs for anything where the input/output can go over the PropertyValue wire protocol.
		cloud_workflows.AHelpfulJob("helper", &cloud_workflows.AHelpfulJobArgs{
			ExecutionId: ctx.ExecutionId,
			Mode:        "be helpful",
		})
		// The inputs to this can be a mix of plain or output'y. They get serialized over to the plugin at preview an
		// execution time so the plugin can tell the graph engine the shape of the job (or graph, or trigger) and then
		// later at execution time get it to actually run the job step code. This can piggy back a lot of our learnings
		// from MLCs.

		return nil
	})
}

// Finally finally some notes on usage and how things would actually execute. I expect users to use three main OSS parts
// while building workflow programs.
//
// 1. 'pulumi workflow preview' - this is the same as what the scheduler would run as well. 'preview' runs the program
// in 'describe' mode, not trying to execute and steps and not resolving any Outputs (e.g. there is no workflow
// execution id even when doing first preview). This just executes as much as possible so the graph engine can see all
// the triggers and the shape of jobs. This would probably pretty print something for users, but also emit a serialized
// form of what it found for the scheduler to consume to setup triggers with.
//
// 2. 'pulumi workflow trigger state.json /path/to/trigger' - This would just be used by local devs to be able to easily
// mock a trigger being set. As mentioned earlier I think triggers could be defined by workflow plugins (of which Cloud
// would just be our first and major one). Inside the plugin it has some code to generate mock triggers, think 'go
// rapid' style sort of test case generation. The scheduler would not use this because it has actual real trigger data
// to write to the workflow input, and users could do that as well but this random example generation feels useful.
//
// 3. 'pulumi workflow run state.json /path/to/job' - This would try to run the single job given by the path. This might
// error that the input state doesn't have enough dependencies marked as done to run the job. But either by running them
// manually or doing state edits (more on that in a bit) this would run the step and write the JobResult to the state
// file.
//
// For state editing we'd probably go with the scheduler just editing the state file directly (it's a simple schema and
// it knows the trigger data). It's also not a great fit to try and pass property maps via CLI args. We may want to
// consider 'workflow state' commands if users want like an easy way to mark jobs as done for local debugging.
//
// In terms of execution as mentioned a lot there's two modes to run workflow programs in. 'describe' and 'execute',
// I'll talk about this in terms of what the scheduler needs to do, because local usage is probably just a subset of
// that.
//
// On first getting a workflow program the scheduler needs to run `preview`. This starts the graph engine (a lot like a
// resource monitor) and runs the user workflow program (we can pretty much re-use the language hosts we already have
// for this). We would set a single envvar to tell the SDK where to connect to the graph engine. It then runs the
// program and for each trigger and job send a gRPC request to register that. If the engine was started with state
// (which it won't on first preview, but other previews will) then the engine will reply to those registrations with the
// Output data for that node (or unknown if it's not yet marked done in state). This build a graph description file that
// the scheduler can look at. This first preview lets the scheduler see all triggers, so it can then internally setup
// how to wait for those triggers.
//
// {
//   "triggers" [ {
//     "path": "/path/to/trigger",
//     "type": "cloud:cron",
//     "inputs": {
//       "schedule": "0 8 * * *"
//     }
//   } ],
//   "jobs": [ {
//     "path": "/path/to/job",
//     "dependencies": ["/path/to/trigger"]
//   } ]
//   // similar for "graphs"
// }
//
// When a trigger fires the scheduler needs to write a state file with that trigger data, this looks fairly simple given
// that for any given execution there's only one trigger fired:
//
// {
//   "trigger": {
//     "path": "/path/to/trigger",
//     "state": { time: "1200", ... }
//   }
// }
//
// We don't have an execution ID yet because the very first thing we want to do is "run" the trigger if it has a filter
// to decide if this deserves a full execution or not. This just uses `workflow run state.json /path/to/trigger` to use
// the evaluator to run the filter function on the trigger object (n.b. The scheduler can skip this if preview didn't
// have `"trigger": true`). `run` _always_ has to run the users full graph function (thus why they really want to be
// small and fast to build the graph) and then if the path resolved to a trigger/job it then runs that function. For a
// trigger we just need the bool result or error, 'run' writes it's result to a file (path given as a cli flag) which
// would look something like:
//
// { error: "some error string" } or { filter: true }
//
// This is as opposed to using exit codes to try and represent both error responses and filter responses.
//
// If the filter passed the scheduler then wants to go an allocate an actual execution id for this run. It then looks at
// the preview data from the first run to see if any jobs or graphs should be run given that '/path/to/trigger' is
// complete. We _could_ put this logic in the evaluator (something like `pulumi workflow ready state.json
// /path/to/thing`) but there's maybe value in not giving away the dependency scheduler.
//
// If any jobs can now be run then the scheduler should trigger them with a new state file including the execution id:
//
// {
//   "execution": "SOMEUUID",
//   "trigger": {
//     "path": "/path/to/trigger",
//     "state": { time: "1200", ... }
//   }
// }
//
// As jobs complete the scheduler adds them to the state file for the evaluator to use when running later jobs. If any
// graph nodes in the preview data become ready to run the scheduler needs to re-run 'workflow preview' but with the
// state file passed so that the evaluator can build more of the graph function (more Outputs will resolve).
//
// The above example only showed a simple dependency array `[ "/path/to/trigger" ]` but dependencies can also be tagged
// objects:
//
// { "tag": "or", "dependencies": [...] }, { "tag": "strict", "dependencies": [...]}
//
// "or" should be considered 'done' if _any_ of its subdependencies are considered done. "strict" should be considered
// done only if it's subdependencies are done successfully. It might also make sense to have "unstrict" to undo deeper
// levels of "strict".
//
// The scheduler keeps iterating these steps until there's nothing left to execute (i.e. no more jobs or graphs become
// ready from all their dependencies being done).
//
// That's all folks!
