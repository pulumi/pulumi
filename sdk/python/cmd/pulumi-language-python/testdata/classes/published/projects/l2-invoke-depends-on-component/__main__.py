import pulumi
import pulumi_component as component

target = component.ComponentCustomRefOutput("target", value="checked")
data = component.identity_output(input="reachable", opts=pulumi.InvokeOutputOptions(depends_on=[target]))
pulumi.export("echoed", data.result)
