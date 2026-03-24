import pulumi.workflow as workflow


@workflow.graph("example:index:main")
def main_graph(ctx: workflow.Context) -> None:
    workflow.trigger(
        "every-minute",
        "cloud:cron",
        {
            "schedule": "* * * * *",
            "timezone": "UTC",
        },
    )


if __name__ == "__main__":
    workflow.run()
