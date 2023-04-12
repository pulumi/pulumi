// Copyright 2016-2023, Pulumi Corporation.  All rights reserved.

using System.Collections.Generic;
using Pulumi;

return await Deployment.RunAsync(() =>
{
    return new Dictionary<string, object>
    {
        {  "exp_static", "foo" },
    };
});