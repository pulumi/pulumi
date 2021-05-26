using System;
using System.Collections.Generic;
using Pulumi;
using System.Diagnostics.CodeAnalysis;

namespace Pulumi.MadeupPackage.Codegentest
{
    static class InvokeOptionsExtensions
    {
        public static InvokeOptions WithVersion(this InvokeOptions? options)
        {
            if (options?.Version != null)
            {
                return options;
            }
            return new InvokeOptions
            {
                Parent = options?.Parent,
                Provider = options?.Provider,
                Version = Version,
            };
        }

        private readonly static string version = "1.0.0";

        public static string Version => version;
    }

    static class InputExtensions
    {
        public static Input<T?> Nullable<T>(this Input<T>? input) where T : class
        {
            if (input == null)
            {
                return default(T);
            }
            else
            {
                return input.Apply(v => (T?)v);
            }
        }

        public static Input<T?> NullableStruct<T>(this Input<T>? input) where T : struct
        {
            if (input == null)
            {
                return default(T);
            }
            else
            {
                return input.Apply(v => (T?)v);
            }
        }
    }

    static class CollectionExtension
    {
        public static List<T> ToList<T>(this IEnumerable<T> elements)
        {
            return new List<T>(elements);
        }
    }

    public class Box
    {
        [AllowNull]
        public Object Value { get; }

        public Box([AllowNull] Object value)
        {
            Value = value;
        }

        public void Set(Object target, string propertyName)
        {
            var v = this.Value;
            if (v != null)
            {
                var p = target.GetType().GetProperty(propertyName);
                if (p != null)
                {
                    p.SetValue(target, v);
                }
            }
        }
    }

    public static class OutputExtensions
    {
        public static Output<Box> Box<T>([AllowNull] this Input<T> input)
        {
            if (input == null)
            {
                return Output.Create( new Box(null));
            }
            else
            {
                return input.Apply(v => new Box(v));
            }
        }
    }
}
