// This test validates that the automation API (and it's local state) has global state.
//
// This is the automation API equivalent of the 015.runtime_sxs test.

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

// Export an async function instead of using top-level await
module.exports = (async () => {
    const stack = await pulumi1.automation.LocalWorkspace.createOrSelectStack(
        {
            stackName: "automation_sxs",
            projectName: "tests",
            program: () => {
                pulumi1.runtime.setConfig("k", "v"); // Set config in one load
                assert(pulumi2.runtime.getConfig("k") === "v"); // Read from the config in the other copy
            },
        },
        {
            projectSettings: {
                name: "tests",
                runtime: "nodejs",
                backend: {
                    url: `file://.`,
                },
            },
            envVars: {
                PULUMI_CONFIG_PASSPHRASE: "",
            },
        },
    );

    await stack.up({ onOutput: console.info });
})();
