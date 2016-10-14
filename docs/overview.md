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

