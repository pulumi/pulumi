import pulumi
import pulumi_basic_unions as basic_unions

# properties field is bound to union case ServerPropertiesForReplica
replica = basic_unions.ExampleServer("replica", properties=basic_unions.ServerPropertiesForReplicaArgs(
    create_mode="Replica",
    version="0.1.0-dev",
))
# properties field is bound to union case ServerPropertiesForRestore
restore = basic_unions.ExampleServer("restore", properties=basic_unions.ServerPropertiesForRestoreArgs(
    create_mode="PointInTimeRestore",
    restore_point_in_time="example",
))
