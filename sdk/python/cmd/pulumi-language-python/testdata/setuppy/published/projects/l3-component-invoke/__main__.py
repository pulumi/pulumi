import pulumi
from invokeComponent import InvokeComponent
import pulumi_config as config

prov = config.Provider("prov", name="my config")
my_component = InvokeComponent("myComponent", opts = pulumi.ResourceOptions(providers=[prov]))
