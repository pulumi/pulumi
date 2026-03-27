import subprocess
from dataclasses import dataclass
from typing import Any
from pulumi import Output
import pulumi.workflow as workflow


@dataclass
class job_build_args:
    input: bool

def register_workflows(registry: workflow.WorkflowRegistry) -> None:
    @registry.job("build", job_build_args)
    def register_job_build(job: workflow.JobContext, args: job_build_args) -> Output[bool]:
        return Output.from_input(not args.input)


if __name__ == "__main__":
    workflow.run("converted", "0.1.0", register_workflows)
