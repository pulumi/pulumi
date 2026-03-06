import * as pulumi from "@pulumi/pulumi";
import * as sync from "@pulumi/sync";

const block_1 = new sync.Block("block-1", {});
const block_2 = new sync.Block("block-2", {});
const block_3 = new sync.Block("block-3", {});
