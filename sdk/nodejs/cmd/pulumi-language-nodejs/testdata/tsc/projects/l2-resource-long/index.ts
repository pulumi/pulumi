import * as pulumi from "@pulumi/pulumi";
import * as long from "@pulumi/long";

export = async () => {
    const small = new long.Resource("small", {value: 256});
    const min53 = new long.Resource("min53", {value: -9.007199254740992e+15});
    const max53 = new long.Resource("max53", {value: 9.007199254740992e+15});
    const min64 = new long.Resource("min64", {value: -9.223372036854775808e+18});
    const max64 = new long.Resource("max64", {value: %!v(PANIC=Format method: fatal: A failure has occurred: unexpected literal type in GenLiteralValueExpression: cty.NumberIntVal(9.223372036854775807e+18) (main.pp:17,13-32))});
    const uint64 = new long.Resource("uint64", {value: %!v(PANIC=Format method: fatal: A failure has occurred: unexpected literal type in GenLiteralValueExpression: cty.NumberIntVal(1.8446744073709551615e+19) (main.pp:21,13-33))});
    const huge = new long.Resource("huge", {value: %!v(PANIC=Format method: fatal: A failure has occurred: unexpected literal type in GenLiteralValueExpression: cty.NumberIntVal(2.0000000000000000001e+19) (main.pp:25,13-33))});
    return {
        huge: %!v(PANIC=Format method: fatal: A failure has occurred: unexpected literal type in GenLiteralValueExpression: cty.NumberIntVal(2.0000000000000000001e+19) (main.pp:29,13-33)),
        roundtrip: huge.value,
        result: pulumi.all([small.value, min53.value, max53.value, min64.value, max64.value, uint64.value, huge.value]).apply(([smallValue, min53Value, max53Value, min64Value, max64Value, uint64Value, hugeValue]) => smallValue + min53Value + max53Value + min64Value + max64Value + uint64Value + hugeValue),
    };
}
