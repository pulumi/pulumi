// Copyright 2016-2019, Pulumi Corporation

namespace Pulumi
{
    /// <summary>
    /// Useful static utility methods for both creating and working wit <see cref="Output{T}"/>s.
    /// </summary>
    public static partial class Output
    {
        private static readonly Output<int> _zeroOut = Create(0);
        private static readonly Input<int> _zeroIn = Create(0);

        /// <summary>
        /// Combines all the <see cref="Input{T}"/> values in the provided parameters and combines
        /// them all into a single tuple containing each of their underlying values.  If any of the
        /// <see cref="Input{T}"/>s are not known, the final result will be not known.  Similarly,
        /// if any of the <see cref="Input{T}"/>s are secrets, then the final result will be a
        /// secret.
        /// </summary>
        public static Output<(T1, T2, T3, T4, T5, T6, T7, T8)> Tuple<T1, T2, T3, T4, T5, T6, T7, T8>(
            Input<T1> item1, Input<T2> item2, Input<T3> item3, Input<T4> item4,
            Input<T5> item5, Input<T6> item6, Input<T7> item7, Input<T8> item8)
            => Output<(T1, T2, T3, T4, T5, T6, T7, T8)>.Tuple(
                item1, item2, item3, item4, item5, item6, item7, item8);

        #region Overloads that take different numbers of inputs.

        /// <summary>
        /// <see cref="Tuple{T1, T2, T3, T4, T5, T6, T7, T8}(Input{T1}, Input{T2}, Input{T3}, Input{T4}, Input{T5}, Input{T6}, Input{T7}, Input{T8})"/>
        /// </summary>
        public static Output<(T1, T2)> Tuple<T1, T2>(
            Input<T1> item1, Input<T2> item2)
            => Tuple(item1, item2, _zeroIn, _zeroIn, _zeroIn, _zeroIn, _zeroIn, _zeroIn)
                .Apply(v => (v.Item1, v.Item2));

        /// <summary>
        /// <see cref="Tuple{T1, T2, T3, T4, T5, T6, T7, T8}(Input{T1}, Input{T2}, Input{T3}, Input{T4}, Input{T5}, Input{T6}, Input{T7}, Input{T8})"/>
        /// </summary>
        public static Output<(T1, T2, T3)> Tuple<T1, T2, T3>(
            Input<T1> item1, Input<T2> item2, Input<T3> item3)
            => Tuple(item1, item2, item3, _zeroIn, _zeroIn, _zeroIn, _zeroIn, _zeroIn)
                .Apply(v => (v.Item1, v.Item2, v.Item3));

        /// <summary>
        /// <see cref="Tuple{T1, T2, T3, T4, T5, T6, T7, T8}(Input{T1}, Input{T2}, Input{T3}, Input{T4}, Input{T5}, Input{T6}, Input{T7}, Input{T8})"/>
        /// </summary>
        public static Output<(T1, T2, T3, T4)> Tuple<T1, T2, T3, T4>(
            Input<T1> item1, Input<T2> item2, Input<T3> item3, Input<T4> item4)
            => Tuple(item1, item2, item3, item4, _zeroIn, _zeroIn, _zeroIn, _zeroIn)
                .Apply(v => (v.Item1, v.Item2, v.Item3, v.Item4));

        /// <summary>
        /// <see cref="Tuple{T1, T2, T3, T4, T5, T6, T7, T8}(Input{T1}, Input{T2}, Input{T3}, Input{T4}, Input{T5}, Input{T6}, Input{T7}, Input{T8})"/>
        /// </summary>
        public static Output<(T1, T2, T3, T4, T5)> Tuple<T1, T2, T3, T4, T5>(
            Input<T1> item1, Input<T2> item2, Input<T3> item3, Input<T4> item4,
            Input<T5> item5)
            => Tuple(item1, item2, item3, item4, item5, _zeroIn, _zeroIn, _zeroIn)
                .Apply(v => (v.Item1, v.Item2, v.Item3, v.Item4, v.Item5));

        /// <summary>
        /// <see cref="Tuple{T1, T2, T3, T4, T5, T6, T7, T8}(Input{T1}, Input{T2}, Input{T3}, Input{T4}, Input{T5}, Input{T6}, Input{T7}, Input{T8})"/>
        /// </summary>
        public static Output<(T1, T2, T3, T4, T5, T6)> Tuple<T1, T2, T3, T4, T5, T6>(
            Input<T1> item1, Input<T2> item2, Input<T3> item3, Input<T4> item4,
            Input<T5> item5, Input<T6> item6)
            => Tuple(item1, item2, item3, item4, item5, item6, _zeroIn, _zeroIn)
                .Apply(v => (v.Item1, v.Item2, v.Item3, v.Item4, v.Item5, v.Item6));

        /// <summary>
        /// <see cref="Tuple{T1, T2, T3, T4, T5, T6, T7, T8}(Input{T1}, Input{T2}, Input{T3}, Input{T4}, Input{T5}, Input{T6}, Input{T7}, Input{T8})"/>
        /// </summary>
        public static Output<(T1, T2, T3, T4, T5, T6, T7)> Tuple<T1, T2, T3, T4, T5, T6, T7>(
            Input<T1> item1, Input<T2> item2, Input<T3> item3, Input<T4> item4,
            Input<T5> item5, Input<T6> item6, Input<T7> item7)
            => Tuple(item1, item2, item3, item4, item5, item6, item7, _zeroIn)
                .Apply(v => (v.Item1, v.Item2, v.Item3, v.Item4, v.Item5, v.Item6, v.Item7));

