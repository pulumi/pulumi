import pulumi
import pulumi_simple as simple
import subprocess


def not_implemented(msg):
    raise NotImplementedError(msg)

def _panic_hook(args):
    subprocess.run([not_implemented("hook panic")], check=True)
panic_hook = pulumi.ResourceHook("panicHook", _panic_hook)
res = simple.Resource("res", value=True,
opts = pulumi.ResourceOptions(hooks=pulumi.ResourceHookBinding(after_create=[panic_hook])))
