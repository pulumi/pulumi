import pulumi.workflow as workflow


def main_graph(ctx: workflow.Context) -> None:
    ctx.trigger(
        "every-minute",
        "cloud:cron",
        {
            "schedule": "* * * * *",
            "timezone": "UTC",
        },
    )


def register_workflows(registry: workflow.WorkflowRegistry) -> None:
    registry.graph("main", main_graph)


if __name__ == "__main__":
    workflow.run("example", "0.0.1", register_workflows)
