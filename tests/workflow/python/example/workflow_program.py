import pulumi.workflow as workflow
import random
import string
import dateparser
import os
from dataclasses import dataclass
from datetime import timezone
from typing import Any
from pulumi import Output

@dataclass
class CronTriggerInput:
    schedule: str
    timezone: str


@dataclass
class CronTriggerOutput:
    timestamp: str


def main_graph(ctx: workflow.Context) -> None:
    def cron_filter(value: Any) -> bool:
        if not isinstance(value, dict) or "timestamp" not in value:
            return False
        return str(value["timestamp"]).endswith("00:00+00:00")

    trigger_output = ctx.trigger(
        "every-minute",
        "cron",
        CronTriggerInput(schedule="* * * * *", timezone="UTC"),
        options=workflow.TriggerOptions(filter=cron_filter),
    )

    @ctx.job("main")
    def main_job(job: workflow.JobContext) -> None:
        @job.step("run")
        def run_step() -> str:
            print("running main step", flush=True)
            return "".join(random.choices(string.ascii_lowercase + string.digits, k=12))

    @ctx.job("from-trigger", trigger_output)
    def from_trigger_job(job: workflow.JobContext, cron: Any) -> None:
        @job.step("consume")
        def consume_step() -> str:
            print(f"consuming trigger payload: {cron}", flush=True)
            if isinstance(cron, dict) and "timestamp" in cron:
                return str(cron["timestamp"])
            return str(cron)

    @ctx.job("three-step")
    def three_step_job(job: workflow.JobContext) -> None:
        @job.step("first")
        def first_step() -> dict[str, Any]:
            cwd = os.getcwd()
            with open(os.path.join(cwd, "workflow-shared.txt"), "w", encoding="utf-8") as f:
                f.write("first=alpha")
            return {"first": "alpha"}

        @job.step("second")
        def second_step() -> dict[str, Any]:
            cwd = os.getcwd()
            shared_path = os.path.join(cwd, "workflow-shared.txt")
            with open(shared_path, "r", encoding="utf-8") as f:
                first_value = f.read().strip()
            second_value = f"{first_value}|second=beta"
            with open(os.path.join(cwd, "workflow-second.txt"), "w", encoding="utf-8") as f:
                f.write(second_value)
            return {"second": second_value}

        @job.step("third", first_step)
        def third_step(first) -> dict[str, Any]:
            cwd = os.getcwd()
            with open(os.path.join(cwd, "workflow-second.txt"), "r", encoding="utf-8") as f:
                second_value = f.read().strip()
            result = {
                "third": "gamma",
                "first": first["first"] if isinstance(first, dict) and "first" in first else first,
                "jobResult": f"{second_value}|third=gamma",
            }
            return result

        return Output.all(second_step, third_step).apply(lambda results: {
            "second": results[0]["second"],
            "third": results[1]["third"],
            "result": results[1]["jobResult"],
        })


def register_workflows(registry: workflow.WorkflowRegistry) -> None:
    def cron_trigger_mock(args: list[str]) -> CronTriggerOutput:
        if len(args) != 1:
            raise ValueError("cron trigger expects exactly one arg: timestamp")
        timestamp = dateparser.parse(args[0])
        if timestamp is None:
            raise ValueError(f"could not parse timestamp arg: {args[0]}")
        if timestamp.tzinfo is None:
            timestamp = timestamp.replace(tzinfo=timezone.utc)
        return CronTriggerOutput(timestamp=timestamp.isoformat())

    registry.trigger(
        "cron",
        CronTriggerInput,
        cron_trigger_mock,
    )
    registry.graph("main", main_graph)


if __name__ == "__main__":
    workflow.run("example", "0.0.1", register_workflows)
