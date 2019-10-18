// Copyright 2016-2018, Pulumi Corporation

#nullable enable

using System;
using System.Diagnostics.CodeAnalysis;

namespace Pulumi
{
    internal interface IInput
    {
        IOutput ToOutput();
    }

    public class Input<T> : IInput
    {
        private readonly Output<T> _outputValue;

        private Input(Output<T> outputValue)
            => _outputValue = outputValue ?? throw new ArgumentNullException(nameof(outputValue));

        public static implicit operator Input<T>([MaybeNull]T value)
            => Output.Create(value);

        public static implicit operator Input<T>(Output<T> value)
            => new Input<T>(value);

        public static implicit operator Output<T>(Input<T> input)
            => input._outputValue;

        public Output<T> ToOutput()
            => this;

        IOutput IInput.ToOutput()
            => ToOutput();
    }
}


//using Google.Protobuf.WellKnownTypes;
//using System;
//using System.Threading.Tasks;

//namespace Pulumi {
//    interface IInput {
//        Task<OutputState<object>> GetValueAsOutputStateAsync();
//    }

//    public sealed class Input<T> : IInput{
//        T m_rawValue;
//        Task<T> m_task;
//        Output<T> m_output;

//        private Input() {}

//        public static implicit operator Input<T>(T rawValue) {
//            return new Input<T> {
//                m_rawValue = rawValue,
//            };
//        }

//        public static implicit operator Input<T>(Task<T> task) {
//            return new Input<T> {
//                m_task = task,
//            };
//        }

//        public static implicit operator Input<T>(Output<T> output) {
//            return new Input<T> {
//                m_output = output,
//            };
//        }

//        public async Task<OutputState<object>> GetValueAsOutputStateAsync() {
//            if (m_task != null) {
//                return new OutputState<object>(await m_task, true, Array.Empty<Resource>());
//            } else if (m_output != null) {
//                return await ((IOutput)m_output).GetOutputStateAsync();
//            } else {
//                // If the underlying value is a resource, ensure we flow the resource as a dependency in the synthetic output state.
//                // TODO(ellismg): Doing this here feels wrong for some reason.
//                Resource r = m_rawValue as Resource;
//                return new OutputState<object>(m_rawValue, true, r != null ? new Resource[] { r } : Array.Empty<Resource>());
//            }
//        }
//    }
//}