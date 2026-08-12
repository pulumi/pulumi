import * as pulumi from '@pulumi/pulumi';

const pulumiConfig = new pulumi.Config();
const example = pulumiConfig.require('example');

console.log(example);
