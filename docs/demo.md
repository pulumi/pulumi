# Lumi Getting Started Demo Script

## Part 1: EC2 Instances and programming with Lumi

We can start with an empty Lumi script.  We've imported the AWS package.

```typescript
import * as aws from "@lumi/aws";

```

We can use completion lists to explore the AWS API for EC2 instances.


```typescript
import * as aws from "@lumi/aws";

let instance = new aws.ec2.Instance("nano", {

})
```

We see we get an error telling us we need to provide an `imageId`.  We don't want to hardcode one in, so let's just use
a helper function to get the right Amazon Linux AMI for the instance type we want.

```typescript
import * as lumi from "@lumi/lumi";
import * as aws from "@lumi/aws";

let instance = new aws.ec2.Instance("nano", {
    imageId: aws.ec2.getLinuxAMI("t2.nano"),
    instanceType: "t2.nano",
});
```

Let's try this out:

```bash
$ lumijs && lumi deploy
Deploying changes:
Applying step #1 [create]
+ aws:ec2/instance:Instance:
      [urn=test::aws/minimal:index::aws:ec2/instance:Instance::nano]
      imageId         : "ami-6869aa05"
      instanceType    : "t2.nano"
      name            : "nano"
info: resource[aws].stdout: Creating new EC2 instance resource
info: resource[aws].stdout: EC2 instance 'i-05e52d6e8b45f1a90' created; now waiting for it to become 'running'
1 total change:
    + 1 resource created
Deployment duration: 17.435391998s
```

But we can also refactor this a bit.

```typescript
import * as lumi from "@lumi/lumi";
import * as aws from "@lumi/aws";

let instanceType: aws.ec2.InstanceType = "t2.nano";

let instance = new aws.ec2.Instance("nano", {
    imageId: aws.ec2.getLinuxAMI(instanceType),
    instanceType: instanceType,
});
```

Or even:

```typescript
import * as lumi from "@lumi/lumi";
import * as aws from "@lumi/aws";

funtion makeInstance(instanceType: aws.ec2.InstanceType): aws.ec2.Instance {
    return new aws.ec2.Instance("nano", {
        imageId: aws.ec2.getLinuxAMI(instanceType),
        instanceType: instanceType,
    });
}

let instance = makeInstance("t2.nano")
```

And if we now redeploy - even though we refactored things - Lumi understands that no changes are
needed to our infrastructure.

```bash
$ lumijs && lumi deploy
Deploying changes:
info: no resources need to be updated
```

We can also add custom checks and validation - for example, make sure we don't create any expensive
EC2 instances.

```typescript
import * as aws from "@lumi/aws";


function makeInstance(instanceType: aws.ec2.InstanceType): aws.ec2.Instance {
    if (instanceType !== "t2.micro" && instanceType !== "t2.nano") {
        throw new Error("Too rich for my blood!")
    }
    return new aws.ec2.Instance("micro", {
        imageId: aws.ec2.getLinuxAMI(instanceType),
        instanceType: instanceType,
    });
}

let instance = makeInstance("t2.xlarge");
```

And now if we deploy something too expensive, we get an error.

```bash
$ lumijs && lumi deploy
error LUMI1001: An unhandled exception in aws/minimal:index's initializer occurred:
	@lumijs:lib/errors:Error{
	    name: "Error"
	    message: "Too rich for my blood!"
	}
	at aws/minimal:index:makeInstance(string)aws:ec2/instance:Instance in index.ts(8,9)
	at aws/minimal:index:.init() in index.ts(16,16)
```

But if we change that to a `t2.micro`, we see Lumi replaces the resource for us, and will cascade this impact
on other resources which depend on this instance resource as needed.

```typescript
import * as aws from "@lumi/aws";


function makeInstance(instanceType: aws.ec2.InstanceType): aws.ec2.Instance {
    if (instanceType !== "t2.micro" && instanceType !== "t2.nano") {
        throw new Error("Too rich for my blood!")
    }
    return new aws.ec2.Instance("micro", {
        imageId: aws.ec2.getLinuxAMI(instanceType),
        instanceType: instanceType,
    });
}

let instance = makeInstance("t2.micro");
```


