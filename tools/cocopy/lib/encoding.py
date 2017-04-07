# Copyright 2017 Pulumi, Inc. All rights reserved.

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

def to_serializable(obj, skip_nones=False, key_mangler=None):
    """
    This routine converts an acyclic object graph into a dictionary of serializable attributes.  This avoids needing to
    do custom serialization.  During this translation, name conversion can be performed, to ensure that, for instance,
    `underscore_cased` names are transformed into `camelCased` names, if appropriate.
    """
    return to_serializable_dict(obj.__dict__, skip_nones, key_mangler)

def to_serializable_dict(m, skip_nones=False, key_mangler=None):
    """This routine converts a simple dictionary into a JSON-serializable map."""
    d = dict()
    for attr in m:
        v = to_serializable_value(m[attr], skip_nones)
        if v is not None or not skip_nones:
            key = attr
            if key_mangler is not None:
                key = key_mangler(key)
            d[key] = v
    return d

def to_serializable_value(v, skip_nones=False, key_mangler=None):
    """This routine converts a singular value into its JSON-serializable equivalent."""
    if (isinstance(v, str) or isinstance(v, unicode) or
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
            a.append(to_serializable_value(e, skip_nones))
        return a
    elif isinstance(v, dict):
        # For a map, just recurse into the map routing above, which copies all key/values.
        return to_serializable_dict(v, skip_nones)
    else:
        # For all others, assume it is an object, and serialize its keys directly.
        return to_serializable(v, skip_nones)

