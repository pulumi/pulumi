// Copyright 2016-2019, Pulumi Corporation

using System.Text.Json;

namespace Pulumi
{
    /// <summary>
    /// Represents an <see cref="Input{T}"/> value that wraps a <see cref="JsonElement"/>.
    /// </summary>
    public sealed class InputJson : Input<JsonElement>
    {
        public InputJson() : this(Output.Create(default(JsonElement)))
        {
        }

        private InputJson(Output<JsonElement> output)
            : base(output)
        {
        }

        #region common 

        public static implicit operator InputJson(string json)
            => JsonDocument.Parse(json);

        public static implicit operator InputJson(JsonDocument document)
            => document.RootElement;

        public static implicit operator InputJson(JsonElement element)
            => Output.Create(element);

        public static implicit operator InputJson(Output<string> json)
            => json.Apply(j => JsonDocument.Parse(j));

        public static implicit operator InputJson(Output<JsonDocument> document)
            => document.Apply(d => d.RootElement);

        public static implicit operator InputJson(Output<JsonElement> element)
            => new InputJson(element);

        #endregion
    }
}
