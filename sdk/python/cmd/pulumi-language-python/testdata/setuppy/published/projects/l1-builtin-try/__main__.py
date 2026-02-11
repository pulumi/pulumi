import pulumi

def try_(*fns):
    for fn in fns:
        try:
            result = fn()
            return result
        except:
            continue
    return None

async def try_output(*fns):
    if len(fns) == 0:
        raise ValueError("expected at least one argument to try_output")
    head, tail = fns[0], fns[1:]
    try:
        r = head()
        if isinstance(r, pulumi.Output):
            # Each output is tracked in SETTINGS.outputs, and on shutdown
            # we report any failed outputs.
            # Here however we handle the failure explicitly.
            # TODO: How do we nicely untrack the output?
            # Why is this a problem for Python but not Node.js?
            # because a.x is a.apply(lift)? so it is itself an output that we have to await?
            # TODO: figure out why the test passes on Node.js without
            # having to await the output, isn't "map.a" an output
            from pulumi.runtime.settings import SETTINGS
            SETTINGS.outputs.remove(r._future)
            is_secret = await r.is_secret()
            r = await r.future()
            if is_secret: r = pulumi.output.Output.secret(r)
        return r
    except Exception as e:
         return try_output(*tail)


config = pulumi.Config()
a_map = config.require_object("aMap")
pulumi.export("plainTrySuccess", try_(
    lambda: a_map["a"],
    lambda: "fallback"
))
pulumi.export("plainTryFailure", try_(
    lambda: a_map["b"],
    lambda: "fallback"
))
a_secret_map = pulumi.Output.secret(a_map)
pulumi.export("outputTrySuccess", try_output(
    lambda: a_secret_map["a"],
    lambda: "fallback"
))
pulumi.export("outputTryFailure", try_output(
    lambda: a_secret_map["b"],
    lambda: "fallback"
))
an_object = config.require_object("anObject")
pulumi.export("dynamicTrySuccess", try_output(
    lambda: an_object["a"],
    lambda: "fallback"
))
pulumi.export("dynamicTryFailure", try_output(
    lambda: an_object["b"],
    lambda: "fallback"
))
a_secret_object = pulumi.Output.secret(an_object)
pulumi.export("outputDynamicTrySuccess", try_output(
    lambda: a_secret_object["a"],
    lambda: "fallback"
))
pulumi.export("outputDynamicTryFailure", try_output(
    lambda: a_secret_object["b"],
    lambda: "fallback"
))
pulumi.export("plainTryNull", [try_output(
    lambda: an_object["opt"],
    lambda: "fallback"
)])
pulumi.export("outputTryNull", [try_output(
    lambda: a_secret_object["opt"],
    lambda: "fallback"
)])
