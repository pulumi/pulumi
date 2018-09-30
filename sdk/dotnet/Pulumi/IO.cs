using System;
using System.Collections.Generic;
using System.Linq;
using System.Threading;
using System.Threading.Tasks;

namespace Pulumi {

    public static class IO {
        public static IO<T[]> WhenAll<T>(IEnumerable<IO<T>> ios) {
            return IO<T>.WhenAll(ios);
        }
    }

    internal struct Output<T> {
        public T Value { get; private set; }
        public bool IsKnown { get; private set; }

        public Output(T value, bool isKnown) : this() {
            Value = value;
            IsKnown = isKnown;
        }
    }

    public sealed class IO<T> {

        internal struct Awaiter : System.Runtime.CompilerServices.INotifyCompletion {
            IO<T> m_io;
            public Awaiter(IO<T> io) {
                m_io = io;
            }

            public bool IsCompleted {
                get {
                    if (m_io.m_isKnown != null) {
                        return m_io.m_task.IsCompleted && m_io.m_isKnown.IsCompleted;
                    } else if (m_io.m_task != null) {
                        return m_io.m_task.IsCompleted;
                    } else {
                        return true;
                    }
                }
            }

            public Output<T> GetResult() {
                if (m_io.m_isKnown != null) {
                    var value = m_io.m_task.Result;
                    var isknown = m_io.m_isKnown.Result;
                    return new Output<T>(value, isknown);
                } else if (m_io.m_task != null) {
                    var value = m_io.m_task.Result;
                    return new Output<T>(value, true);
                } else {
                    return new Output<T>(m_io.m_rawValue, true);
                }
            }

            public async void OnCompleted(Action action) {
                if (m_io.m_isKnown != null) {
                    await m_io.m_isKnown;
                }
                if (m_io.m_task != null) {
                    await m_io.m_task;
                }
                action();
            }
        }

        private static HashSet<Resource> m_empty = new HashSet<Resource>();
        private HashSet<Resource> m_resources = m_empty;
        private Task<bool> m_isKnown;
        private Task<T> m_task;
        private T m_rawValue;

        private Task<T> Task { get {
            if (m_task == null) {
                Interlocked.CompareExchange(ref m_task, Task<T>.FromResult(m_rawValue), null);
            }
            return m_task;
        } }

        private Task<bool> IsKnown { get {
            if (m_isKnown == null) {
                Interlocked.CompareExchange(ref m_isKnown, Task<bool>.FromResult(true), null);
            }
            return m_isKnown;
        } }

        internal IEnumerable<Resource> Resources { get {
            return m_resources;
        } }

        internal IO(HashSet<Resource> resources, Task<T> task, Task<bool> isKnown) {
            m_resources = resources;
            m_task = task;
            m_isKnown = isKnown;
        }

        internal IO(Resource resource, Task<T> task, Task<bool> isKnown) {
            m_resources = new HashSet<Resource>();
            m_resources.Add(resource);
            m_task = task;
            m_isKnown = isKnown;
        }

        public IO(Task<T> task) {
            m_task = task;
        }

        public IO(T rawValue) {
            m_rawValue = rawValue;
        }

        // Internal output functions

        internal Awaiter GetAwaiter() {
            return new Awaiter(this);
        }

        // Public IO functions

        public static implicit operator IO<T>(T rawValue) {
            return new IO<T>(rawValue);
        }

        public static implicit operator IO<T>(Task<T> task) {
            return new IO<T>(task);
        }

        public IO<U> Select<U>(Func<T, U> selector) {
            if (m_isKnown != null) {
                var task = Task<U>.Run(async () => {
                    if(await m_isKnown) {
                        return selector(await Task);
                    } else {
                        return default(U);
                    }
                });
                return new IO<U>(m_resources, task, m_isKnown);
            } if (m_task != null) {
                var task = m_task.Select(result => selector(result));
                return new IO<U>(m_resources, task, null);
            } else {
                return selector(m_rawValue);
            }
        }

        public IO<T> Catch(Func<Exception, T> handler) {
            if (m_isKnown != null) {
                var task = Task<T>.Run(async () => {
                    if(await m_isKnown) {
                        try {
                            return await Task;
                        } catch (Exception e) {
                            return handler(e);
                        }
                    } else {
                        return default(T);
                    }
                });
                return new IO<T>(m_resources, task, m_isKnown);
            } if (m_task != null) {
                var task = m_task.Catch(handler);
                return new IO<T>(m_resources, task, null);
            } else {
                return m_rawValue;
            }
        }

        public IO<U> SelectMany<U>(Func<T, IO<U>> selector) {
            if (m_isKnown != null) {
                var isKnown = new TaskCompletionSource<bool>();
                var task = Task<U>.Run(async () => {
                    if(await m_isKnown) {
                        var io = selector(await Task);
                        if(io.m_isKnown != null) {
                            isKnown.SetResult(await io.m_isKnown);
                            return await io.m_task;
                        } else if(io.m_task != null) {
                            isKnown.SetResult(true);
                            return await io.Task;
                        } else {
                            return io.m_rawValue;
                        }
                    } else {
                        isKnown.SetResult(false);
                        return default(U);
                    }
                });
                return new IO<U>(m_resources, task, isKnown.Task);
            } if (m_task != null) {
                var isKnown = new TaskCompletionSource<bool>();
                var task = Task<U>.Run(async () => {
                    var io = selector(await m_task);
                    if(io.m_isKnown != null) {
                        isKnown.SetResult(await io.m_isKnown);
                        return await io.m_task;
                    } else if(io.m_task != null) {
                        isKnown.SetResult(true);
                        return await io.Task;
                    } else {
                        return io.m_rawValue;
                    }
                });
                return new IO<U>(m_resources, task, isKnown.Task);

            } else {
                return selector(m_rawValue);
            }
        }

        public IO<V> Zip<U, V>(IO<U> other, Func<T, U, V> resultSelector) {
            if (m_isKnown == null && other.m_isKnown == null) {
                // Simple task or raw value IO
                if (m_task == null && other.m_task == null) {
                    // Both raw values, easy zip
                    return new IO<V>(resultSelector(m_rawValue, other.m_rawValue));
                } else {
                    return Task.Zip(other.Task, resultSelector);
                }
            } else {
                // Any combination of output/task/raw!
                var isKnown = IsKnown.Zip(other.IsKnown, (a, b) => a && b);
                var task = Task<V>.Run(async () => {
                    if(await isKnown) {
                        return resultSelector(await Task, await other.Task);
                    } else {
                        return default(V);
                    }
                });

                var resources = new HashSet<Resource>();
                resources.UnionWith(m_resources);
                resources.UnionWith(other.m_resources);

                return new IO<V>(resources, task, isKnown);
            }
        }

        public static IO<T[]> WhenAll(IEnumerable<IO<T>> ios) {
            var task = Task<T>.WhenAll(ios.Select(io => io.Task));

            var isKnown = Task<bool>
                .WhenAll(ios.Select(io => io.IsKnown))
                .Select(ks => ks.All(b => b));

            var resources = new HashSet<Resource>();
            foreach(var io in ios) {
                resources.UnionWith(io.m_resources);
            }

            return new IO<T[]>(resources, task, isKnown);
        }
    }
}
