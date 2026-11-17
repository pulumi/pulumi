import pulumi
from typing import Any
import pulumi_unknown as unknown

provider = pulumi.providers.Unknown("provider")
main = unknown.Main("main",
    first=hello,
    second={
        foo: bar,
    })
from_module: list[unknown.eks.Example] = []
for from_module_range in [{"value": i} for i in range(0, 10)]:
    from_module.append(unknown.eks.Example(f"fromModule-{from_module_range['value']}", associated_main=main.id))
pulumi.export("mainId", main["id"])
pulumi.export("values", from_module["values"]["first"])
