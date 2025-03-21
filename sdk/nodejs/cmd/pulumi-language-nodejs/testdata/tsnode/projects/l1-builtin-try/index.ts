import * as pulumi from "@pulumi/pulumi";

function tryOutput_(
	...fns: Array<() => pulumi.Input<any>>
): pulumi.Output<any> {
	if (fns.length === 0) {
		throw new Error("try: all parameters failed");
	}
	const [fn, ...rest] = fns;
	try {
		return pulumi.output(fn()).apply(result => result !== undefined ? result : tryOutput_(...rest));
	} catch {
		return tryOutput_(...rest);
	}
	throw new Error("try: all parameters failed");
}


function try_(
	...fns: Array<() => any>
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


const config = new pulumi.Config();
const aMap = config.requireObject<Record<string, string>>("aMap");
export const plainTrySuccess = try_(
    () => aMap.a,
    () => "fallback"
);
export const plainTryFailure = try_(
    () => aMap.b,
    () => "fallback"
);
const aSecretMap = pulumi.secret(aMap);
export const outputTrySuccess = tryOutput_(
    () => aSecretMap.a,
    () => "fallback"
);
export const outputTryFailure = tryOutput_(
    () => aSecretMap.b,
    () => "fallback"
);
const anObject = config.requireObject<any>("anObject");
export const dynamicTrySuccess = tryOutput_(
    () => anObject.a,
    () => "fallback"
);
export const dynamicTryFailure = tryOutput_(
    () => anObject.b,
    () => "fallback"
);
const aSecretObject = pulumi.secret(anObject);
export const outputDynamicTrySuccess = tryOutput_(
    () => aSecretObject.apply(aSecretObject => aSecretObject.a),
    () => "fallback"
);
export const outputDynamicTryFailure = tryOutput_(
    () => aSecretObject.apply(aSecretObject => aSecretObject.b),
    () => "fallback"
);
