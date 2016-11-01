# Mu

The core concepts in Mu are:

1. **Stack**: A blueprint that describes a topology of cloud resources.
2. **Service**: A grouping of stateless or stateful logic with an optional API.
3. **Function**: A single stateless function that is unbundled with a single "API": invoke.
4. **Trigger**: A subscription that calls a Service or Function in response to an event.

Each Stack "instantiates" one or more Services, Functions, and Triggers to create cloud functionality.  This can include
databases, queues, containers, pub/sub topics, and overall container-based microservices, to name a few examples.  These
constructs compose nicely, such that a Service may create a Stack if it wishes to encapsulate its own resource needs.

A Service may be stateless or stateful depending on the scenario's state and scale requirements.  Multiple kinds of
Services exist and may be backed by different physical facilities: Docker containers, VMs, AWS Lambdas, and/or cloud
hosted SaaS services, to name a few.  The programming model remains consistent across them.  A Service may export APIs
for RPC-based consumption by other Services or even exported as an HTTP/2 endpoint for external consumption.

A Function is actually just a special kind of Service, however they feature prominently enough to call them out as a
top-level construct in the system.  Many of the same policies that apply to stateless Services also apply to Functions.

A rich ecosystem of Trigger events exists so that you can write reactive, serverless code where convenient without
managing whole Services.  This includes the standard ones -- like CRUD operations in your favorite NoSQL database -- in
addition to more novel ones -- like SalesForce customer events -- to deliver a uniform event-driven programming model.

Here is a brief example of Stack that represents a voting service:

    var mu = require("mu");
    
    // Create a HTTP endpoint Service that receives votes from an API:
    var voteAPI = new mu.HTTPGateway();
    var votingService = new VotingService();
    voteAPI.register(votingService);
    
    // Define a Service that creates a Stack, wires up Functions to Triggers, and exposes an API:
    class VotingService {
        constructor() {
            this.votes = new mu.Table();
            this.voteCounts = new mu.Table();
            this.votes.forEach(vote => {
                // Keep our aggregated counts up-to-date:
                this.voteCounts.updateIncrement(vote.color, vote.count);
            });
        }

        vote(info) {
            this.votes.push({ color: info.color, count: 1 });
        }
    }

Imagining this were in a single file, `voting_stack.js`, the single command

    $ mu up ./voting_stack.js

would provision all of the requisite cloud resources and make the service come to life.

This simple example demonstrates many facets:

1. Infrastructure as code and application logic living side-by-side.
2. Provisioning cloud-native resources, like `HTTPGateway` and `Table`, as though they are ordinary services.
3. Creating a custom stateless service, `VotingService`, that encapsulates cloud resources and exports a `vote` API.
4. Registering a function that runs in response to database updates using "reactive" APIs.

## Infrastructure as Code

Mu lets you express your service topologies directly in code:

    var votes = new mu.Table();
    var voteCounts = new mu.Table();

    // An endpoint that receives votes from a 3rd party service:
    var voteAPI = new mu.HTTPGateway();
    voteAPI.post("/vote").forEach(vote => {
        votes.push({ color: vote.color, count: 1 });
    });

    // A DynamoDB trigger to keep our aggregated counts up-to-date:
    votes.forEach(vote => {
        voteCounts.updateAdd(vote.color, vote.count);
    });

// TODO: map from HTTPGateway to a service API, rather than explicit GET/POST/etc. mappings.

It's even possible to wrap up these topologies, name them, and even multi-instance them.  For example, the above can
be rewritten to track polls across all 50 states, simply by defining a class and exporting it:

    class VotingService {
        constructor(state) {
            var votes = new mu.Table(state + "-Votes");
            var voteCounts = new mu.Table(state + "-VoteCounts");
            // as above ...
        }
    }

    mu.export(VotingService);

// TODO(joe): not entirely true; the HTTPGateway needs to be routed from the parent, somehow.
// TODO(joe): I am wondering whether services and stacks really ought to be the same.  Perhaps a service can contain a
//     stack, however they seem possibly distinct.  Maybe a service implies "statefulness" (i.e., Docker container with
//     an exposed RPC interface, or interfaces); OTOH, a bare constructor could be used for constructable stacks.
// TODO(joe): regardless, we need a way to incorporate Docker containers of three kinds: 1) an existing container by
//     name but no API (e.g., MongoDB); 2) an existing continer by name but give it an API; 3) a 1st class API service.

Now, we can write a new stack that aggregates many instances into a single service:

    var stateVotes = [];
    for (var state of [ "AL", "AK", ... "WI", "WY" ]) {
        stateVotes.push(new VotingService(state));
    }

Note that we may want to aggregate votes across all of the states, into one "uber" vote count.  To do that, our
aggregator can instance a new table and share it with the VotingServices that it creates:

    var stateVotes = new Map();
    var voteTotals = new mu.Table();
    for (let state of [ "AL", "AK", ... "WI", "WY" ]) {
        stateVotes.set(state, new VotingService(state, voteTotals));
    }

And of course the VotingService constructor must be altered to update the new total table:

    class VotingService {
        constructor(state, voteTotals) {
            // as above ...
            votes.forEach(vote => {
                voteCounts.updateAdd(vote.color, vote.count); // per-state counts
                voteTotals.updateAdd(vote.color, vote.count); // overall total count
            });
        }
    }

Finally, let's add an HTTP endpoint to fetch the totals:

    var results = new mu.HTTPGateway();
    results.get("/results").forEach(http => {
        http.response.write("<html>");
        http.response.write("    <table>");
        http.response.write("        <tr>");
        http.response.write("            <th>State</th>");
        http.response.write("            <th>Red</th>");
        http.response.write("            <th>Blue</th>");
        http.response.write("            <th>Green</th>");
        http.response.write("        </tr>");

        for (let stateVote of stateVotes) {
            http.response.write("        <tr>");
            http.response.write(`            <td>${stateVote[0]}</td>`);
            for (let color of [ "RED", "GREEN", "BLUE" ]) {
                http.response.write(`            <td>${stateVote[1].getTotal(color)}</td>`);
            }
            http.response.write("        </tr>");
        }

        http.response.write("    </table>");
    });

## Services as Reusable Components

Each topology may optionally export an interface that can be invoked from other services.

    var mu = require("mu");

    var customers = new mu.Table();

    var service = new mu.Service(MyInterface, new MyServer());

Examples:




