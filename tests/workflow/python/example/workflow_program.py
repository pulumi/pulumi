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


@dataclass
class ExportedJobInput:
    message: str = "run"
    repeat: int = 2


@dataclass
class ExportedJobOutput:
    summary: str
    repeated: str
    final_file: str


@dataclass
class ExternalStepInput:
    value: str


@dataclass
class ExternalStepOutput:
    value: str


def main_graph(
    ctx: workflow.Context,
    compose_message_job_ref=None,
    to_upper_step_ref=None,
    add_suffix_step_ref=None,
) -> None:
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

    def producer_job(job: workflow.JobContext) -> Output[dict[str, Any]]:
        @job.step("produce")
        def produce_step() -> dict[str, Any]:
            cwd = os.getcwd()
            path = os.path.join(cwd, "producer-output.txt")
            with open(path, "w", encoding="utf-8") as f:
                f.write("separate-process-value")
            return {"value": "separate-process-value"}

        return produce_step

    producer_output = ctx.job("producer", producer_job)

    @ctx.job("consumer", dependencies=["producer"])
    def consumer_job(job: workflow.JobContext) -> None:
        @job.step("consume", producer_output)
        def consume_step(producer: Any) -> str:
            cwd = os.getcwd()
            with open(os.path.join(cwd, "producer-output.txt"), "r", encoding="utf-8") as f:
                file_value = f.read().strip()
            if isinstance(producer, dict):
                return f"{producer.get('value')}|{file_value}"
            return str(producer)

    @ctx.job("from-trigger")
    def from_trigger_job(job: workflow.JobContext) -> None:
        @job.step("consume", trigger_output)
        def consume_step(cron: Any) -> str:
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

    if compose_message_job_ref is None:
        ctx.job("example:compose-message", workflow.JobOptions(name="external-compose"))
    else:
        ctx.job("external-compose", compose_message_job_ref)

    @ctx.job("external-steps")
    def external_steps_job(job: workflow.JobContext) -> None:
        if to_upper_step_ref is None:
            upper = job.step("example:to-upper", ExternalStepInput(value="alpha"))
        else:
            upper = job.step("to-upper", ExternalStepInput(value="alpha"), to_upper_step_ref)

        if add_suffix_step_ref is None:
            with_suffix = job.step(
                "example:add-suffix",
                upper,
                workflow.StepOptions(name="suffix-step"),
            )
        else:
            with_suffix = job.step("suffix-step", upper, add_suffix_step_ref)

        @job.step("emit", with_suffix, dependencies=["to-upper", "suffix-step"])
        def emit_step(result: ExternalStepOutput) -> dict[str, Any]:
            return {"value": result.value}

    @ctx.job("broken-external-step")
    def broken_external_step_job(job: workflow.JobContext) -> None:
        job.step("example:missing-step", ExternalStepInput(value="boom"))

    @ctx.job("bad-external-input")
    def bad_external_input_job(job: workflow.JobContext) -> None:
        job.step("example:to-upper", {"oops": "wrong-shape"})

    @ctx.job("step-if")
    def step_if_job(job: workflow.JobContext) -> None:
        @job.step("run", filter=Output.from_input(True))
        def run_step() -> str:
            return "ran"

        @job.step("skip", filter=Output.from_input(False))
        def skip_step() -> str:
            return "should-not-run"

    @ctx.job("job-if", workflow.JobOptions(filter=Output.from_input(True)))
    def job_if_job(job: workflow.JobContext) -> None:
        @job.step("run")
        def run_step() -> str:
            return "job-ran"

    @ctx.job("job-if-disabled", workflow.JobOptions(filter=Output.from_input(False)))
    def job_if_disabled_job(job: workflow.JobContext) -> None:
        @job.step("run")
        def run_step() -> str:
            return "job-should-not-run"


def register_workflows(registry: workflow.WorkflowRegistry) -> None:
    @registry.trigger(
        "cron",
        CronTriggerInput,
    )
    def cron_trigger_mock(args: list[str]) -> CronTriggerOutput:
        if len(args) != 1:
            raise ValueError("cron trigger expects exactly one arg: timestamp")
        timestamp = dateparser.parse(args[0])
        if timestamp is None:
            raise ValueError(f"could not parse timestamp arg: {args[0]}")
        if timestamp.tzinfo is None:
            timestamp = timestamp.replace(tzinfo=timezone.utc)
        return CronTriggerOutput(timestamp=timestamp.isoformat())

    @registry.job("compose-message", ExportedJobInput)
    def compose_message_job(
        job: workflow.JobContext,
        job_input: ExportedJobInput = ExportedJobInput(),
    ) -> Output[ExportedJobOutput]:
        @job.step("seed")
        def seed_step() -> dict[str, Any]:
            cwd = os.getcwd()
            seed_text = f"{job.execution_id}:{job.workflow_version}:{job_input.message}:{job_input.repeat}"
            with open(os.path.join(cwd, "exported-job-seed.txt"), "w", encoding="utf-8") as f:
                f.write(seed_text)
            return {"seed": seed_text}

        @job.step("expand", seed_step)
        def expand_step(seed: dict[str, Any]) -> dict[str, Any]:
            cwd = os.getcwd()
            with open(os.path.join(cwd, "exported-job-seed.txt"), "r", encoding="utf-8") as f:
                seed_file = f.read().strip()
            seed_value = str(seed.get("seed", seed_file))
            repeated = " ".join([job_input.message] * job_input.repeat)
            combined = f"{seed_value}|{repeated}"
            with open(os.path.join(cwd, "exported-job-expanded.txt"), "w", encoding="utf-8") as f:
                f.write(combined)
            return {"combined": combined, "repeated": repeated}

        @job.step("finalize", expand_step)
        def finalize_step(expanded_output: dict[str, Any]) -> ExportedJobOutput:
            cwd = os.getcwd()
            with open(os.path.join(cwd, "exported-job-expanded.txt"), "r", encoding="utf-8") as f:
                expanded = f.read().strip()
            seed = expanded.split("|", 1)[0]
            repeated = str(expanded_output.get("repeated", ""))
            final_file = os.path.join(cwd, "exported-job-final.txt")
            summary = f"{seed}|{expanded}"
            with open(final_file, "w", encoding="utf-8") as f:
                f.write(summary)
            return ExportedJobOutput(summary=summary, repeated=repeated, final_file=final_file)

        return finalize_step

    @registry.step("to-upper", ExternalStepInput)
    def to_upper_step(step_input: ExternalStepInput) -> ExternalStepOutput:
        return ExternalStepOutput(value=step_input.value.upper())

    @registry.step("example:add-suffix", ExternalStepOutput)
    def add_suffix_step(step_input: ExternalStepOutput) -> ExternalStepOutput:
        return ExternalStepOutput(value=step_input.value + "!")

    @registry.graph("main")
    def registered_main_graph(ctx: workflow.Context) -> None:
        main_graph(
            ctx,
            compose_message_job_ref=compose_message_job,
            to_upper_step_ref=to_upper_step,
            add_suffix_step_ref=add_suffix_step,
        )


if __name__ == "__main__":
    workflow.run("example", "0.0.1", register_workflows)
