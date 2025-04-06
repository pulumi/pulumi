import pulumi
import pulumi_call as call

explicit_prov = call.Provider("explicitProv", value="explicitProvValue")
explicit_res = call.Custom("explicitRes", value="explicitValue",
opts = pulumi.ResourceOptions(provider=explicit_prov))
pulumi.export("explicitProviderValue", explicit_res.provider_value().apply(lambda call: call.result))
pulumi.export("explicitProvFromIdentity", explicit_prov.identity().apply(lambda call: call.result))
pulumi.export("explicitProvFromPrefixed", explicit_prov.prefixed(prefix="call-prefix-").apply(lambda call: call.result))
