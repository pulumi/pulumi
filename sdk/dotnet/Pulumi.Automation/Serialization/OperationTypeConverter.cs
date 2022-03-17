// Copyright 2016-2021, Pulumi Corporation

using System;

namespace Pulumi.Automation.Serialization
{
    internal class OperationTypeConverter : IStringToEnumConverter<OperationType>
    {
        public OperationType Convert(string input) =>
            input switch
            {
                "create" => OperationType.Create,
                "create-replacement" => OperationType.CreateReplacement,
                "delete" => OperationType.Delete,
                "delete-replaced" => OperationType.DeleteReplaced,
                "replace" => OperationType.Replace,
                "same" => OperationType.Same,
                "update" => OperationType.Update,
                "read" => OperationType.Read,
                "read-replacement" => OperationType.ReadReplacement,
                "refresh" => OperationType.Refresh,
                "discard" => OperationType.ReadDiscard,
                "discard-replaced" => OperationType.DiscardReplaced,
                "remove-pending-replace" => OperationType.RemovePendingReplace,
                "import" => OperationType.Import,
                "import-replacement" => OperationType.ImportReplacement,
                _ => throw new InvalidOperationException($"'{input}' is not valid {typeof(OperationType).FullName}"),
            };
    }
}
