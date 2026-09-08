import pulumi
import pulumi_output as output

r = output.Resource("r", value=float(1))
pulumi.export("wrapped", pulumi.Output.secret(r.output))
