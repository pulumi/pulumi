package resource

import pulumirpc "github.com/pulumi/pulumi/sdk/v3/proto/go"

type DeleteBehaviour int32

const (
	DeleteBehaviourDelete  DeleteBehaviour = DeleteBehaviour(pulumirpc.RegisterResourceRequest_DELETE)
	DeleteBehaviourDrop                    = DeleteBehaviour(pulumirpc.RegisterResourceRequest_DROP)
	DeleteBehaviourProtect                 = DeleteBehaviour(pulumirpc.RegisterResourceRequest_PROTECT)
)

func (s DeleteBehaviour) String() string {
	switch s {
	case DeleteBehaviourDelete:
		return "delete"
	case DeleteBehaviourDrop:
		return "drop"
	case DeleteBehaviourProtect:
		return "protect"
	}
	return "unknown"
}
