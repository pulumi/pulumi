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
    mu.func("hello", function(req, res) { res.write("Hello, Mu!"); });

    // 2. Endpoints
    //     - API routes
    mu.http.get("/", function(req, res) {...});
    mu.http.post("/login", function(req, res) {...});

    //     - Static content
    mu.http.get("/static", mu.mw.static("./static"));

    // 3. Schedules
    mu.daily(function(req, res) {...});

    // 4. Triggers
    mu.on(salesforce.customer.added, function(req, res) {...});
    mu.on(marketo.customer.deleted, function(req, res) {...});


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

## Installation

