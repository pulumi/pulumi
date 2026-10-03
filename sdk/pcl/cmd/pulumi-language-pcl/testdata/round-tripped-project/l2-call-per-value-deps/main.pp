// Exercises per-leaf OutputValue dependency propagation through a method Call.
//
// `getBoth` returns `{wrapper: {a, b}}` where `wrapper` is a plain field of a named object type and `a`,
// `b` are non-plain. The provider annotates `ReturnDependencies["wrapper"] = {pa, pb}` — the honest coarse
// answer, since ReturnDependencies cannot address nested leaves. Per-leaf precision only survives via
// OutputValues wrapping `a` and `b` individually.
//
// `g` consumes only `wrapper.a`:
//   - OutputValues path: per-leaf dep on `a` carries only pa; g depends on {pa, c}.
//   - Legacy path: the coarse {pa, pb} on `wrapper` propagates to any descent; g depends on {pa, pb, c}.

resource "c" "pick-call:index:Picker" {
    value = "c"
}

resource "pa" "simple-invoke:index:StringResource" {
    text = "pa"
}

resource "pb" "simple-invoke:index:StringResource" {
    text = "pb"
}

both = call(c, "getBoth", {
    a = pa.text
    b = pb.text
})

resource "g" "simple-invoke:index:StringResource" {
    text = both.wrapper.a
}
