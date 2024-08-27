import pulumi
import pulumi_long as long

small = long.Resource("small", value=256)
min53 = long.Resource("min53", value=-9.007199254740992e+15)
max53 = long.Resource("max53", value=9.007199254740992e+15)
min64 = long.Resource("min64", value=-9.223372036854776e+18)
max64 = long.Resource("max64", value=9223372036854775807)
uint64 = long.Resource("uint64", value=18446744073709551615)
huge = long.Resource("huge", value=20000000000000000001)
pulumi.export("huge", 20000000000000000001)
pulumi.export("roundtrip", huge.value)
pulumi.export("result", pulumi.Output.all(
    smallValue=small.value,
    min53Value=min53.value,
    max53Value=max53.value,
    min64Value=min64.value,
    max64Value=max64.value,
    uint64Value=uint64.value,
    hugeValue=huge.value
).apply(lambda resolved_outputs: resolved_outputs['smallValue'] + resolved_outputs['min53Value'] + resolved_outputs['max53Value'] + resolved_outputs['min64Value'] + resolved_outputs['max64Value'] + resolved_outputs['uint64Value'] + resolved_outputs['hugeValue'])
)
