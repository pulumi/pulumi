"""Defines test subsets of integration tests.

Separates out the monolithic package in "tests/integration" into five parts, one for each SDK and
the "rest". It does this by generating a valid value for the "-run" parameter to go test, which
expects a regular expression of test names.
"""
import os
import re

def get_integration_tests():
    current_dir = os.path.dirname(__file__)
    root_dir = os.path.realpath(os.path.join(current_dir, '..'))
    integration_dir = os.path.join(root_dir, 'tests', 'integration')

    languages = ["go", "nodejs", "python"]
    get_lang_filename = lambda lang: f"integration_{lang}_test.go"

    sdk_tests = [get_lang_filename(lang) for lang in languages]
    other_tests = [
        f for f
        in os.listdir(integration_dir)
        if os.path.isfile(os.path.join(integration_dir, f))
        and f.endswith("_test.go")
        and f not in sdk_tests
    ]

    integration_tests = {}

    for lang in languages:
        with open(os.path.join(integration_dir, get_lang_filename(lang)), encoding='utf_8') as f:
            contents = f.read()
            test_funcs = re.findall(r'func\s+(Test\w+)', contents)
            run_arg = "^(" + '|'.join(test_funcs) + ')$'
            integration_tests[lang] = run_arg

    all_other_tests = []
    for filename in other_tests:
        with open(os.path.join(integration_dir, filename), encoding='utf_8') as f:
            contents = f.read()
            test_funcs = re.findall(r'func\s+(Test\w+)', contents)
            all_other_tests += test_funcs

    run_arg = "^(" + '|'.join(all_other_tests) + ')$'
    integration_tests['rest'] = run_arg

    return integration_tests

INTEGRATION_TESTS = get_integration_tests()
