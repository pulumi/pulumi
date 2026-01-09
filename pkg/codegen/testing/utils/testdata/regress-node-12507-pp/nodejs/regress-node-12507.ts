import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const config = new pulumi.Config();
const localGatewayVirtualInterfaceGroupId = config.require("localGatewayVirtualInterfaceGroupId");
const rts = aws.ec2.getLocalGatewayRouteTablesOutput({
    filters: [{
        name: "tag:kubernetes.io/kops/role",
        values: ["private*"],
    }],
});
const routes: aws.ec2.LocalGatewayRoute[] = [];
rts.ids.length.apply(rangeBody => {
    for (const range = {value: 0}; range.value < rangeBody; range.value++) {
        routes.push(new aws.ec2.LocalGatewayRoute(`routes-${range.value}`, {
            destinationCidrBlock: "10.0.1.0/22",
            localGatewayRouteTableId: rts.apply(rts => rts.ids[range.value]),
            localGatewayVirtualInterfaceGroupId: localGatewayVirtualInterfaceGroupId,
        }));
    }
});
