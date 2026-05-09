from pulumi_release import versions


def test_parse():
    assert versions.parse("v3.235.0") == (3, 235, 0, None)
    assert versions.parse("3.235.0-rc.1") == (3, 235, 0, "rc.1")


def test_normalize():
    assert versions.normalize("v3.235.0") == "3.235.0"
    assert versions.normalize("3.235.0") == "3.235.0"


def test_with_v():
    assert versions.with_v("3.235.0") == "v3.235.0"
    assert versions.with_v("v3.235.0") == "v3.235.0"


def test_bump_minor():
    assert versions.bump_minor("3.235.0") == "3.236.0"
    assert versions.bump_minor("v3.236.0") == "3.237.0"
    assert versions.bump_minor("3.99.7") == "3.100.0"   # patch reset


def test_bump_patch():
    assert versions.bump_patch("3.216.1") == "3.216.2"
    assert versions.bump_patch("v3.235.0") == "3.235.1"


def test_compare_basic():
    assert versions.compare("3.235.0", "3.236.0") == -1
    assert versions.compare("3.236.0", "3.235.0") == 1
    assert versions.compare("3.235.0", "v3.235.0") == 0


def test_compare_suffix():
    # any non-suffixed version is "after" the same X.Y.Z with a suffix
    assert versions.compare("3.235.0-rc.1", "3.235.0") == -1
    assert versions.compare("3.235.0", "3.235.0-rc.1") == 1
    assert versions.compare("3.235.0-rc.1", "3.235.0-rc.2") == -1
