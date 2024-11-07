from pulumi.dynamic import Config


def test_config_get():
    c = Config({"my-project:a": "A", "namespace:b": "B"}, "my-project")

    assert c.get("my-project:a") == "A"
    assert c.get("a") == "A"
    assert c.get("namespace:b") == "B"
    assert c.get("b") == None


def test_config_require():
    c = Config({"my-project:a": "A", "namespace:b": "B"}, "my-project")

    assert c.require("my-project:a") == "A"
    assert c.require("a") == "A"
    assert c.require("namespace:b") == "B"
    try:
        c.require("b")
        assert False
    except ValueError as e:
        assert str(e) == "missing required configuration key: b"
