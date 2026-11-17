import pulumi
import pulumi_flaky as flaky
import subprocess

config = pulumi.Config()
hook_test_file = config.require("hookTestFile")
def _retry_hook(args):
    result = subprocess.run(["touch", hook_test_file], check=False)
    return result.returncode == 0
retry_hook = pulumi.ErrorHook("retryHook", _retry_hook)
res = flaky.FlakyCreate("res", opts = pulumi.ResourceOptions(hooks=pulumi.ResourceHookBinding(on_error=[retry_hook])))
