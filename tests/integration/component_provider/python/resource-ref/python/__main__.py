import pulumi
import pulumi_command as command
import pulumi_provider as provider

command_in = command.local.Command(
    "random",
    create='echo "Hey there ${NAME}!"',
    environment={
        "NAME": "Fridolin",
    },
)

comp = provider.EchoCommand(
    "comp",
    name="Bonnie",
    command_in=command_in,
    loglevel_in=command.local.Logging.STDOUT_AND_STDERR,
)

pulumi.export("urn", comp.command_out.urn)
pulumi.export("commandOutStdout", comp.command_out.stdout)
pulumi.export("commandInStdout", comp.command_in_stdout)
pulumi.export("loglevelOut", comp.loglevel_out)
