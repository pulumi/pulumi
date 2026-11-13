import pulumi
import pulumi_unionref as unionref

github = unionref.Github("github")
gitlab = unionref.Gitlab("gitlab")
pipeline = unionref.Pipeline("pipeline",
    source=github,
    program=unionref.ProgramArgs(
        source=gitlab,
    ),
    sources=[
        github,
        gitlab,
    ])