```bash
$ lumijs && lumi deploy
Deploying changes:
Applying step #1 [replace-create] (part of a replacement change)
~+aws:ec2/instance:Instance:
      [urn=test::aws/minimal:index::aws:ec2/instance:Instance::micro]
      imageId         : "ami-6869aa05"
      instanceType    : "t2.micro"
      name            : "micro"
info: resource[aws].stdout: Creating new EC2 instance resource
info: resource[aws].stdout: EC2 instance 'i-012cfca6b7dbc9212' created; now waiting for it to become 'running'
Applying step #2 [replace]
-+aws:ec2/instance:Instance:
      [id=arn:aws:ec2:us-east-1:490047557317:instance:i-05bb64cb057939895]
      [urn=test::aws/minimal:index::aws:ec2/instance:Instance::micro]
      imageId     : "ami-6869aa05"
    - instanceType: "t2.nano"
    + instanceType: "t2.micro"
      name        : "micro"
Applying step #3 [replace-delete] (part of a replacement change)
~-aws:ec2/instance:Instance:
      [id=arn:aws:ec2:us-east-1:490047557317:instance:i-05bb64cb057939895]
      [urn=test::aws/minimal:index::aws:ec2/instance:Instance::micro]
      imageId     : "ami-6869aa05"
      instanceType: "t2.nano"
      name        : "micro"
info: resource[aws].stdout: Terminating EC2 instance 'arn:aws:ec2:us-east-1:490047557317:instance:i-05bb64cb057939895'
info: resource[aws].stdout: EC2 instance termination request submitted; waiting for it to terminate
1 total change:
    -+1 resource replaced
Deployment duration: 1m39.704930817s
```

## Part 2: Serverless resources and higher-level abstractions

In the first step, we looked at using Lumi to program raw infrastructue.  But we can apply Lumi to any programable 
infrastructure, including higher-level hosted Cloud services.  In particular, we can work with Serverless
resources like Tables, Functions and APIs.

Let's start by looking at how we can program the raw AWS resources.  We can create a simple lambda function.

```typescript
import * as aws from "@lumi/aws";

let policy = {
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": "sts:AssumeRole",
      "Principal": {
        "Service": "lambda.amazonaws.com"
      },
      "Effect": "Allow",
      "Sid": ""
    }
  ]
}

let role = new aws.iam.Role("mylambdarole", {
  assumeRolePolicyDocument: policy,
  managedPolicyARNs: [aws.iam.AWSLambdaFullAccess],
});

let lambda = new aws.lambda.Function("mylambda", {
  code: new lumi.asset.AssetArchive({
    "index.js": new lumi.asset.String("exports.handler = (ev, ctx, cb) => cb('Hello, world!');"),
  }),
  role: role,
  handler: "index.handler",
  runtime: aws.lambda.NodeJS6d10Runtime,
});
```

The resources in use here will be familiar to anyone who has worked with AWS Lambda functions before, but there are a
few nice things to notice about the Lumi programming model.  
* We can write JSON inline, and could even compose JSON object nicely as objects instead of strings within Lumi.  
* There are named constants available to refer to the known managed IAM policies.
* We could pass a .zip file on disk directly, but we can also describe the contents of the archive directly as a map
  from filenames to file contents.

But there's still a bit of boilerplate here.  So we can provide higher-level abstractions.  For example, there is an
`aws.serverless` module available with a higher-level Function abstraction.

```typescript
import * as aws from "@lumi/aws";

let lambda = new aws.serverless.Function(
  "mylambda",
  [aws.iam.AWSLambdaFullAccess],
  (ev, ctx, cb) => {
    callback(null, "Succeeed with " + ctx.getRemainingTimeInMillis() + "ms remaining.");
  }
)
```

Note that we've simplified the API a little - the default policy document is inferred, and the managed IAM policies
to apply are provided directly to the `aws.serverless.Function` component.  But more importantly, instead of 
providing a string of text for the body of the Lambda, we provide an actual LumiJS arrow function.  This function code 
will when the lambda is invoked, not during deployment.  But the code can be authored, versioned and maintained along
with the rest of the infrastructure it needs.

It can even reference captured variables:


```typescript
import * as aws from "@lumi/aws";

let hello = "Hello, world!"

let lambda = new aws.serverless.Function(
  "mylambda",
  [aws.iam.AWSLambdaFullAccess],
  (ev, ctx, cb) => {
    console.log(hello);
    callback(null, "Succeeed with " + ctx.getRemainingTimeInMillis() + "ms remaining.");
  }
)
```

We can test this out:

```bash
$ export LAMBDA=<insert lambda name from lumi deploy output here>
$ aws lambda invoke --function-name $LAMBDA --log-type Tail out.txt | jq '.LogResult' -r | base64 --decode
START RequestId: 5a948c10-4fbc-11e7-bb4f-6d1f9cca5a26 Version: $LATEST
2017-06-12T22:13:25.352Z	5a948c10-4fbc-11e7-bb4f-6d1f9cca5a26	Hello, world!
END RequestId: 5a948c10-4fbc-11e7-bb4f-6d1f9cca5a26
REPORT RequestId: 5a948c10-4fbc-11e7-bb4f-6d1f9cca5a26	Duration: 20.47 ms	Billed Duration: 100 ms 	Memory Size: 128 MB	Max Memory Used: 17 MB```
```