        #endregion

        #region Overloads that take different numbers of outputs.

        /// <summary>
        /// <see cref="Tuple{T1, T2, T3, T4, T5, T6, T7, T8}(Input{T1}, Input{T2}, Input{T3}, Input{T4}, Input{T5}, Input{T6}, Input{T7}, Input{T8})"/>
        /// </summary>
        public static Output<(T1, T2)> Tuple<T1, T2>(
            Output<T1> item1, Output<T2> item2)
            => Tuple(item1, item2, _zeroOut, _zeroOut, _zeroOut, _zeroOut, _zeroOut, _zeroOut)
                .Apply(v => (v.Item1, v.Item2));

        /// <summary>
        /// <see cref="Tuple{T1, T2, T3, T4, T5, T6, T7, T8}(Input{T1}, Input{T2}, Input{T3}, Input{T4}, Input{T5}, Input{T6}, Input{T7}, Input{T8})"/>
        /// </summary>
        public static Output<(T1, T2, T3)> Tuple<T1, T2, T3>(
            Output<T1> item1, Output<T2> item2, Output<T3> item3)
            => Tuple(item1, item2, item3, _zeroOut, _zeroOut, _zeroOut, _zeroOut, _zeroOut)
                .Apply(v => (v.Item1, v.Item2, v.Item3));

        /// <summary>
        /// <see cref="Tuple{T1, T2, T3, T4, T5, T6, T7, T8}(Input{T1}, Input{T2}, Input{T3}, Input{T4}, Input{T5}, Input{T6}, Input{T7}, Input{T8})"/>
        /// </summary>
        public static Output<(T1, T2, T3, T4)> Tuple<T1, T2, T3, T4>(
            Output<T1> item1, Output<T2> item2, Output<T3> item3, Output<T4> item4)
            => Tuple(item1, item2, item3, item4, _zeroOut, _zeroOut, _zeroOut, _zeroOut)
                .Apply(v => (v.Item1, v.Item2, v.Item3, v.Item4));

        /// <summary>
        /// <see cref="Tuple{T1, T2, T3, T4, T5, T6, T7, T8}(Input{T1}, Input{T2}, Input{T3}, Input{T4}, Input{T5}, Input{T6}, Input{T7}, Input{T8})"/>
        /// </summary>
        public static Output<(T1, T2, T3, T4, T5)> Tuple<T1, T2, T3, T4, T5>(
            Output<T1> item1, Output<T2> item2, Output<T3> item3, Output<T4> item4,
            Output<T5> item5)
            => Tuple(item1, item2, item3, item4, item5, _zeroOut, _zeroOut, _zeroOut)
                .Apply(v => (v.Item1, v.Item2, v.Item3, v.Item4, v.Item5));

        /// <summary>
        /// <see cref="Tuple{T1, T2, T3, T4, T5, T6, T7, T8}(Input{T1}, Input{T2}, Input{T3}, Input{T4}, Input{T5}, Input{T6}, Input{T7}, Input{T8})"/>
        /// </summary>
        public static Output<(T1, T2, T3, T4, T5, T6)> Tuple<T1, T2, T3, T4, T5, T6>(
            Output<T1> item1, Output<T2> item2, Output<T3> item3, Output<T4> item4,
            Output<T5> item5, Output<T6> item6)
            => Tuple(item1, item2, item3, item4, item5, item6, _zeroOut, _zeroOut)
                .Apply(v => (v.Item1, v.Item2, v.Item3, v.Item4, v.Item5, v.Item6));

        /// <summary>
        /// <see cref="Tuple{T1, T2, T3, T4, T5, T6, T7, T8}(Input{T1}, Input{T2}, Input{T3}, Input{T4}, Input{T5}, Input{T6}, Input{T7}, Input{T8})"/>
        /// </summary>
        public static Output<(T1, T2, T3, T4, T5, T6, T7)> Tuple<T1, T2, T3, T4, T5, T6, T7>(
            Output<T1> item1, Output<T2> item2, Output<T3> item3, Output<T4> item4,
            Output<T5> item5, Output<T6> item6, Output<T7> item7)
            => Tuple(item1, item2, item3, item4, item5, item6, item7, _zeroOut)
                .Apply(v => (v.Item1, v.Item2, v.Item3, v.Item4, v.Item5, v.Item6, v.Item7));

        /// <summary>
        /// <see cref="Tuple{T1, T2, T3, T4, T5, T6, T7, T8}(Input{T1}, Input{T2}, Input{T3}, Input{T4}, Input{T5}, Input{T6}, Input{T7}, Input{T8})"/>
        /// </summary>
        public static Output<(T1, T2, T3, T4, T5, T6, T7, T8)> Tuple<T1, T2, T3, T4, T5, T6, T7, T8>(
            Output<T1> item1, Output<T2> item2, Output<T3> item3, Output<T4> item4,
            Output<T5> item5, Output<T6> item6, Output<T7> item7, Output<T8> item8)
            => Output<(T1, T2, T3, T4, T5, T6, T7, T8)>.Tuple(
                (Input<T1>)item1, (Input<T2>)item2, (Input<T3>)item3, (Input<T4>)item4,
                (Input<T5>)item5, (Input<T6>)item6, (Input<T7>)item7, (Input<T8>)item8);

        #endregion
    }
}
