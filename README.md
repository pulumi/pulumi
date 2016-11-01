# Mu

Mu is a framework for building [serverless](https://en.wikipedia.org/wiki/Serverless_computing) applications, APIs, and
services.  It combines the best of FaaS and PaaS to support a broad range of scenarios with minimal infrastructure.

## Overview

Mu makes two major categories of serverless software easy to create and manage.

First, Mu enables *lightweight API gateways*.  These are serverless web applications, used either to serve up web pages,
expose REST APIs, or a combination of the two.  Mu lets you express your endpoints and logic without needing to worry
about the associated infrastructure (such as VMs, containers, web servers, and load balancers).

Second, Mu enables *on-demand computations* of multiple kinds.  This includes batch tasks or streams processors, which
can be initiated either by a time-based schedule or reactive event-based trigger.

Mu unifies how you author the logic for these scenarios, in addition to how you manage the associated software packages.
Mu is polyglot, and supports many languages out of the box: Node.js, Go, Python, Java, and others.  Mu is also cloud-
neutral, supporting many execution environments: local/on-premise, AWS Lambda, Google Cloud Functions, and more.

## Examples

Before diving into the details, let's see some Mu examples, written in Node.js.

Here is the simplest possible Mu program:

    var mu = require("mu");
    mu.func(function(req, res) { res.write("Hello, Mu!"); });

We have simply imported the `"mu"` package and registered a single callback function.  Although this is a simple program
akin to what AWS Lambda gives you, it doesn't showcase Mu's ability to manage whole serverless applications.

A more comprehensive Mu program might look something like this:

    var mu = require("mu");

    // 1. Functions
    var hello = new mu.Function("hello", function(req, res) { res.write("Hello, Mu!"); });

    // 2. Endpoints
    var http = new mu.HTTPGateway();

    //     - API routes
    http.get("/").forEach(function(req, res) {...});
    http.post("/login").forEach(function(req, res) {...});

    //     - Static content
    http.get("/static").forEach(mu.mw.static("./static"));

    // 3. Schedules
    mu.Timer.daily.forEach(function(req, res) {...});

    // 4. Triggers
    salesforce.customers.forEach(function(req, res) {...});
    marketo.customer.deleted.forEach(function(req, res) {...});

Now things have gotten interesting!  This example demonstrates a few ways to register a serverless function:

1. **Functions**: The `func` routine registers a function with a name.  Although they aren't automatically hooked up to
   anything, such functions can be invoked by handle or by name using the command line and web interfaces.

2. **Endpoints**: `http` exposes a HTTP endpoint using a standard route syntax.  This can be used to create web
   pages or REST APIs using your favorite frameworks, deployed atop an entirely serverless architecture.  The usual
   HTTP verbs are available -- like `get` and `post` -- and the response may be a custom serverless function or
   middleware.  `mu.static` is a middleware function to serve static content such as HTML, CSS, and JavaScript.

3. **Schedules**: `daily` runs the given function once/day.  The obvious relatives like `hourly` also exist, in
   addition to the general-purpose `schedule` function which accepts a cron-like schedule.

4. **Triggers**: Lastly, `on` subscribes to a named event -- there are many to choose from! -- and runs the function
   with its payload anytime that event occurs.  Streams-based events that automatically batch are also available.

Mu will manage deploying, wiring up, and running all of these functions.  Below we will see how.

## Installing Mu

## Managing Mu Projects

Managing your Mu projects and deployments is easy to do with the `mu` command line.

The simplest case is when a single Git repo contains a Mu package.  In that case, simply run:

    $ mu init

This registers with the Mu Cloud so that anytime changes to your Git repo are published, automatic CI/CD processes
will provision, test, and, provided that works, deploy your changes to your public cloud of choice.

Of course, all of this can be done manually, if you do not wish to use the Mu Cloud for management.

For illustrative purposes, let's break it down.

First, we can initialize a Mu project without attaching to the Mu Cloud:

    $ mu init --detached

If you later decide to use the Mu Cloud service, we can login and then attach our current project:

    $ mu login
    $ mu attach

The `init` command provisions the metadata for your project in the form of a `mu.yaml` file.  In our case, this will
start off minimally, just with some handy package manager-like metadata (like name, description, and language).

It's possible to run Mu functions locally.  For example, to run the `"hello"` function from above, just type:

    $ mu run --func hello

This executes the function a single time.  To pass a payload to it, you may use stdin, a literal, or a filename:

    $ mu run --func hello - < payload.json
    $ mu run --func hello --in "{ \"some\": \"data\" }"
    $ mu run --func hello --in @payload.json

// TODO(joe): more examples; e.g., HTTP endpoints, schedules, triggers, etc.

Notice that we are running functions directly.  To instead activate a project's routes, run the following command:

    $ mu listen

This fires up all routes and awaits the stimuli that runs them.  This tests out your project end-to-end; hit ^C to stop
awaiting.  If you'd like to activate specific routes, simply list them by type, name, or both:

    $ mu listen --http             # run all HTTP endpoints
    $ mu listen --http "/login"    # run just the /login HTTP endpoint
    $ mu listen --http --schedules # run all HTTP endpoints and schedules

// TODO(joe): more examples.

Finally, if you are using Mu Tests, you can run them locally to validate your changes:

    $ mu test

// TODO(joe): more details; run specific subsets of tests, integration with other test frameworks, etc.

All of this is running locally on your machine.  Of course, once we are ready to try it out in our production
environment, we need a way of deploying the changes.  Mu handles this for us too.  Although the Mu Cloud mentioned
earlier does everything in a turnkey style, we can break apart the steps and perform them by hand.  They are:

1. `mu build`: Building a project first generates all the requisite Mu metadata.
2. `mu package`: Packaging a project generates the target cloud environment's native formats.
3. `mu deploy`: Deploying a project takes all of the above and applies it to the target environment.

Let's walk these steps, in order.

The first step is to build your package's metadata:

    $ mu build

This command gathers up all of the metadata necessary to fully map out all of the routes, etc. defined in code.

Next, we will prepare the data required to apply our deployment to the target environment.  The specific steps and
formats here will differ based on the target.  For example, when deploying to AWS, the steps are governed by the AWS
API Gateway and Lambda metadata formats.  The `--provider` flag selects the target; if omitted, the Mu Cloud is used:

    $ mu package                    # for the Mu Cloud
    $ mu package --provider aws     # or, for the AWS Cloud
    $ ...                           # or, ...

The final step is to perform a deployment.  This step is "intelligent" in that, by default, it does smart
delta-patching, creating, updating, and/or deleting only those resources necessary to reach the desired target state:

    $ mu deploy                     # to the Mu Cloud
    $ mu deploy --provider aws      # or, to the AWS Cloud
    $ ...                           # or, ...

// TODO(joe): do you need to specify the --provider for package *and* deploy?

Note that the AWS provider requires that you've [configured your AWS credentials properly](
http://docs.aws.amazon.com/cli/latest/topic/config-vars.html).  As the `deploy` command executes, it will print out
the changes made to your environment, in addition to any relevant endpoints.  If you wish to try out the command without
actually modifying your environment, you can add the `--dry-run` flag:

    $ mu deploy --provider aws --dry-run

// TODO(joe): link to the more sophistciated deployment options, e.g. using Terraform.

Note that you can skip manually running `build`, `package`, and `deploy` commands; if you run `package` without having
run `build`, it will be run automatically; and if you run `deploy` without having run `package`, it too will be run
automatically.  Therefore, if you want to skip the first two steps and simply perform a new deployment, just run:

    $ mu deploy --...

Mu also supports the notion of multiple environments (production, staging, test, etc).  If you are using the Mu
Cloud-managed deployment option, you may attach to any number of branches, each of which will get its own isolated
environment.  To do so, simply change branches, and run the `mu attach` command:

    $ mu checkout -b stage
    $ mu attach

If you are performing deployments by hand, you may specify the environment name in the `deploy` command:

    $ mu deploy --provider aws --environment stage

// TODO(joe): it seems unwise to assume "production" is the default.  Maybe this should be configurable too.

In addition to all of those commands, you can list what's in production (`mu ls`), what is actively running (`mu ps`),
and obtain logs or performance metrics for functions that have run or are running (`mu logs` and `mu metrics`).

