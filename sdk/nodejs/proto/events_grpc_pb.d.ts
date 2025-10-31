// package: pulumirpc
// file: pulumi/events.proto

/* tslint:disable */
/* eslint-disable */

import * as grpc from "@grpc/grpc-js";
import * as pulumi_events_pb from "./events_pb";
import * as google_protobuf_empty_pb from "google-protobuf/google/protobuf/empty_pb";

interface IEventsService extends grpc.ServiceDefinition<grpc.UntypedServiceImplementation> {
    streamEvents: IEventsService_IStreamEvents;
}

interface IEventsService_IStreamEvents extends grpc.MethodDefinition<pulumi_events_pb.EventRequest, google_protobuf_empty_pb.Empty> {
    path: "/pulumirpc.Events/StreamEvents";
    requestStream: true;
    responseStream: false;
    requestSerialize: grpc.serialize<pulumi_events_pb.EventRequest>;
    requestDeserialize: grpc.deserialize<pulumi_events_pb.EventRequest>;
    responseSerialize: grpc.serialize<google_protobuf_empty_pb.Empty>;
    responseDeserialize: grpc.deserialize<google_protobuf_empty_pb.Empty>;
}

export const EventsService: IEventsService;

export interface IEventsServer extends grpc.UntypedServiceImplementation {
    streamEvents: grpc.handleClientStreamingCall<pulumi_events_pb.EventRequest, google_protobuf_empty_pb.Empty>;
}

export interface IEventsClient {
    streamEvents(callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientWritableStream<pulumi_events_pb.EventRequest>;
    streamEvents(metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientWritableStream<pulumi_events_pb.EventRequest>;
    streamEvents(options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientWritableStream<pulumi_events_pb.EventRequest>;
    streamEvents(metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientWritableStream<pulumi_events_pb.EventRequest>;
}

export class EventsClient extends grpc.Client implements IEventsClient {
    constructor(address: string, credentials: grpc.ChannelCredentials, options?: Partial<grpc.ClientOptions>);
    public streamEvents(callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientWritableStream<pulumi_events_pb.EventRequest>;
    public streamEvents(metadata: grpc.Metadata, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientWritableStream<pulumi_events_pb.EventRequest>;
    public streamEvents(options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientWritableStream<pulumi_events_pb.EventRequest>;
    public streamEvents(metadata: grpc.Metadata, options: Partial<grpc.CallOptions>, callback: (error: grpc.ServiceError | null, response: google_protobuf_empty_pb.Empty) => void): grpc.ClientWritableStream<pulumi_events_pb.EventRequest>;
}
