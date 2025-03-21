import * as pulumi from "@pulumi/pulumi";

const config = new pulumi.Config();
const requiredString = config.require("requiredString");
const requiredInt = config.requireNumber("requiredInt");
const requiredFloat = config.requireNumber("requiredFloat");
const requiredBool = config.requireBoolean("requiredBool");
const requiredAny = config.requireObject<any>("requiredAny");
const optionalString = config.get("optionalString") || "defaultStringValue";
const optionalInt = config.getNumber("optionalInt") || 42;
const optionalFloat = config.getNumber("optionalFloat") || 3.14;
const optionalBool = config.getBoolean("optionalBool") || true;
const optionalAny = config.getObject<any>("optionalAny") || {
    key: "value",
};
