import pulumi
import pulumi_simple as simple
import subprocess

def _failing_hook(args):
    subprocess.run(["false"], check=True)
failing_hook = pulumi.ResourceHook("failingHook", _failing_hook, opts=pulumi.ResourceHookOptions(ignore_errors=True))
res = simple.Resource("res", value=True,
opts = pulumi.ResourceOptions(hooks=pulumi.ResourceHookBinding(after_create=[failing_hook])))
