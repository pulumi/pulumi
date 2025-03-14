import * as pulumi from "@pulumi/pulumi";

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


const str = "str";
const config = new pulumi.Config();
const object = config.requireObject("object");
export const trySucceed = try_(
    // @ts-ignore
    () => str,
    // @ts-ignore
    () => object.a,
    // @ts-ignore
    () => "fallback"
);
export const tryFallback1 = try_(
    // @ts-ignore
    () => object.a,
    // @ts-ignore
    () => "fallback"
);
export const tryFallback2 = try_(
    // @ts-ignore
    () => object.a,
    // @ts-ignore
    () => object.b,
    // @ts-ignore
    () => "fallback"
);
export const tryMultipleTypes = try_(
    // @ts-ignore
    () => object.a,
    // @ts-ignore
    () => object.b,
    // @ts-ignore
    () => 42,
    // @ts-ignore
    () => "fallback"
);
