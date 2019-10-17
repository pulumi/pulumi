
//using System;
//using System.Threading.Tasks;

//namespace Pulumi {

//    public class OutputState<T> {
//        public T Value { get; private set; }
//        public bool IsKnown { get; private set; }

//        public Resource[] DependsOn { get; private set; }

//        public OutputState(T value, bool isKnown, params Resource[] dependsOn) {
//            Serilog.Log.Debug("Creating new OutputState {value} {isKnown} {dependsOn}", value, isKnown, dependsOn);
//            Value = value;
//            IsKnown = isKnown;
//            DependsOn = dependsOn;
//        }
//    }
//    interface IOutput {
//        Task<OutputState<object>> GetOutputStateAsync();
//    }

//    public sealed class Output<T> : IOutput{
//        Task<OutputState<T>> m_stateTask;

//        public Output(Task<OutputState<T>> stateTask) {
//            m_stateTask = stateTask;
//        }

//        async Task<OutputState<object>> IOutput.GetOutputStateAsync() {
//            var resolvedState = await m_stateTask;
//            return new OutputState<object>(resolvedState.Value, resolvedState.IsKnown, resolvedState.DependsOn);
//        }

//        public void Apply(Action<T> fn) {
//            if (this.m_stateTask.Result.IsKnown) {
//                fn(this.m_stateTask.Result.Value);
//            }
//        }

//        public Output<U> Apply<U>(Func<T, U> fn) {
//            return new Output<U>(this.m_stateTask.ContinueWith(x => new OutputState<U>(
//                x.Result.IsKnown ? fn(x.Result.Value) : default(U),
//                x.Result.IsKnown,
//                x.Result.DependsOn)));
//        }
//    }
//}
