import pulumi
import pulumi_simple as simple
import subprocess

config = pulumi.Config()
hook_test_file = config.require("hookTestFile")
hook_preview_file = config.require("hookPreviewFile")
def _create_hook(args):
    subprocess.run(["touch", hook_test_file])
create_hook = pulumi.ResourceHook("createHook", _create_hook)
def _preview_hook(args):
    subprocess.run(["touch", f"{hook_preview_file}_{args.name}"])
preview_hook = pulumi.ResourceHook("previewHook", _preview_hook, opts=pulumi.ResourceHookOptions(on_dry_run=True))
res = simple.Resource("res", value=True,
opts = pulumi.ResourceOptions(hooks=pulumi.ResourceHookBinding(before_create=[create_hook, preview_hook])))
