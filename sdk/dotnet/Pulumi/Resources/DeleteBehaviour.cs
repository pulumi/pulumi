// Copyright 2016-2021, Pulumi Corporation

using System.Collections.Generic;

namespace Pulumi
{
    public enum DeleteBehaviour {
        Delete = Pulumirpc.DeleteBehaviour.Delete,
        Drop = Pulumirpc.DeleteBehaviour.Drop,
        Protect = Pulumirpc.DeleteBehaviour.Protect,
    }
}
