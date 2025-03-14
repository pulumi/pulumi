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
// When accessing an output of a component inside a direct call to try we should have to use apply.
//
// TODO(pulumi/pulumi#18895) When value is directly a scope traversal inside the
// output this fails to generate the "apply" call. eg if the output's internals
// are `value = componentTried.value`
const componentTried = tryOutput_(
    // @ts-ignore
    () => component1.ref,
    // @ts-ignore
    () => "fallback"
).apply(_try => _try.value);
export const tryWithOutput = componentTried;
const componentTriedNested = tryOutput_(
    // @ts-ignore
    () => component1.ref.value,
    // @ts-ignore
    () => "fallback"
);
export const tryWithOutputNested = componentTriedNested;
// Invokes produces outputs.  This output will have apply called on it and try
// utilized within the apply.  The result of this apply is already an output
// which has apply called on it again to pull out `result`
const resultContainingOutput = tryOutput_(
    // @ts-ignore
    () => simple_invoke.myInvokeOutput({
        value: "hello",
    }),
    // @ts-ignore
    () => "fakefallback"
).apply(_try => _try.result);
export const hello = resultContainingOutput;
const str = "str";
export const tryScalar = try_(
    // @ts-ignore
    () => str,
    // @ts-ignore
    () => "fallback"
);
