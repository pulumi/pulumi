import 'mocha';
import * as assert from 'assert';

describe('anton 1', () => {

    it('should work', (done) => {
        assert.equal(2, 1 + 1);
        done()
    });
});

/*


import * as assert from 'assert';
import * as pulumi from '@pulumi/pulumi';
import { listStorageAccountKeysOutput } from './listStorageAccountKeys';
import { funcWithAllOptionalInputsOutput } from './funcWithAllOptionalInputs';
import { funcWithDefaultValueOutput } from './funcWithDefaultValue';
import { funcWithListParamOutput } from './funcWithListParam';
import { funcWithDictParamOutput } from './funcWithDictParam';

pulumi.runtime.setMocks({
    newResource: function(_: pulumi.runtime.MockResourceArgs): {id: string, state: any} {
        throw new Error('newResource not implemented');
    },
    call: function(args: pulumi.runtime.MockCallArgs) {
        if (args.token == 'madeup-package:codegentest:listStorageAccountKeys') {
            return {
                'keys': [args.inputs]
            };
        }
        if (args.token == 'madeup-package:codegentest:funcWithAllOptionalInputs' ||
            args.token == 'madeup-package:codegentest:funcWithDefaultValue' ||
            args.token == 'madeup-package:codegentest:funcWithListParam' ||
            args.token == 'madeup-package:codegentest:funcWithDictParam')
        {
            return {
                'r': JSON.stringify(args.inputs)
            };
        }
        throw new Error('call not implemented for ' + args.token);
    },
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

describe('listStorageAccountKeysOutput', () => {
    it('should work', (done) => {
        const output = listStorageAccountKeysOutput({
            accountName: pulumi.output('my-account-name'),
            resourceGroupName: pulumi.output('my-resource-group-name'),
        });
        checkOutput(done, output, res => {
            assert.equal(res.keys.length, 1);
            assert.equal(Object.keys(res.keys[0]).length, 2);
            assert.equal(res.keys[0].accountName, 'my-account-name');
            assert.equal(res.keys[0].resourceGroupName, 'my-resource-group-name');
        });
    });

    it('should work when the optional arg is set', (done) => {
        const output = listStorageAccountKeysOutput({
            accountName: pulumi.output('my-account-name'),
            resourceGroupName: pulumi.output('my-resource-group-name'),
            expand: pulumi.output('my-expand')
        });
        checkOutput(done, output, res => {
            assert.equal(res.keys.length, 1);
            assert.equal(Object.keys(res.keys[0]).length, 3);
            assert.equal(res.keys[0].accountName, 'my-account-name');
            assert.equal(res.keys[0].resourceGroupName, 'my-resource-group-name');
            assert.equal(res.keys[0].expand, 'my-expand');
        });
    });
});

describe('funcWithAllOptionalInputsOutput', () => {
    it('should work', (done) => {
        const output = funcWithAllOptionalInputsOutput({a: pulumi.output('my-a')});
        checkOutput(done, output, res => {
            assert.equal(res.r, '{"a":"my-a"}');
        });
    });
});

// TODO it seems that Node codegen does not respect default values at
// the moment.
describe('funcWithDefaultValueOutput', () => {
    it('should work', (done) => {
        const output = funcWithDefaultValueOutput({a: pulumi.output('my-a')});
        checkOutput(done, output, res => {
            assert.equal(res.r, '{"a":"my-a"}');
        })
    });
});

describe('funcWithListParamOutput', () => {
    it('should work', (done) => {
        const output = funcWithListParamOutput({a: [
            pulumi.output('my-a1'),
            pulumi.output('my-a2'),
            pulumi.output('my-a3'),
        ]});
        checkOutput(done, output, res => {
            assert.equal(res.r, '{"a":["my-a1","my-a2","my-a3"]}');
        })
    });
});

describe('funcWithDictParamOutput', () => {
    it('should work', (done) => {
        const output = funcWithDictParamOutput({a: {
            'one': pulumi.output('1'),
            'two': pulumi.output('2')
        }});
        checkOutput(done, output, res => {
            assert.equal(res.r, '{"a":{"one":"1","two":"2"}}');
        })
    });
});

*/
