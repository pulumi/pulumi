import json
import subprocess as sp
import jsonschema
import jsonschema.exceptions


def load_json_file(path):
    with open(path) as fp:
        return json.load(fp)


def parse_ref(j):
    if type(j) == dict and '$ref' in j:
        return j['$ref']

    return None


def resolve_ref(root, ref):
    parts = ref.split('/')
    assert parts[0] == '#'
    for p in parts[1:]:
        root = root[p]
    return root


ignored_keys = [
    '$defs',
    '$id',
    '$schema',
    'const',
    'description',
    'enum', # TODO enum coverage may be interesting
    'format',
    'pattern',
    'propertyNames',
    'required',
    'minItems',
    'title',
]


understood_keys = [
    'type',
    'items',
    'properties',
    'additionalProperties',
    'allOf',
    'oneOf',
]


def is_valid(root_schema, schema, json):
    s = {}

    for k in schema:
        s[k] = schema[k]

    s['$defs'] = root_schema['$defs']

    try:
        jsonschema.validate(schema=s, instance=json)
        return True
    except jsonschema.exceptions.ValidationError as e:
        return False


PATHS = set([])
"""Collects covered schema paths."""


POSSIBLE_PATHS = set([])
"""Collects all possible  schema paths."""


def find_all_possible_paths(root_schema, schema, path=None):
    if path is None:
        path = '#'

    if path in POSSIBLE_PATHS:
        return  # seen already

    POSSIBLE_PATHS.add(path)

    ref = parse_ref(schema)
    if ref is not None:
        # print(f'Forwarding {path} to {ref}')
        find_all_possible_paths(root_schema, resolve_ref(root_schema, ref), path=ref)
        return

    if 'const' in schema:
        return

    if 'type' not in schema:
        raise Exception(f"Cannot parse type spec: 'type' field missing: {schema}")

    def prim(t):
        return t in ['string', 'boolean', 'number', 'integer']

    if prim(schema['type']):
        return

    if type(schema['type']) == list and all(prim(t) for t in schema['type']):
        return

    if schema['type'] == 'array':
        find_all_possible_paths(root_schema, schema['items'], f'{path}/items')
        return

    if schema['type'] == 'object':

        properties = schema.get('properties', {})
        for prop_name, prop_schema in properties.items():
            find_all_possible_paths(root_schema, prop_schema, f'{path}/{prop_name}')

        aProperties = schema.get('additionalProperties', False)
        if type(aProperties) != bool:
            find_all_possible_paths(root_schema,
                                    aProperties,
                                    f'{path}/additionalProperties')

        all_of = schema.get('allOf', [])
        if all_of:
            for i, ty in enumerate(all_of):
                find_all_possible_paths(root_schema, ty, path=f'{path}/allOf[{i}]')

        one_of = schema.get('oneOf', [])
        if one_of:
            for i, ty in enumerate(one_of):
                find_all_possible_paths(root_schema, ty, path=f'{path}/oneOf[{i}]')

        return

    raise Exception(f'Cannot understand schema: {schema}')


def validate(root_schema, schema, json, path=None):
    if path is None:
        path = '#'

    PATHS.add(path)

    # print(f'validating path={path}')

    ref = parse_ref(schema)
    if ref is not None:
        # print(f'Forwarding {path} to {ref}')
        validate(root_schema, resolve_ref(root_schema, ref), json, path=ref)
        return

    for key in schema:
        if key not in ignored_keys + understood_keys:
            print(f'Confusing KEY={key} AT path={path}')

    if 'const' in schema:
        return

    if 'type' not in schema:
        raise Exception(f"Cannot parse type spec: 'type' field missing: {schema}")

    def prim(t):
        return t in  ['string', 'boolean', 'number', 'integer']

    if prim(schema['type']):
        return

    if type(schema['type']) == list and all(prim(t) for t in schema['type']):
        return

    if schema['type'] == 'array':
        t = schema['items']

        if type(json) == list:
            for v in json:
                validate(root_schema, t, v, path=f'{path}/items')

        return

    if schema['type'] == 'object':

        properties = schema.get('properties', {})
        for prop_name, prop_schema in properties.items():
            if prop_name in json:
                validate(root_schema, prop_schema, json[prop_name], f'{path}/{prop_name}')

        aProperties = schema.get('additionalProperties', False)
        if aProperties != False:
            if type(json) == dict:
                for value_key, value in json.items():
                    if value_key not in properties:
                        validate(root_schema,
                                 schema['additionalProperties'],
                                 value,
                                 path=f'{path}/additionalProperties')

        allOf = schema.get('allOf', [])
        for i, ty in enumerate(allOf):
            validate(root_schema, ty, json, path=f'{path}/allOf[{i}]')

        one_of = schema.get('oneOf', [])
        if one_of:
            matches = [
                (f'{path}/oneOf[{i}]', ty)
                for (i, ty) in enumerate(one_of)
                if is_valid(root_schema, ty, json)
            ]

            if len(matches) != 1:
                print(f'WARN: when validating against {path}/oneOf expected exactly 1 match, got {len(matches)}')
                # print(json)
            else:
                (path, ty) = matches[0]
                validate(root_schema, ty, json, path=path)

        return

    raise Exception(f'Cannot understand schema: {schema}')



meta_schema = load_json_file('../schema/pulumi.json')
find_all_possible_paths(meta_schema, meta_schema)


schema_corpus = [{'file': line, 'schema': load_json_file(line)}
                 for line in sp.check_output(
                         'ls ../internal/test/testdata/*/schema.json',
                         shell=True).decode('utf-8').split('\n')
                 if line]


for exemplar in schema_corpus:
    print('Parsing', exemplar['file'])
    valid = is_valid(meta_schema, meta_schema,  exemplar['schema'])
    print('Valid overall: ', valid)
    validate(meta_schema, meta_schema, exemplar['schema'])


print('=' * 80)
print('Covered meta-schema paths')
print('=' * 80)
for p in sorted(p for p in PATHS):
    print(p)

print()
print()

print('=' * 80)
print('Uncovered meta-schema paths')
print('=' * 80)
for p in sorted(p for p in POSSIBLE_PATHS - PATHS):
    print(p)

print()
print()

print('=' * 80)
print('Statistics')
print('=' * 80)

print('possible        ', len(POSSIBLE_PATHS))
print('covered         ', len(PATHS))
print('covered/possible', len(PATHS)/len(POSSIBLE_PATHS))
