// Copyright 2016-2021, Pulumi Corporation

using System.Collections.Generic;

namespace Pulumi
{
    public enum DeleteBehaviour {
        Delete = Pulumirpc.RegisterResourceRequest.Types.DeleteBehaviour.Delete,
        Drop = Pulumirpc.RegisterResourceRequest.Types.DeleteBehaviour.Drop,
        Protect = Pulumirpc.RegisterResourceRequest.Types.DeleteBehaviour.Protect,
    }
}
