import pulumi
import pulumi_using_dashes as using_dashes

main = using_dashes.Dash("main", stack="dev")
