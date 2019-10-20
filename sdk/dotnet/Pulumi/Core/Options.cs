// Copyright 2016-2018, Pulumi Corporation

#nullable enable

namespace Pulumi
{
    internal class Options
    {
        public readonly string Project;
        public readonly string Stack;

        internal readonly bool DryRun;
        internal readonly bool QueryMode;
        internal readonly int Parallel;
        internal readonly string? Pwd;
        internal readonly string Monitor;
        internal readonly string Engine;
        internal readonly string? Tracing;

        public Options(bool dryRun, bool queryMode, int parallel, string project, string stack, string? pwd, string monitor, string engine, string? tracing)
        {
            DryRun = dryRun;
            QueryMode = queryMode;
            Parallel = parallel;
            Project = project;
            Stack = stack;
            Pwd = pwd;
            Monitor = monitor;
            Engine = engine;
            Tracing = tracing;
        }
    }
}
