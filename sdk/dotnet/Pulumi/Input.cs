
using System;
using System.Threading.Tasks;

namespace Pulumi {
    interface IInput {
        Task<object> GetTask();
    }

    public struct Input<T> : IInput{
        T m_rawValue;
        Task<T> m_task;

        public static implicit operator Input<T>(T rawValue) {
            return new Input<T> {
                m_rawValue = rawValue,
            };
        }

        public static implicit operator Input<T>(Task<T> task) {
            return new Input<T> {
                m_task = task,
            };
        }

        // TODO(ellismg): Maybe use a custom awaiter here insted...?
        public Task<object> GetTask() {
            if (m_task != null) {
                return m_task.ContinueWith(x => (object) x.Result);
            } else {
                return Task.FromResult((object) m_rawValue);
            }
        }
    }
}
