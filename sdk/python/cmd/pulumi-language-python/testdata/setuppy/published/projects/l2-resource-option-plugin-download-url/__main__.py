import pulumi
import pulumi_simple as simple

with_default_url = simple.Resource("withDefaultURL", value=True)
with_explicit_default_url = simple.Resource("withExplicitDefaultURL", value=True)
with_custom_url1 = simple.Resource("withCustomURL1", value=True,
opts = pulumi.ResourceOptions(plugin_download_url="https://custom.pulumi.test/provider1"))
with_custom_url2 = simple.Resource("withCustomURL2", value=False,
opts = pulumi.ResourceOptions(plugin_download_url="https://custom.pulumi.test/provider2"))
