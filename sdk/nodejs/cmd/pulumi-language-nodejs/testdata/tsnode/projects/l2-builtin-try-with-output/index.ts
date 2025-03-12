import * as pulumi from "@pulumi/pulumi";
import * as component from "@pulumi/component";
import * as simple_invoke from "@pulumi/simple-invoke";

function tryOutput_(
    ...fns: Array<() => unknown>
): pulumi.Output<any> {
    for (const fn of fns) {
        try {
            const result = fn();
            if (result === undefined) {
                continue;
            }
            return pulumi.output(result);
        } catch (e) {
            continue;
        }
    }
    throw new Error("try: all parameters failed");
}


function try_(
    ...fns: Array<() => unknown>
): any {
    for (const fn of fns) {
        try {
            const result = fn();
            if (result === undefined) {
                continue;
            }
            return result;
        } catch (e) {
            continue;
        }
    }
    throw new Error("try: all parameters failed");
}


const component1 = new component.ComponentCustomRefOutput("component1", {value: "foo-bar-baz"});
export const tryWithOutput = tryOutput_(
    // @ts-ignore
    () => component1.ref,
    // @ts-ignore
    () => "failure"
);
// This should result in a try who's result is an output. 
// It seems to generate the correct function, likely due to checking independently when generating 
// the call site, but when generating it is treated as a regular value, and not an output that would require .apply.
// The generated code is marked with this comment.
// Ideas? Debugging Tips?
const resultContainingOutput = tryOutput_(
    // @ts-ignore
    () => simple_invoke.myInvokeOutput({
        value: "hello",
    })
);
export const hello = resultContainingOutput?.result;
const resultContaingOutputWithoutTry = simple_invoke.myInvokeOutput({
    value: "hello",
});
export const helloNoTry = resultContaingOutputWithoutTry.result;
const str = "str";
export const tryScalar = try_(
    // @ts-ignore
    () => str,
    // @ts-ignore
    () => "fallback"
);
