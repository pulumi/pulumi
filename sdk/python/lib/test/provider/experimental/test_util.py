from pulumi.provider.experimental.util import camel_case


def test_camel_case():
    assert camel_case("foo_bar") == "fooBar"
    assert camel_case("foo_bar_baz") == "fooBarBaz"
    assert camel_case("Foo_Bar_Baz") == "FooBarBaz"
    assert camel_case("URL") == "URL"
    assert camel_case("url") == "url"
    assert camel_case("Foo_URL") == "FooURL"
    assert camel_case("FooURL") == "FooURL"
    assert camel_case("foo_") == "foo"
    assert camel_case("foo_bar_") == "fooBar"
