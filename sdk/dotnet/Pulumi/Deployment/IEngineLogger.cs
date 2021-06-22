// Copyright 2016-2019, Pulumi Corporation

using System.Threading.Tasks;

namespace Pulumi
{
    internal interface IEngineLogger
    {
        bool LoggedErrors { get; }

        Task DebugAsync(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null);
        Task InfoAsync(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null);
        Task WarnAsync(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null);
        Task ErrorAsync(string message, Resource? resource = null, int? streamId = null, bool? ephemeral = null);
    }
}
