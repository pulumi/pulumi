import * as pulumi from "@pulumi/pulumi";
import * as simple from "@pulumi/simple";
import * as simple_invoke from "@pulumi/simple-invoke";

const first = new simple.Resource("first", {value: false});
// assert that resource second depends on resource first
// because it uses .secret from the invoke which depends on first
const second = new simple.Resource("second", {value: simple_invoke.secretInvokeOutput({
    value: "hello",
    secretResponse: first.value,
}).apply(invoke => invoke.secret)});
const third = new simple_invoke.StringResource("third", {text: "third"});
// third.text is known during preview, but third does not exist yet. SDKs must
// infer the dependency on third from the invoke's arguments and skip the
// invoke while third's ID is unknown: getText fails if it is called before
// third has been created.
const data = simple_invoke.getTextOutput({
    text: third.text,
});
const fourth = new simple_invoke.StringResource("fourth", {text: data.result});
