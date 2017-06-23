// Copyright 2016-2017, Pulumi Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import * as cmd from "@lumi/cmd";
import * as docker from "@lumi/docker";

export let agentCount: number | undefined;
export let dns: string | undefined;

export function new() {
    // First create our cluster infrastructure services; for now, just a key/value store.
    let infraNodeName = prefix + "-infra";
    let infraNode = new docker.machine.Machine(infraNodeName, {});
    let startInfraConsul = new cmd.SshCommand("start-infra-consul", {
        command: "sudo docker run -d -p \"8500:8500\" -h \"consul\" progrium/consul -server -bootstrap",
    });

    // Next create an actual Swarm master and agent nodes, initializing the Swarm cluster along the way.
    let swarmDiscovery = "consul://" + infraNode.address + ":8500";
    let masterNode = new docker.machine.Machine(prefix + "-master", {});
    for (let i = 0; i < agentCount; i++) {
        let agentNodeName = prefix + "-agent-" + i;
        new docker.machine.Machine(prefix + "-agent-" + i, {});
    }
}

function createMachine(
        machine: string, createEbsMounts: boolean, swarmDiscovery: string, swarmMaster: boolean): void {
    // Prepare the arguments to guide the Docker Machine creation.
    let machineArgs: docker.machine.MachineArgs = {
        engineOpt: [],
    };

    // If custom DNS servers were requested, ensure Docker Engine uses them.
    if (dns) {
        machineOptions.engineOpt.push("dns=" + dns);
    }

    // If joining a Swarm, we need to use the Swarm discovery as our store.
    if (swarmDiscovery) {
        machineOptions.engineOpt.push("cluster-store=" + swarmDiscovery);
    }

    // TODO: copy a bunch of the existing stuff!
}

