import pulumi.workflow as workflow
import random
import string


def main_graph(ctx: workflow.Context) -> None:
    ctx.trigger(
        "every-minute",
        "cloud:cron",
        {
            "schedule": "* * * * *",
            "timezone": "UTC",
        },
    )

    @ctx.job("main")
    def main_job(job: workflow.JobContext) -> None:
        @job.step("run")
        def run_step() -> str:
            print("running main step", flush=True)
            return "".join(random.choices(string.ascii_lowercase + string.digits, k=12))


def register_workflows(registry: workflow.WorkflowRegistry) -> None:
    registry.graph("main", main_graph)


if __name__ == "__main__":
    workflow.run("example", "0.0.1", register_workflows)
