import pulumi
import pulumi_call as call

default_res = call.Custom("defaultRes", value="defaultValue")
pulumi.export("defaultProviderValue", default_res.provider_value().apply(lambda call: call.result))
