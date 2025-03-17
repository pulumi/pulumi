import * as pulumi from "@pulumi/pulumi";
import * as component from "@pulumi/component";
import * as simple_invoke from "@pulumi/simple-invoke";

function tryOutput_(
    ...fns: Array<() => pulumi.Input<unknown>>
): pulumi.Output<any> {
    if (fns.length === 0) {
	       throw new Error("try: all parameters failed");
    }

    const [fn, ...rest] = fns;
    let resultOutput: pulumi.Output<any> | undefined;
    try {
        const result = fn();
        if (result === undefined) {
            return tryOutput_(...rest);
        }
        resultOutput = pulumi.output(result);
    } catch {
	       return tryOutput_(...rest);
    }

    // @ts-ignore
	   return resultOutput;
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
// TODO(pulumi/pulumi#18895) When value is directly a scope traversal inside the
// output this fails to generate the "apply" call. eg if the output's internals
// are `value = componentTried.value`
//
// Apply is used when a resource output attribute is accessed.
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
// Invokes produces outputs. 
// This output will have apply called on it and try utilized within the apply.
// The result of this apply is already an output which has apply called on it
// again to pull out `result`
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
