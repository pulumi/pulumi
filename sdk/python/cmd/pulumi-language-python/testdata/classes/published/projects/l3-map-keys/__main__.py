import pulumi
from myComponent import MyComponent

cmp = MyComponent("cmp", {
    'booleanMap': {
        "my key": False,
        "my.key": True,
        "my-key": False,
        "my_key": True,
        "MY_KEY": False,
        "myKey": True,
        "__type": True,
        "__internal": False,
        "__provider": True,
        "__version": False,
        "": True,
        "Some ${common} \"characters\" 'that' need escaping: \\ (backslash), \x09 (tab), \x1b (escape), \x07 (bell), \x00 (null), \U000e0021 (tag space)": False,
        "Format and glob specifiers: %percent ...ellipsis {open }close *asterisk ?question ,comma &&and ||or !not =>arrow ==equal :colon /slash": True,
    }})
pulumi.export("resourceBooleanMap", cmp.boolean_map)
