# Mu

The core concepts in Mu are:

1. **Stack**: A blueprint that describes a topology of cloud resources.
2. **Service**: A grouping of stateless or stateful logic with an optional API.
3. **Function**: A single stateless function that is unbundled with a single "API": invoke.
4. **Trigger**: A subscription that calls a Service or Function in response to an event.

// TODO(joe): map to Kube concepts; do we need "more" (e.g., Controller)?

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

Here is a brief example of Stack that represents a voting service, authored in Node.js:

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

Let's quickly look at two slight variants of this same code.

First, we could have written this without a `VotingService` whatsoever.  Although real code tends to be complex and
encapsulation of resources and state encourages this sort of organization, we can simply do it entirely with Functions:

    var mu = require("mu");
    
    var votes = new mu.Table();
    var voteCounts = new mu.Table();

    // Create a HTTP endpoint Service that receives votes from an API:
    var voteAPI = new mu.HTTPGateway();
    voteAPI.post("/vote", (req, res) => {
        votes.push({ color: req.info.color, count: 1 });
    });

    // Keep our aggregated counts up-to-date:
    votes.forEach(vote => {
        voteCounts.updateIncrement(vote.color, vote.count);
    });

This makes for nice "minimal code" demos.  Defining a class helps to encapsulate resources and logic, but has another
benfit.  This brings us to our second variant, which is to "multi-instance" our service.

Imagine that we want to offer voting for each of the 50 states.  We can simply create many `VotingService`s:

    var mu = require("mu");

    // Create a HTTP endpoint Service that receives votes from an API:
    var voteAPI = new mu.HTTPGateway();

    for (var state of [ "AL", "AK", ... "WI", "WY" ]) {
        var votingService = new VotingService();
        voteAPI.register(`/${state}`, votingService);
    }

    // VotingService is unchanged from above.

Instead of a single `/vote` endpoint, there will now be endpoints for each of the 50 states -- `/vote/AL`, `/vote/AK`,
..., `/vote/WI`, and `/vote/WY` -- each with its own votes and voteCounts tables.  Notice that we didn't even have to
change the definition of `VotingService` to do this.  Of course, we may want to, in order to perform state-specific
logic, name its internal resources to have state prefixes in their names, and so on.  But this demonstrates the power
of reusability when we define Services in the manner shown above.

## A Teardown

Although a developer wrote very simple code in the introductory example, there is a fair bit of machinery behind making
it work.  In fact, the specific details differ greatly depending on which cloud orchestration fabric you are targeting
(such as AWS native, Google Cloud native, Kubernetes, Docker Swarm, and so on); moreover, multiple backends are
available for some providers (such as AWS CloudFormation or Terraform when targeting AWS native deployments).

To illustrate how the projections work, let's pick a single provider: AWS native using CloudFormation.

The above example contains two Stacks:

1. The top-level Stack.
2. The inner Stack allocated by `VotingService`'s constructor.

Each of these maps to a single "Stack" in AWS's CloudFormation terminology.  To generate them, run:

    $ mu build ./voting_stack.js

Inside of each Stack, there are a number of resources.  Let's first take a look at the top-level Stack:

1. A native AWS API Gateway.
2. A native AWS Lambda, containing the code for `vote` wired up to said API Gateway at `/vote`.

Next, the inner Stack allocated by `VotingService`:

1. Two native AWS DynamoDB "no-SQL" tables: votes and voteCounts.
2. A native AWS Lambda, containing the callback wired up to the votes DynamoDB table.

In this particular example, there is little advantage to having two Stacks, since we only ever create one
`VotingService`.  It's important to remember, however, that Services can be multi-instanced, as in our 50 states
example, so they must remain distinct.  Of course, many AWS resources may be generated in like fashion: S3 buckets,
Route53 DNS entries, and so on.  Furthermore, stateful Services will end up requiring EC2 VMs and/or Docker containers.

In addition to generating the metadata, the code is prepared for deployment.  This includes some massaging of the code
so that it is in the requisite form (e.g., Docker images, S3 tarballs for AWS Lambdas, and so on).

If you were to change the code, rerunning `mu build` would regenerate the modified Stack.  Leveraging the usual
techniques for applying diffs to an existing environment allows incremental changes to be made, rather than needing to
destroy and redeploy the entire cluster again.  Blue green, staged deployments and high availability are both supported.

For simple scenarios, developers may not care what goes on behind the scenes.  In those cases, just writing code like
the above and running the CLI is perfect.  For complex scenarios, on the other hand -- particularly in multi-tenant
environments, hybrid or on-premise clouds, and/or when IT organizations want more control over things -- the contents of
this section become more important.  In fact, organizations may wish to manage the cloud deployment artifacts more
intently, possibly even editing them by hand, and/or checking them into source control.  Moreover, it's even possible to
author these definitions by hand and map them to the program using a `mu.yaml` file that sits in the middle.

