import subprocess
from dataclasses import dataclass
from typing import Any
from pulumi import Output
import pulumi.workflow as workflow


@dataclass
class step_invert_args:
    input: bool

def register_workflows(registry: workflow.WorkflowRegistry) -> None:
    @registry.step("invert", step_invert_args)
    def register_step_invert(input: step_invert_args) -> bool:
        return not input.input


if __name__ == "__main__":
    workflow.run("converted", "0.1.0", register_workflows)
