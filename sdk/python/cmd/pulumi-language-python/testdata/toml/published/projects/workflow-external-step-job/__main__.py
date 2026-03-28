import subprocess
from dataclasses import dataclass
from typing import Any
from pulumi import Output
import pulumi.workflow as workflow


import pulumi_simple_step_workflow

@dataclass
class step_invert_args:
    input: bool

@dataclass
class job_build_args:
    input: bool

def register_workflows(registry: workflow.WorkflowRegistry) -> None:
    @registry.step("invert", step_invert_args)
    def register_step_invert(input: step_invert_args) -> bool:
        return not input.input
    @registry.job("build", job_build_args)
    def register_job_build(job: workflow.JobContext, args: job_build_args) -> Output[bool]:
        build_invert_output = pulumi_simple_step_workflow.step_simple_step_workflow_invert(job, pulumi_simple_step_workflow.step_simple_step_workflow_invert_args(input=args.input), workflow.StepOptions(name="invert"))
        return build_invert_output


if __name__ == "__main__":
    workflow.run("converted", "0.1.0", register_workflows)
