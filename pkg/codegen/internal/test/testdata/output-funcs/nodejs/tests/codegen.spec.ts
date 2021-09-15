import 'mocha';
import * as assert from 'assert';

import * as pulumi from '@pulumi/pulumi';

import { listStorageAccountKeysOutput, ListStorageAccountKeysResult } from '../listStorageAccountKeys';
import { funcWithAllOptionalInputsOutput } from '../funcWithAllOptionalInputs';
import { funcWithDefaultValueOutput } from '../funcWithDefaultValue';
import { funcWithListParamOutput } from '../funcWithListParam';
import { funcWithDictParamOutput } from '../funcWithDictParam';

pulumi.runtime.setMocks({
    newResource: function(_: pulumi.runtime.MockResourceArgs): {id: string, state: any} {
        throw new Error('newResource not implemented');
    },
    call: function(args: pulumi.runtime.MockCallArgs) {
        if (args.token == 'mypkg::listStorageAccountKeys') {
            return {
                'keys': [{
                    'creationTime': 'my-creation-time',
                    'keyName': 'my-key-name',
                    'permissions': 'my-permissions',
                    'value': JSON.stringify(args.inputs),
                }]
            };
        }
        if (args.token == 'mypkg::funcWithAllOptionalInputs' ||
            args.token == 'mypkg::funcWithDefaultValue' ||
            args.token == 'mypkg::funcWithListParam' ||
            args.token == 'mypkg::funcWithDictParam')
        {
            return {
                'r': JSON.stringify(args.inputs)
            };
        }
        throw new Error('call not implemented for ' + args.token);
    },
});

describe('output-funcs', () => {
    it('funcWithAllOptionalInputsOutput', (done) => {
        const output = funcWithAllOptionalInputsOutput({a: pulumi.output('my-a')});
        checkOutput(done, output, res => {
            assert.equal(res.r, '{"a":"my-a"}');
        });
    });

    // TODO it seems that Node codegen does not respect default values at
    // the moment. TODO format this comment properly.
    it('funcWithDefaultValueOutput', (done) => {
        const output = funcWithDefaultValueOutput({a: pulumi.output('my-a')});
        checkOutput(done, output, res => {
            assert.equal(res.r, '{"a":"my-a"}');
        })
    });

    it('funcWithListParamOutput', (done) => {
        const output = funcWithListParamOutput({a: [
            pulumi.output('my-a1'),
            pulumi.output('my-a2'),
            pulumi.output('my-a3'),
        ]});
        checkOutput(done, output, res => {
            assert.equal(res.r, '{"a":["my-a1","my-a2","my-a3"]}');
        })
    });

    it('funcWithDictParamOutput', (done) => {
        const output = funcWithDictParamOutput({a: {
            'one': pulumi.output('1'),
            'two': pulumi.output('2')
        }});
        checkOutput(done, output, res => {
            assert.equal(res.r, '{"a":{"one":"1","two":"2"}}');
        })
    });

    it('listStorageAccountKeysOutput', (done) => {
        const output = listStorageAccountKeysOutput({
            accountName: pulumi.output('my-account-name'),
            resourceGroupName: pulumi.output('my-resource-group-name'),
        });
        checkOutput(done, output, (res: ListStorageAccountKeysResult) => {
            assert.equal(res.keys.length, 1);
            const k = res.keys[0];
            assert.equal(k.creationTime, 'my-creation-time');
            assert.equal(k.keyName, 'my-key-name');
            assert.equal(k.permissions, 'my-permissions');
            assert.deepStrictEqual(JSON.parse(k.value), {
                'accountName': 'my-account-name',
                'resourceGroupName': 'my-resource-group-name'
            });
        });
    });

    it('listStorageAccountKeysOutput with optional arg set', (done) => {
        const output = listStorageAccountKeysOutput({
            accountName: pulumi.output('my-account-name'),
            resourceGroupName: pulumi.output('my-resource-group-name'),
            expand: pulumi.output('my-expand'),
        });
        checkOutput(done, output, (res: ListStorageAccountKeysResult) => {
            assert.equal(res.keys.length, 1);
            const k = res.keys[0];
            assert.equal(k.creationTime, 'my-creation-time');
            assert.equal(k.keyName, 'my-key-name');
            assert.equal(k.permissions, 'my-permissions');
            assert.deepStrictEqual(JSON.parse(k.value), {
                'accountName': 'my-account-name',
                'resourceGroupName': 'my-resource-group-name',
                'expand': 'my-expand'
            });
        });
    });
 });


function checkOutput<T>(done: any, output: pulumi.Output<T>, check: (value: T) => void) {
    output.apply(value => {
        try {
            check(value);
            done();
        } catch (error) {
            done(error);
        }
    });
}
