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
