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

// Check that we can set mocks successfully
// We don't need an actual test monitor here, just something to set and get.
class Mocks {
    newResource(args) {
        return {
            id: args.inputs.name + "_id",
            state: {
                ...args.inputs,
            },
        };
    }
    call(args) {
        return args;
    }
}

pulumi1.runtime.setMocks(new Mocks());
assert(pulumi1.runtime.getMonitor() !== undefined);
assert(pulumi2.runtime.getMonitor() !== undefined);
assert.strictEqual(pulumi1.runtime.getMonitor(), pulumi2.runtime.getMonitor());
assert(pulumi1.runtime.hasMonitor());
assert(pulumi2.runtime.hasMonitor());
