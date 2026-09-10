import pulumi
import pulumi_output as output

prov_elided = output.Provider("provElided", elide_unknowns=True)
prov_not_elided = output.Provider("provNotElided")
top_level_elided = output.Resource("topLevelElided", value=float(1),
opts = pulumi.ResourceOptions(provider=prov_elided))
top_level_not_elided = output.Resource("topLevelNotElided", value=float(1),
opts = pulumi.ResourceOptions(provider=prov_not_elided))
pulumi.export("topLevelElided", top_level_elided.secret_output)
pulumi.export("topLevelNotElided", top_level_not_elided.secret_output)
