import * as pulumi from "@pulumi/pulumi";
import * as long from "@pulumi/long";

export = async () => {
    const small = new long.Resource("small", {value: BigInt(256)});
    const min53 = new long.Resource("min53", {value: BigInt(-9.007199254740992e+15)});
    const max53 = new long.Resource("max53", {value: BigInt(9.007199254740992e+15)});
    const min64 = new long.Resource("min64", {value: BigInt(-9.223372036854775808e+18)});
    const max64 = new long.Resource("max64", {value: BigInt("9223372036854775807")});
    const uint64 = new long.Resource("uint64", {value: BigInt("18446744073709551615")});
    const huge = new long.Resource("huge", {value: BigInt("20000000000000000001")});
    return {
        huge: BigInt("20000000000000000001"),
        roundtrip: huge.value,
        result: pulumi.all([small.value, min53.value, max53.value, min64.value, max64.value, uint64.value, huge.value]).apply(([smallValue, min53Value, max53Value, min64Value, max64Value, uint64Value, hugeValue]) => smallValue + min53Value + max53Value + min64Value + max64Value + uint64Value + hugeValue),
    };
}
