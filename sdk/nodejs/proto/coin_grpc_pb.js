// GENERATED CODE -- DO NOT EDIT!

// Original file comments:
// Copyright 2016-2018, Pulumi Corporation.  All rights reserved.
//
'use strict';
var grpc = require('grpc');
var coin_pb = require('./coin_pb.js');

function serialize_pulumirpc_DumpRequest(arg) {
  if (!(arg instanceof coin_pb.DumpRequest)) {
    throw new Error('Expected argument of type pulumirpc.DumpRequest');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_pulumirpc_DumpRequest(buffer_arg) {
  return coin_pb.DumpRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_DumpResponse(arg) {
  if (!(arg instanceof coin_pb.DumpResponse)) {
    throw new Error('Expected argument of type pulumirpc.DumpResponse');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_pulumirpc_DumpResponse(buffer_arg) {
  return coin_pb.DumpResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_OfferRequest(arg) {
  if (!(arg instanceof coin_pb.OfferRequest)) {
    throw new Error('Expected argument of type pulumirpc.OfferRequest');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_pulumirpc_OfferRequest(buffer_arg) {
  return coin_pb.OfferRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_OfferResponse(arg) {
  if (!(arg instanceof coin_pb.OfferResponse)) {
    throw new Error('Expected argument of type pulumirpc.OfferResponse');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_pulumirpc_OfferResponse(buffer_arg) {
  return coin_pb.OfferResponse.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_PumpRequest(arg) {
  if (!(arg instanceof coin_pb.PumpRequest)) {
    throw new Error('Expected argument of type pulumirpc.PumpRequest');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_pulumirpc_PumpRequest(buffer_arg) {
  return coin_pb.PumpRequest.deserializeBinary(new Uint8Array(buffer_arg));
}

function serialize_pulumirpc_PumpResponse(arg) {
  if (!(arg instanceof coin_pb.PumpResponse)) {
    throw new Error('Expected argument of type pulumirpc.PumpResponse');
  }
  return new Buffer(arg.serializeBinary());
}

function deserialize_pulumirpc_PumpResponse(buffer_arg) {
  return coin_pb.PumpResponse.deserializeBinary(new Uint8Array(buffer_arg));
}


// CoinProvider is an abstraction for new cryptocurrency ventures.
var CoinProviderService = exports.CoinProviderService = {
  // Offer offers a new Initial Coin Offering (ICO) to the investment and mining community at large.
  offer: {
    path: '/pulumirpc.CoinProvider/Offer',
    requestStream: false,
    responseStream: false,
    requestType: coin_pb.OfferRequest,
    responseType: coin_pb.OfferResponse,
    requestSerialize: serialize_pulumirpc_OfferRequest,
    requestDeserialize: deserialize_pulumirpc_OfferRequest,
    responseSerialize: serialize_pulumirpc_OfferResponse,
    responseDeserialize: deserialize_pulumirpc_OfferResponse,
  },
  // Pump that coin price!
  pump: {
    path: '/pulumirpc.CoinProvider/Pump',
    requestStream: false,
    responseStream: false,
    requestType: coin_pb.PumpRequest,
    responseType: coin_pb.PumpResponse,
    requestSerialize: serialize_pulumirpc_PumpRequest,
    requestDeserialize: deserialize_pulumirpc_PumpRequest,
    responseSerialize: serialize_pulumirpc_PumpResponse,
    responseDeserialize: deserialize_pulumirpc_PumpResponse,
  },
  // Dump that coin and get rich!
  dump: {
    path: '/pulumirpc.CoinProvider/Dump',
    requestStream: false,
    responseStream: false,
    requestType: coin_pb.DumpRequest,
    responseType: coin_pb.DumpResponse,
    requestSerialize: serialize_pulumirpc_DumpRequest,
    requestDeserialize: deserialize_pulumirpc_DumpRequest,
    responseSerialize: serialize_pulumirpc_DumpResponse,
    responseDeserialize: deserialize_pulumirpc_DumpResponse,
  },
};

exports.CoinProviderClient = grpc.makeGenericClientConstructor(CoinProviderService);
