import * as pulumi from "@pulumi/pulumi";
import * as conformance_component from "@pulumi/conformance-component";
import * as replaceonchanges from "@pulumi/replaceonchanges";
import * as simple from "@pulumi/simple";

// Stage 0: Initial resource creation
// Scenario 1: Schema-based replaceOnChanges on replaceProp
const schemaReplace = new replaceonchanges.ResourceA("schemaReplace", {
    value: true,
    replaceProp: true,
});
// Scenario 2: Option-based replaceOnChanges on value
const optionReplace = new replaceonchanges.ResourceB("optionReplace", {value: true}, {
    replaceOnChanges: ["value"],
});
// Scenario 3: Both schema and option - will change value
const bothReplaceValue = new replaceonchanges.ResourceA("bothReplaceValue", {
    value: true,
    replaceProp: true,
}, {
    replaceOnChanges: ["value"],
});
// Scenario 4: Both schema and option - will change replaceProp
const bothReplaceProp = new replaceonchanges.ResourceA("bothReplaceProp", {
    value: true,
    replaceProp: true,
}, {
    replaceOnChanges: ["value"],
});
// Scenario 5: No replaceOnChanges - baseline update
const regularUpdate = new replaceonchanges.ResourceB("regularUpdate", {value: true});
// Scenario 6: replaceOnChanges set but no change
const noChange = new replaceonchanges.ResourceB("noChange", {value: true}, {
    replaceOnChanges: ["value"],
});
// Scenario 7: replaceOnChanges on value, but only replaceProp changes
const wrongPropChange = new replaceonchanges.ResourceA("wrongPropChange", {
    value: true,
    replaceProp: true,
}, {
    replaceOnChanges: ["value"],
});
// Scenario 8: Multiple properties in replaceOnChanges array
const multiplePropReplace = new replaceonchanges.ResourceA("multiplePropReplace", {
    value: true,
    replaceProp: true,
}, {
    replaceOnChanges: [
        "value",
        "replaceProp",
    ],
});
// Remote component with replaceOnChanges
const remoteWithReplace = new conformance_component.Simple("remoteWithReplace", {value: true}, {
    replaceOnChanges: ["value"],
});
// Keep a simple resource so all expected plugins are required.
const simpleResource = new simple.Resource("simpleResource", {value: false});
