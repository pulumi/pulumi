import * as pulumi from "@pulumi/pulumi";
import * as specialchars from "@pulumi/specialchars";

// The nested object type has property names containing characters that are not legal
// identifier characters in most languages: "@timestamp" and "entity.name".
const first = new specialchars.Thing("first", {value: "hello"});
export const itemOutput = first.item;
export const invoked = specialchars.getItemOutput({
    value: "world",
});
