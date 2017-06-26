# Copyright 2016-2017, Pulumi Corporation.  All rights reserved.

def underscore_to_camel_case(key):
    # Quickly check to see if there is no _; in that case, just return the key as-is.
    has = False
    for c in key:
        if c == "_":
            has = True
            break
    if not has:
        return key

    # If there's an underscore, accumulate the contents into a buffer, swapping _s with camelCased strings.
    new = ""
    next_case = False
    for c in key:
        if c == "_" and new != "":
            next_case = True # skip and capitalize the next character.
        else:
            if next_case:
                c = c.upper()
                next_case = False
            new += c
    return new

class Options:
    """Options to control serialization."""
    def __init__(self, skip_nones=False, key_encoder=None):
        self.skip_nones = skip_nones
        self.key_encoder = key_encoder

def to_serializable(obj, opts=None):
    """
    This routine converts an acyclic object graph into a dictionary of serializable attributes.  This avoids needing to
    do custom serialization.  During this translation, name conversion can be performed on object attributes, to ensure
    that, for instance, `underscore_cased` names are transformed into `camelCased` names, if appropriate.
    """
    return to_serializable_dictobj(obj.__dict__, True, opts)

def to_serializable_dict(d, opts=None):
    """This routine converts a simple dictionary into a JSON-serializable map."""
    return to_serializable_dictobj(d, False, opts)

def to_serializable_dictobj(d, isobj, opts=None):
    """This routine is used by both object and dictionary serialization routines to produce a map."""
    result = dict()
    for attr in d:
        v = to_serializable_value(d[attr], opts)
        if v is not None or opts is None or not opts.skip_nones:
            key = attr
            if isobj and opts and opts.key_encoder:
                key = opts.key_encoder(key)
            result[key] = v
    return result

def to_serializable_value(v, opts=None):
    """This routine converts a singular value into its JSON-serializable equivalent."""
    if (isinstance(v, basestring) or
            isinstance(v, int) or isinstance(v, long) or isinstance(v, float) or
            isinstance(v, bool) or v is None):
        # Simple serializable values can be stored without any translation.
        return v
    elif isinstance(v, list) or isinstance(v, set):
        # For lists (or sets), just convert to a list of the values.
        if isinstance(v, set):
            v = list(v) # convert the set to a list.
        a = list()
        for e in v:
            a.append(to_serializable_value(e, opts))
        return a
    elif isinstance(v, dict):
        # For a map, just recurse into the map routing above, which copies all key/values.
        return to_serializable_dict(v, opts)
    else:
        # For all others, assume it is an object, and serialize its keys directly.
        return to_serializable(v, opts)

