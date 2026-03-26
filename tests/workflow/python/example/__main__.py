import pulumi.workflow as workflow
from workflow_program import register_workflows


if __name__ == "__main__":
    workflow.run("example", "0.0.1", register_workflows)
