import * as mu from 'mu';
import * as aws from 'aws';

// Attaches a gateway to a VPC.
// @name: aws/ec2/vpcGatewayAttachment
// @website: http://docs.aws.amazon.com/AWSCloudFormation/latest/UserGuide/aws-resource-ec2-vpc-gateway-attachment.html
export class VPCGatewayAttachment extends aws.cloudformation.Resource {
    constructor(ctx: mu.Context, args: VPCGatewayAttachmentArgs) {
        super(ctx, {
            resource: "AWS::EC2::VPCGatewayAttachment",
            properties: {
                vpcId: args.vpc,
                internetGatewayId: args.internetGateway,
            },
        });
    }
}

export interface VPCGatewayAttachmentArgs {
    // The VPC to associate with this gateway.
    readonly vpc: aws.ec2.VPC;
    // The Internet gateway to attach to the VPC.
    readonly internetGateway: aws.ec2.InternetGateway;
}

