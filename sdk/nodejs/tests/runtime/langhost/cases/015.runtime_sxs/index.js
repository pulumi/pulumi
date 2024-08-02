// This tests the runtime's ability to be loaded side-by-side with another copy of the same runtime library.
// This is a hard and subtle problem because the runtime is configured with a bunch of state, like whether
// we are doing a dry-run and, more importantly, RPC addresses to communicate with the engine.  Normally we
// go through the startup shim to configure all of these things, but when the second copy gets loaded we don't.
// Subsequent copies of the runtime are able to configure themselves by using environment variables.

let assert = require("assert");
let path = require("path");

const sdkPath = "../../../../../";

// Load the first copy:
let pulumi1 = require(sdkPath);

// Now delete the entries in the require cache, and load up the second copy:
const resolvedSdkPath = path.dirname(require.resolve(sdkPath));
Object.keys(require.cache).forEach((path) => {
    if (path.startsWith(resolvedSdkPath)) {
        delete require.cache[path];
    }
});
let pulumi2 = require(sdkPath);

// Make sure they are different:
assert(pulumi1 !== pulumi2, "pulumi1 !== pulumi2");
assert(pulumi1.runtime !== pulumi2.runtime, "pulumi1.runtime !== pulumi2.runtime");

// Check that various settings are equal:
assert.strictEqual(
    pulumi1.runtime.isDryRun(),
    pulumi2.runtime.isDryRun(),
    "pulumi1.runtime.isDryRun() !== pulumi2.runtime.isDryRun()",
);
assert.strictEqual(
    pulumi1.runtime.getProject(),
    pulumi2.runtime.getProject(),
    "pulumi1.runtime.getProject() !== pulumi2.runtime.getProject()",
);
assert.strictEqual(
    pulumi1.runtime.getStack(),
    pulumi2.runtime.getStack(),
    "pulumi1.runtime.getStack() !== pulumi2.runtime.getStack()",
);
assert.deepStrictEqual(
    pulumi1.runtime.allConfig(),
    pulumi2.runtime.allConfig(),
    "pulumi1.runtime.allConfig() !== pulumi2.runtime.getStack()",
);

// Check that the two runtimes agree on the stack resource
let stack1 = pulumi1.runtime.getStackResource();
let stack2 = pulumi2.runtime.getStackResource();
assert.strictEqual(stack1, stack2, "pulumi1.runtime.getStackResource() !== pulumi2.runtime.getStackResource()");

// allConfig should have caught this, but let's check individual config values too.
let cfg1 = new pulumi1.Config("sxs");
let cfg2 = new pulumi2.Config("sxs");
assert.strictEqual(cfg1.get("message"), cfg2.get("message"));

// Try and set a stack transformation
function transform1(args) {
    args.props["runtime1"] = 1;
    return { props: args.props, opts: args.opts };
}
function transform2(args) {
    args.props["runtime2"] = 2;
    return { props: args.props, opts: args.opts };
}

pulumi1.runtime.registerStackTransformation(transform1);
pulumi2.runtime.registerStackTransformation(transform2);

// Now do some useful things that require RPC connections:
pulumi1.log.info("logging via Pulumi1 works!");
pulumi2.log.info("logging via Pulumi2 works too!");
let res1 = new pulumi1.CustomResource("test:x:resource", "p1p1p1");
res1.urn.apply((urn) => assert.strictEqual(urn, "test:x:resource::p1p1p1"));
let res2 = new pulumi2.CustomResource("test:y:resource", "p2p2p2");
res2.urn.apply((urn) => assert.strictEqual(urn, "test:y:resource::p2p2p2"));

// Both resources should have the stack transforms applied
res1.runtime1.apply((value) => assert.strictEqual(value, 1));
res1.runtime2.apply((value) => assert.strictEqual(value, 2));
res2.runtime1.apply((value) => assert.strictEqual(value, 1));
res2.runtime2.apply((value) => assert.strictEqual(value, 2));

pulumi1.runtime.registerResourcePackage("test1", {
    version: "0.0.1",
});
pulumi2.runtime.registerResourcePackage("test2", {
    version: "0.0.2",
});
let test1pulumi1 = pulumi1.runtime.getResourcePackage("test1");
assert.strictEqual(test1pulumi1.version, "0.0.1");
let test1pulumi2 = pulumi2.runtime.getResourcePackage("test1");
assert.strictEqual(test1pulumi2.version, "0.0.1");
let test2pulumi1 = pulumi1.runtime.getResourcePackage("test2");
assert.strictEqual(test2pulumi1.version, "0.0.2");
let test2pulumi2 = pulumi2.runtime.getResourcePackage("test2");
assert.strictEqual(test2pulumi2.version, "0.0.2");
