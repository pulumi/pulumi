import pulumi
import pulumi_scalar_returns as scalar_returns

pulumi.export("secret", scalar_returns.invoke_secret_output(value="goodbye"))
pulumi.export("array", scalar_returns.invoke_array_output(value="the word"))
pulumi.export("map", scalar_returns.invoke_map_output(value="hello"))
pulumi.export("secretMap", scalar_returns.invoke_map_output(value="secret"))
