import subprocess
from dataclasses import dataclass
from typing import Any
from pulumi import Output
import pulumi.workflow as workflow


@dataclass
class step_touch_file_args:
    input_file: str

def register_workflows(registry: workflow.WorkflowRegistry) -> None:
    @registry.step("touch-file", step_touch_file_args)
    def register_step_touch_file(input: step_touch_file_args) -> str:
        return subprocess.check_output(f"touch \"{input.input_file}\"", shell=True, text=True).strip()


if __name__ == "__main__":
    workflow.run("converted", "0.1.0", register_workflows)
