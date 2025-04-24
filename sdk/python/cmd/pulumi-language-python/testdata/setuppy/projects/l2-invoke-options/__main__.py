import pulumi
import pulumi_simple_invoke as simple_invoke

explicit_provider = simple_invoke.Provider("explicitProvider")
data = simple_invoke.my_invoke_output(value="hello", opts=pulumi.InvokeOutputOptions(provider=explicit_provider, parent=explicit_provider, version="10.0.0", plugin_download_url="https://example.com/github/example"))
pulumi.export("hello", data.result)
