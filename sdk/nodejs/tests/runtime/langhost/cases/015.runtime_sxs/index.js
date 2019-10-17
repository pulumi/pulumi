// This tests the runtime's ability to be loaded side-by-side with another copy of the same runtime library.
// This is a hard and subtle problem because the runtime is configured with a bunch of state, like whether
// we are doing a dry-run and, more importantly, RPC addresses to communicate with the engine.  Normally we
// go through the startup shim to configure all of these things, but when the second copy gets loaded we don't.
// Subsequent copies of the runtime are able to configure themselves by using environment variables.

let assert = require("assert");

const sdkPath = "../../../../../";

// Load the first copy:
let pulumi1 = require(sdkPath);

// Now delete the entry in the require cache, and load up the second copy:
delete require.cache[require.resolve(sdkPath)];
delete require.cache[require.resolve(sdkPath + "/runtime")];
delete require.cache[require.resolve(sdkPath + "/runtime/config")];
delete require.cache[require.resolve(sdkPath + "/runtime/settings")];
let pulumi2 = require(sdkPath);

// Make sure they are different:
assert(pulumi1 !== pulumi2);
assert(pulumi1.runtime !== pulumi2.runtime);

// Check that various settings are equal:
assert.strictEqual(pulumi1.runtime.isDryRun(), pulumi2.runtime.isDryRun(), "pulumi1.runtime.isDryRun() !== pulumi2.runtime.isDryRun()");
assert.strictEqual(pulumi1.runtime.getProject(), pulumi2.runtime.getProject(), "pulumi1.runtime.getProject() !== pulumi2.runtime.getProject()");
assert.strictEqual(pulumi1.runtime.getStack(), pulumi2.runtime.getStack(), "pulumi1.runtime.getStack() !== pulumi2.runtime.getStack()");
assert.deepEqual(pulumi1.runtime.allConfig(), pulumi2.runtime.allConfig(), "pulumi1.runtime.allConfig() !== pulumi2.runtime.getStack()");

// Check that the two runtimes agree on the root resource
pulumi1.runtime.getRootResource().then(r => {
    pulumi2.runtime.getRootResource().then(other => {
        assert.strictEqual(r, other, "pulumi1.runtime.getRootResouce() !== pulumi2.runtime.getRootResource()");
    });
});

// allConfig should have caught this, but let's check individual config values too.
let cfg1 = new pulumi1.Config("sxs");
let cfg2 = new pulumi2.Config("sxs");
assert.strictEqual(cfg1.get("message"), cfg2.get("message"));

// Now do some useful things that require RPC connections:
pulumi1.log.info("logging via Pulumi1 works!");
pulumi2.log.info("logging via Pulumi2 works too!");
let res1 = new pulumi1.CustomResource("test:x:resource", "p1p1p1");
res1.urn.apply(urn => assert.strictEqual(urn, "test:x:resource::p1p1p1"));
let res2 = new pulumi2.CustomResource("test:y:resource", "p2p2p2");
res2.urn.apply(urn => assert.strictEqual(urn, "test:y:resource::p2p2p2"));
