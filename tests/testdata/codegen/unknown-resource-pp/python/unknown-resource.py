import pulumi
from typing import Any
import pulumi_unknown as unknown

provider = pulumi.providers.Unknown("provider")
main = unknown.Main("main",
    first=hello,
    second={
        foo: bar,
    })
from_module: list[Any] = []
from_module_range: list[dict[str, Any]] = [{"value": i} for i in range(0, 10)]
for range in from_module_range:
    from_module.append(unknown.eks.Example(f"fromModule-{range['value']}", associated_main=main.id))
pulumi.export("mainId", main["id"])
pulumi.export("values", from_module["values"]["first"])
