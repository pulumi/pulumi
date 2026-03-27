import subprocess
from dataclasses import dataclass
from typing import Any
from pulumi import Output
import pulumi.workflow as workflow


@dataclass
class job_build_args:
    input: str

def register_workflows(registry: workflow.WorkflowRegistry) -> None:
    @registry.job("build", job_build_args)
    def register_job_build(job: workflow.JobContext, args: job_build_args) -> Output[str]:
        build_first_output = job.step("first", args.input, lambda input: input)
        build_second_output = job.step("second", build_first_output, lambda input: f"{input} text")
        build_third_output = job.step("third", build_second_output, lambda input: f"{input} tail")
        return Output.all(first=build_first_output, third=build_third_output).apply(lambda o: f"{o['first']} + {o['third']}")


if __name__ == "__main__":
    workflow.run("converted", "0.1.0", register_workflows)
