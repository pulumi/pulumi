// Copyright 2016-2022, Pulumi Corporation.

import * as pulumi from '@pulumi/pulumi'

// A ResourceProvider using the default generic type, with explicit return type defined.
class DefaultGenericProvider implements pulumi.dynamic.ResourceProvider {
  async create (props: any): Promise<pulumi.dynamic.CreateResult> {
    return { id: 'resource-id', outs: {} }
  }

  async check (olds: any, news: any): Promise<pulumi.dynamic.CheckResult> {
    return Promise.resolve({ inputs: news })
  }

  async diff (id: pulumi.ID, olds: any, news: any): Promise<pulumi.dynamic.DiffResult> {
    return Promise.resolve({})
  }

  async delete (id: pulumi.ID, props: any): Promise<void> { return Promise.resolve() }

  async update (id: pulumi.ID, olds: any, news: any): Promise<pulumi.dynamic.UpdateResult> {
    return Promise.resolve({ outs: {} })
  }

  async read(id: pulumi.ID, props: any): Promise<pulumi.dynamic.ReadResult> {
    return Promise.resolve({ props: {} })
  }
}

type InputArgs = {
  names: string
}

type OutputArgs = {
  resourceId: string
  name: string
}

// All parameters and returns typed are inferred through the generic types provided.
class TypedGenericProvider implements pulumi.dynamic.ResourceProvider<InputArgs, OutputArgs> {
  async create (props) {
    return { id: 'resource-id', outs: { resourceId: "id", name: "test" } }
  }

  async check (olds, news) {
    return Promise.resolve({ inputs: news })
  }

  async diff (id, olds, news) {
    return Promise.resolve({})
  }

  async delete (id, props) { return Promise.resolve() }

  async update (id, olds, news) {
    return Promise.resolve({ outs: { resourceId: olds.resourceId, ...news } })
  }

  async read(id, props) {
    return Promise.resolve({ props: props })
  }
}
