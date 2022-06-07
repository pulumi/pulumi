// Copyright 2016-2021, Pulumi Corporation

using System;
using System.Collections.Generic;
using System.Threading.Tasks;
using Pulumi.Serialization;
using Pulumi.Testing;

namespace Pulumi.Tests.Mocks
{
    /// <summary>
    /// Supports testing that Invoke gets skippped when passed unknown inputs,
    /// instead of failing and breaking preview.
    ///
    /// See pulumi/pulumi#8322.
    /// </summary>
    public sealed class Issue8322
    {
        public class ReproStack : Stack
        {
            [Output("result")]
            public Output<string> Result { get; private set; }

            public ReproStack()
            {
                // First we need an unknown output (in preview). Use
                // Instance here as Output.CreateUnknown is not public.
                var instance = new Instance("instance1", new InstanceArgs());
                var unknown = instance.PublicIp;
                // To reproduce the bug, we call an Invoke with unknowns.
                var result = GetRole.Invoke(new GetRoleInvokeArgs() { Name = unknown });
                this.Result = result.Apply(r => r.Id);
            }
        }

        public class ReproMocks : IMocks
        {
            public Task<object> CallAsync(MockCallArgs args)
            {
                throw new Exception("CallAsync should not be called");
            }

            public Task<(string? id, object state)> NewResourceAsync(MockResourceArgs args)
                => args.Type switch
                {
                    "aws:ec2/instance:Instance" => Task.FromResult<(string?, object)>(
                        ("i-1234567890abcdef0",
                         new Dictionary<string, object>
                         {
                             { "publicIp", Constants.UnknownValue },
                         })),
                    _ => throw new Exception($"Unknown resource {args.Type}")
                };
        }
    }
}
