import subprocess
from dataclasses import dataclass
from typing import Any
from pulumi import Output
import pulumi.workflow as workflow


@dataclass
class step_constant_args:
    pass

def register_workflows(registry: workflow.WorkflowRegistry) -> None:
    @registry.step("constant", step_constant_args)
    def register_step_constant(input: step_constant_args) -> str:
        return "done"


if __name__ == "__main__":
    workflow.run("converted", "0.1.0", register_workflows)
