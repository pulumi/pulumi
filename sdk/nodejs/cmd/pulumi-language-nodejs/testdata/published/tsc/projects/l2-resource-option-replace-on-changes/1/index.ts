import * as pulumi from "@pulumi/pulumi";
import * as component from "@pulumi/component";
import * as replaceonchanges from "@pulumi/replaceonchanges";
import * as simple from "@pulumi/simple";

// Stage 1: Change properties to trigger replacements
// Scenario 1: Change replaceProp → REPLACE (schema triggers)
const schemaReplace = new replaceonchanges.ResourceA("schemaReplace", {
    value: true,
    replaceProp: false,
});
// Changed from true
// Scenario 2: Change value → REPLACE (option triggers)
const optionReplace = new replaceonchanges.ResourceB("optionReplace", {value: false}, {
    replaceOnChanges: ["value"],
});
// Scenario 3: Change value → REPLACE (option on value triggers)
const bothReplaceValue = new replaceonchanges.ResourceA("bothReplaceValue", {
    value: false,
    replaceProp: true,
}, {
    replaceOnChanges: ["value"],
});
// Scenario 4: Change replaceProp → REPLACE (schema on replaceProp triggers)
const bothReplaceProp = new replaceonchanges.ResourceA("bothReplaceProp", {
    value: true,
    replaceProp: false,
}, {
    replaceOnChanges: ["value"],
});
// Scenario 5: Change value → UPDATE (no replaceOnChanges)
const regularUpdate = new replaceonchanges.ResourceB("regularUpdate", {value: false});
// Changed from true
// Scenario 6: No change → SAME (no operation)
const noChange = new replaceonchanges.ResourceB("noChange", {value: true}, {
    replaceOnChanges: ["value"],
});
// Scenario 7: Change replaceProp (not value) → UPDATE (marked property unchanged)
const wrongPropChange = new replaceonchanges.ResourceA("wrongPropChange", {
    value: true,
    replaceProp: false,
}, {
    replaceOnChanges: ["value"],
});
// Scenario 8: Change value → REPLACE (multiple properties marked)
const multiplePropReplace = new replaceonchanges.ResourceA("multiplePropReplace", {
    value: false,
    replaceProp: true,
}, {
    replaceOnChanges: [
        "value",
        "replaceProp",
    ],
});
// Remote component from built-in provider.
const remoteWithReplace = new component.ComponentCallable("remoteWithReplace", {value: "one"}, {
    replaceOnChanges: ["value"],
});
// Keep a simple resource so all expected plugins are required.
const simpleResource = new simple.Resource("simpleResource", {value: false});
