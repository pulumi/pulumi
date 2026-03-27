import subprocess
from typing import Any
from pulumi import Output
import pulumi.workflow as workflow


def main_graph(ctx: workflow.Context) -> None:
    @ctx.job("build")
    def main_build_job(job: workflow.JobContext) -> None:
        @job.step("echo")
        def main_build_echo_step() -> Any:
            return not input

def register_workflows(registry: workflow.WorkflowRegistry) -> None:
    @registry.step("echo", bool)
    def register_step_echo(input: bool) -> bool:
        return not input
    @registry.graph("main")
    def register_main_graph(ctx: workflow.Context) -> None:
        main_graph(ctx)


if __name__ == "__main__":
    workflow.run("converted", "0.1.0", register_workflows)
