// Copyright 2016-2021, Pulumi Corporation

using System;
using Pulumi.Automation.Events;

namespace Pulumi.Automation.Serialization
{
    internal class DiffKindConverter : IStringToEnumConverter<DiffKind>
    {
        public DiffKind Convert(string input) =>
            input switch
            {
                "add" => DiffKind.Add,
                "add-replace" => DiffKind.AddReplace,
                "delete" => DiffKind.Delete,
                "delete-replace" => DiffKind.DeleteReplace,
                "update" => DiffKind.Update,
                "update-replace" => DiffKind.UpdateReplace,
                _ => throw new InvalidOperationException($"'{input}' is not valid {typeof(DiffKind).FullName}"),
            };
    }
}
