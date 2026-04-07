import pulumi
import pulumi_output as output

res = output.ComplexResource("res", value=1)
pulumi.export("entriesOutput", res.output_map.apply(lambda output_map: [{"key": k, "value": v} for k, v in sorted(output_map.items())]))
pulumi.export("lookupOutput", res.output_map.apply(lambda output_map: output_map.get("x", "default")))
pulumi.export("lookupOutputDefault", res.output_map.apply(lambda output_map: output_map.get("y", "default")))
pulumi.export("entriesObjectOutput", res.output_object.apply(lambda output_object: [{"key": k, "value": v} for k, v in sorted(output_object.items())]))
pulumi.export("lookupObjectOutput", res.output_object.apply(lambda output_object: output_object.get("output", "default")))
pulumi.export("lookupObjectOutputDefault", res.output_object.apply(lambda output_object: output_object.get("missing", "default")))
