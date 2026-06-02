// Copyright 2024, Pulumi Corporation.  All rights reserved.

import {
    Environment,
    EnvironmentDefinitionValues,
    OpenEnvironment,
    OrgEnvironments,
    OrgEnvironment,
    EnvironmentDefinition,
    EscApi as EscRawApi,
    Configuration,
    Value,
    EnvironmentDiagnostics,
    CheckEnvironment,
    Pos,
    Range,
    Trace,
    EnvironmentRevision,
    EnvironmentRevisionTag,
    EnvironmentRevisionTags,
    EnvironmentTag,
    ListEnvironmentTags,
} from "./raw/index";
import * as yaml from "js-yaml";
import { AxiosError } from "axios";
import { getCurrentAccount } from "./workspace";

export {
    Configuration,
    Environment,
    EnvironmentDefinitionValues,
    OpenEnvironment,
    OrgEnvironments,
    OrgEnvironment,
    EnvironmentDefinition,
    EscRawApi,
    Value,
    EnvironmentDiagnostics,
    CheckEnvironment,
    Pos,
    Range,
    Trace,
    EnvironmentRevision,
    EnvironmentRevisionTag,
    EnvironmentRevisionTags,
};

export interface EnvironmentDefinitionResponse {
    definition: EnvironmentDefinition;
    yaml: string;
}

export interface EnvironmentResponse {
    environment?: Environment;
    values?: EnvironmentDefinitionValues;
}

export interface EnvironmentPropertyResponse {
    property: Value;
    value: any;
}

export interface CloneEnvironmentOptions {
    preserveHistory?: boolean;
    preserveAccess?: boolean;
    preserveEnvironmentTags?: boolean;
    preserveRevisionTags?: boolean;
}

type KeyValueMap = { [key: string]: string };

/**
 *
 * EscApi is a client for the ESC API.
 * It wraps the raw API client and provides a more convenient interface.
 * @export
 * @class EscApi
 */
export class EscApi {
    rawApi: EscRawApi;
    config: Configuration;
    constructor(config: Configuration) {
        // Normalize backend url
        if (config.basePath) {
            const url = new URL(config.basePath)
            const appendedUrl = new URL(`/api/esc`, `${url.protocol}//${url.hostname}`);
            config.basePath = appendedUrl.toString();
        }

        this.config = config;
        this.rawApi = new EscRawApi(config);
    }

    /**
     * listEnvironments lists the environments in an organization.
     * @summary List environments
     * @param {string} orgName Organization name
     * @param {string} continuationToken continuation Token from previous query to fetch next page of results
     * @returns {Promise<OrgEnvironments | undefined>} A list of environments
     */
    async listEnvironments(orgName: string, continuationToken?: string): Promise<OrgEnvironments | undefined> {
        const resp = await this.rawApi.listEnvironments(orgName, continuationToken);
        if (resp.status === 200) {
            return resp.data;
        }

        throw new Error(`Failed to list environments: ${resp.statusText}`);
    }

    /**
     * getEnvironment gets the definition of an environment.
     * @summary Get environment
     * @param {string} orgName Organization name
     * @param {string} projectName Project name
     * @param {string} envName Environment name
     * @returns {Promise<EnvironmentDefinitionResponse | undefined>} The environment definition and the YAML representation
     */
    async getEnvironment(
        orgName: string,
        projectName: string,
        envName: string,
    ): Promise<EnvironmentDefinitionResponse | undefined> {
        const resp = await this.rawApi.getEnvironment(orgName, projectName, envName);
        if (resp.status === 200) {
            const doc = yaml.load(resp.data as string);
            return {
                definition: doc as EnvironmentDefinition,
                yaml: resp.data as string,
            };
        }

        throw new Error(`Failed to get environment: ${resp.statusText}`);
    }

    /**
     * getEnvironmentAtVersion gets the definition of an environment at a specific version.
     * @summary Get environment at version
     * @param {string} orgName Organization name
     * @param {string} projectName Project name
     * @param {string} envName Environment name
     * @param {string} version Version of the environment
     * @returns {Promise<EnvironmentDefinitionResponse | undefined>} The environment definition and the YAML representation
     */
    async getEnvironmentAtVersion(
        orgName: string,
        projectName: string,
        envName: string,
        version: string,
    ): Promise<EnvironmentDefinitionResponse | undefined> {
        const resp = await this.rawApi.getEnvironmentAtVersion(orgName, projectName, envName, version);
        if (resp.status === 200) {
            const doc = yaml.load(resp.data as string);
            return {
                definition: doc as EnvironmentDefinition,
                yaml: resp.data as string,
            };
        }

        throw new Error(`Failed to get environment: ${resp.statusText}`);
    }

    /**
     * openEnvironment opens an environment session
     * @summary Open environment
     * @param {string} orgName Organization name
     * @param {string} projectName Project name
     * @param {string} envName Environment name
     * @returns {Promise<OpenEnvironment | undefined>} The open environment session information
     */
    async openEnvironment(orgName: string, projectName: string, envName: string): Promise<OpenEnvironment | undefined> {
        const resp = await this.rawApi.openEnvironment(orgName, projectName, envName);
        if (resp.status === 200) {
            return resp.data;
        }

        throw new Error(`Failed to open environment: ${resp.statusText}`);
    }

    /**
     * openEnvironmentAtVersion opens an environment session at a specific version
     * @summary Open environment at version
     * @param {string} orgName Organization name
     * @param {string} projectName Project name
     * @param {string} envName Environment name
     * @param {string} version Version of the environment
     * @returns {Promise<OpenEnvironment | undefined>} The open environment session information
     */
    async openEnvironmentAtVersion(
        orgName: string,
        projectName: string,
        envName: string,
        version: string,
    ): Promise<OpenEnvironment | undefined> {
        const resp = await this.rawApi.openEnvironmentAtVersion(orgName, projectName, envName, version);
        if (resp.status === 200) {
            return resp.data;
        }

        throw new Error(`Failed to open environment: ${resp.statusText}`);
    }

    /**
     * readOpenEnvironment reads the environment properties in an open session,
     * resolving configuration variables and secrets.
     * @summary Read environment
     * @param {string} orgName Organization name
     * @param {string} projectName Project name
     * @param {string} envName Environment name
     * @param {string} openSessionID Open session ID
     * @returns {Promise<EnvironmentResponse | undefined>} The environment and its values
     */
    async readOpenEnvironment(
        orgName: string,
        projectName: string,
        envName: string,
        openSessionID: string,
    ): Promise<EnvironmentResponse | undefined> {
        const resp = await this.rawApi.readOpenEnvironment(orgName, projectName, envName, openSessionID);
        if (resp.status === 200) {
            return {
                environment: resp.data,
                values: convertEnvPropertiesToValues(resp.data.properties),
            };
        }

        throw new Error(`Failed to read environment: ${resp.statusText}`);
    }

    /**
     * openAndReadEnvironment opens an environment session and reads the environment properties,
     * resolving configuration variables and secrets.
     * @summary Open and read environment
     * @param {string} orgName Organization name
     * @param {string} projectName Project name
     * @param {string} envName Environment name
     * @returns {Promise<EnvironmentResponse | undefined>} The environment and its values
     */
    async openAndReadEnvironment(
        orgName: string,
        projectName: string,
        envName: string,
    ): Promise<EnvironmentResponse | undefined> {
        const open = await this.openEnvironment(orgName, projectName, envName);
        if (open?.id) {
            return await this.readOpenEnvironment(orgName, projectName, envName, open.id);
        }

        throw new Error(`Failed to open and read environment: ${open}`);
    }

    /**
     * openAndReadEnvironmentAtVersion opens an environment session at a specific version and reads the environment properties,
     * resolving configuration variables and secrets.
     * @summary Open and read environment at version
     * @param {string} orgName Organization name
     * @param {string} projectName Project name
     * @param {string} envName Environment name
     * @param {string} version Version of the environment
     * @returns {Promise<EnvironmentResponse | undefined>} The environment and its values
     */
    async openAndReadEnvironmentAtVersion(
        orgName: string,
        projectName: string,
        envName: string,
        version: string,
    ): Promise<EnvironmentResponse | undefined> {
        const open = await this.openEnvironmentAtVersion(orgName, projectName, envName, version);
        if (open?.id) {
            return await this.readOpenEnvironment(orgName, projectName, envName, open.id);
        }

        throw new Error(`Failed to open and read environment: ${open}`);
    }

    /**
     * readOpenEnvironmentProperty reads a specific environment property in an open session,
     * resolving configuration variables and secrets.
     * @summary Read environment property
     * @param {string} orgName Organization name
     * @param {string} projectName Project name
     * @param {string} envName Environment name
     * @param {string} openSessionID Open session ID
     * @param {string} property Property name
     * @returns {Promise<EnvironmentPropertyResponse | undefined>} The environment property and its value
     */
    async readOpenEnvironmentProperty(
        orgName: string,
        projectName: string,
        envName: string,
        openSessionID: string,
        property: string,
    ): Promise<EnvironmentPropertyResponse | undefined> {
        const resp = await this.rawApi.readOpenEnvironmentProperty(
            orgName,
            projectName,
            envName,
            openSessionID,
            property,
        );
        if (resp.status === 200) {
            return {
                property: resp.data,
                value: convertPropertyToValue(resp.data),
            };
        }

        throw new Error(`Failed to read environment property: ${resp.statusText}`);
    }

    /**
     * createEnvironment creates a new environment.
     * @summary Create environment
     * @param {string} orgName Organization name
     * @param {string} projectName Project name
     * @param {string} envName Environment name
     * @returns {Promise<void>} A promise that resolves when the environment is created
     */
    async createEnvironment(orgName: string, projectName: string, envName: string): Promise<void> {
        const body = {
            project: projectName,
            name: envName,
        };

        const resp = await this.rawApi.createEnvironment(orgName, body);
        if (resp.status === 204) {
            return;
        }

        throw new Error(`Failed to create environment: ${resp.statusText}`);
    }

    /**
     * cloneEnvironment clones an environment
     * @summary Clone environment
     * @param {string} orgName Organization name
     * @param {string} cloneProjectName Clone project name
     * @param {string} cloneEnvName Clone environment name
     * @param {string} destProjectName Destination project name
     * @param {string} destEnvName Destionation environment name
     * @param {CloneEnvironmentOptions} cloneOptions Clone options
     * @returns {Promise<void>} A promise that resolves when the environment is created
     */
    async cloneEnvironment(
        orgName: string,
        srcProjectName: string,
        srcEnvName: string,
        destProjectName: string,
        destEnvName: string,
        cloneOptions?: CloneEnvironmentOptions,
    ): Promise<void> {
        const body = {
            project: destProjectName,
            name: destEnvName,
            ...cloneOptions,
        };

        const resp = await this.rawApi.cloneEnvironment(orgName, srcProjectName, srcEnvName, body);
        if (resp.status === 204) {
            return;
        }

        throw new Error(`Failed to clone environment: ${resp.statusText}`);
    }

    /**
     * updateEnvironmentYaml updates the environment definition from a YAML string.
     * @summary Update environment YAML
     * @param {string} orgName Organization name
     * @param {string} projectName Project name
     * @param {string} envName Environment name
     * @param {string} yaml YAML representation of the environment
     * @returns {Promise<EnvironmentDiagnostics | undefined>} The environment diagnostics
     */
    async updateEnvironmentYaml(
        orgName: string,
        projectName: string,
        envName: string,
        yaml: string,
    ): Promise<EnvironmentDiagnostics | undefined> {
        const resp = await this.rawApi.updateEnvironmentYaml(orgName, projectName, envName, yaml);
        if (resp.status === 200) {
            return resp.data;
        }

        throw new Error(`Failed to update environment: ${resp.statusText}`);
    }

    /**
     * updateEnvironment updates the environment definition.
     * @summary Update environment
     * @param {string} orgName Organization name
     * @param {string} projectName Project name
     * @param {string} envName Environment name
     * @param {EnvironmentDefinition} values The environment definition
     * @returns {Promise<EnvironmentDiagnostics | undefined>} The environment diagnostics
     */
    async updateEnvironment(
        orgName: string,
        projectName: string,
        envName: string,
        values: EnvironmentDefinition,
    ): Promise<EnvironmentDiagnostics | undefined> {
        const body = yaml.dump(values);
        const resp = await this.rawApi.updateEnvironmentYaml(orgName, projectName, envName, body);
        if (resp.status === 200) {
            return resp.data;
        }

        throw new Error(`Failed to update environment: ${resp.statusText}`);
    }

    /**
     * deleteEnvironment deletes an environment.
     * @summary Delete environment
     * @param {string} orgName Organization name
     * @param {string} projectName Project name
     * @param {string} envName Environment name
     * @returns {Promise<void>} A promise that resolves when the environment is deleted
     */
    async deleteEnvironment(orgName: string, projectName: string, envName: string): Promise<void> {
        const resp = await this.rawApi.deleteEnvironment(orgName, projectName, envName);
        if (resp.status === 204) {
            return;
        }

        throw new Error(`Failed to delete environment: ${resp.statusText}`);
    }

    /**
     * checkEnvironmentYaml checks the environment definition from a YAML string.
     * @summary Check environment YAML
     * @param {string} orgName Organization name
     * @param {string} yaml YAML representation of the environment
     * @returns {Promise<CheckEnvironment | undefined>} The environment diagnostics
     */
    async checkEnvironmentYaml(orgName: string, yaml: string): Promise<CheckEnvironment | undefined> {
        try {
            const resp = await this.rawApi.checkEnvironmentYaml(orgName, yaml);
            if (resp.status === 200) {
                return resp.data;
            }

            throw new Error(`Failed to check environment: ${resp.statusText}`);
        } catch (err: any) {
            if (err instanceof AxiosError) {
                if (err.response?.status === 400) {
                    return err.response?.data;
                }
            }
            throw err;
        }
    }

    /**
     * checkEnvironment checks the environment definition.
     * @summary Check environment
     * @param {string} orgName Organization name
     * @param {EnvironmentDefinition} env The environment definition
     * @returns {Promise<CheckEnvironment | undefined>} The environment diagnostics
     */
    async checkEnvironment(orgName: string, env: EnvironmentDefinition): Promise<CheckEnvironment | undefined> {
        const body = yaml.dump(env);
        return await this.checkEnvironmentYaml(orgName, body);
    }

    /**
     * decryptEnvironment decrypts the environment definition.
     * @summary Decrypt environment
     * @param {string} orgName Organization name
     * @param {string} projectName Project name
     * @param {string} envName Environment name
     * @returns {Promise<EnvironmentDefinitionResponse | undefined>} The decrypted environment definition and the YAML representation
     */
    async decryptEnvironment(
        orgName: string,
        projectName: string,
        envName: string,
    ): Promise<EnvironmentDefinitionResponse | undefined> {
        const resp = await this.rawApi.decryptEnvironment(orgName, projectName, envName);
        if (resp.status === 200) {
            const doc = yaml.load(resp.data as string);
            return {
                definition: doc as EnvironmentDefinition,
                yaml: resp.data as string,
            };
        }

        throw new Error(`Failed to decrypt environment: ${resp.statusText}`);
    }

    /**
     * listEnvironmentRevisions lists the environment revisions, from oldest to newest.
     * @summary List environment revisions
     * @param {string} orgName Organization name
     * @param {string} projectName Project name
     * @param {string} envName Environment name
     * @param {number} before The revision number to start listing from
     * @param {number} count The number of revisions to list
     * @returns {Promise<Array<EnvironmentRevision> | undefined>} A list of environment revisions
     */
    async listEnvironmentRevisions(
        orgName: string,
        projectName: string,
        envName: string,
        before?: number,
        count?: number,
    ): Promise<Array<EnvironmentRevision> | undefined> {
        const resp = await this.rawApi.listEnvironmentRevisions(orgName, projectName, envName, before, count);
        if (resp.status === 200) {
            return resp.data;
        }

        throw new Error(`Failed to list environment revisions: ${resp.statusText}`);
    }

    /**
     * listEnvironmentRevisionTags lists the environment revision tags.
     * @summary List environment revision tags
     * @param {string} orgName Organization name
     * @param {string} projectName Project name
     * @param {string} envName Environment name
     * @param {string} after The tag to start listing from
     * @param {number} count The number of tags to list
     * @returns {Promise<EnvironmentRevisionTags | undefined>} A list of environment revision tags
     */
    async listEnvironmentRevisionTags(
        orgName: string,
        projectName: string,
        envName: string,
        after?: string,
        count?: number,
    ): Promise<EnvironmentRevisionTags | undefined> {
        const resp = await this.rawApi.listEnvironmentRevisionTags(orgName, projectName, envName, after, count);
        if (resp.status === 200) {
            return resp.data;
        }

        throw new Error(`Failed to list environment revision tags: ${resp.statusText}`);
    }

    /**
     * getEnvironmentRevisionTag gets the environment revision tag.
     * @summary Get environment revision tag
     * @param {string} orgName Organization name
     * @param {string} projectName Project name
     * @param {string} envName Environment name
     * @param {string} tag The tag name
     * @returns {Promise<EnvironmentRevisionTag | undefined>} The environment revision tag
     */
    async getEnvironmentRevisionTag(
        orgName: string,
        projectName: string,
        envName: string,
        tag: string,
    ): Promise<EnvironmentRevisionTag | undefined> {
        const resp = await this.rawApi.getEnvironmentRevisionTag(orgName, projectName, envName, tag);
        if (resp.status === 200) {
            return resp.data;
        }

        throw new Error(`Failed to get environment revision tag: ${resp.statusText}`);
    }

    /**
     * createEnvironmentRevisionTag creates a new environment revision tag.
     * @summary Create environment revision tag
     * @param {string} orgName Organization name
     * @param {string} projectName Project name
     * @param {string} envName Environment name
     * @param {string} tag The tag name
     * @param {number} revision The revision number
     * @returns {Promise<void>} A promise that resolves when the tag is created
     */
    async createEnvironmentRevisionTag(
        orgName: string,
        projectName: string,
        envName: string,
        tag: string,
        revision: number,
    ): Promise<void> {
        const createTag = {
            name: tag,
            revision: revision,
        };

        const resp = await this.rawApi.createEnvironmentRevisionTag(orgName, projectName, envName, createTag);
        if (resp.status === 204) {
            return;
        }

        throw new Error(`Failed to create environment revision tag: ${resp.statusText}`);
    }

    /**
     * updateEnvironmentRevisionTag updates the environment revision tag.
     * @summary Update environment revision tag
     * @param {string} orgName Organization name
     * @param {string} projectName Project name
     * @param {string} envName Environment name
     * @param {string} tag The tag name
     * @param {number} revision The revision number
     * @returns {Promise<void>} A promise that resolves when the tag is updated
     */
    async updateEnvironmentRevisionTag(
        orgName: string,
        projectName: string,
        envName: string,
        tag: string,
        revision: number,
    ): Promise<void> {
        const updateTag = {
            revision: revision,
        };
        const resp = await this.rawApi.updateEnvironmentRevisionTag(orgName, projectName, envName, tag, updateTag);
        if (resp.status === 204) {
            return;
        }

        throw new Error(`Failed to update environment revision tag: ${resp.statusText}`);
    }

    /**
     * deleteEnvironmentRevisionTag deletes the environment revision tag.
     * @summary Delete environment revision tag
     * @param {string} orgName Organization name
     * @param {string} projectName Project name
     * @param {string} envName Environment name
     * @param {string} tag The tag name
     * @returns {Promise<void>} A promise that resolves when the tag is deleted
     */
    async deleteEnvironmentRevisionTag(
        orgName: string,
        projectName: string,
        envName: string,
        tag: string,
    ): Promise<void> {
        const resp = await this.rawApi.deleteEnvironmentRevisionTag(orgName, projectName, envName, tag);
        if (resp.status === 204) {
            return;
        }

        throw new Error(`Failed to delete environment revision tag: ${resp.statusText}`);
    }

    /**
     * listEnvironmentTags lists the environment tags.
     * @summary List environment tags
     * @param {string} orgName Organization name
     * @param {string} projectName Project name
     * @param {string} envName Environment name
     * @param {string} after The tag to start listing from
     * @param {number} count The number of tags to list
     * @returns {Promise<ListEnvironmentTags | undefined>} A list of environment tags
     */
    async listEnvironmentTags(
        orgName: string,
        projectName: string,
        envName: string,
        after?: string,
        count?: number,
    ): Promise<ListEnvironmentTags | undefined> {
        const resp = await this.rawApi.listEnvironmentTags(orgName, projectName, envName, after, count);
        if (resp.status === 200) {
            return resp.data;
        }

        throw new Error(`Failed to list environment tags: ${resp.statusText}`);
    }

    /**
     * getEnvironmentTag gets the environment tag.
     * @summary Get environment tag
     * @param {string} orgName Organization name
     * @param {string} projectName Project name
     * @param {string} envName Environment name
     * @param {string} tag The tag name
     * @returns {Promise<EnvironmentTag | undefined>} The environment tag
     */
    async getEnvironmentTag(
        orgName: string,
        projectName: string,
        envName: string,
        tag: string,
    ): Promise<EnvironmentTag | undefined> {
        const resp = await this.rawApi.getEnvironmentTag(orgName, projectName, envName, tag);
        if (resp.status === 200) {
            return resp.data;
        }

        throw new Error(`Failed to get environment tag: ${resp.statusText}`);
    }

    /**
     * createEnvironmentTag creates a new environment tag.
     * @summary Create environment tag
     * @param {string} orgName Organization name
     * @param {string} projectName Project name
     * @param {string} envName Environment name
     * @param {string} tag The tag name
     * @param {string} value The tag value
     * @returns {Promise<EnvironmentTag>} A promise that resolves when the tag is created
     */
    async createEnvironmentTag(
        orgName: string,
        projectName: string,
        envName: string,
        tag: string,
        value: string,
    ): Promise<void> {
        const createTag = {
            name: tag,
            value: value,
        };

        const resp = await this.rawApi.createEnvironmentTag(orgName, projectName, envName, createTag);
        if (resp.status === 200) {
            return;
        }

        throw new Error(`Failed to create environment tag: ${resp.statusText}`);
    }

    /**
     * updateEnvironmentTag updates the environment tag.
     * @summary Update environment tag
     * @param {string} orgName Organization name
     * @param {string} projectName Project name
     * @param {string} envName Environment name
     * @param {string} tag The tag name
     * @param {string} current_value The tag value
     * @param {string} new_tag The new tag name
     * @param {string} new_value The new tag value
     * @returns {Promise<EnvironmentTag>} A promise that resolves when the tag is updated
     */
    async updateEnvironmentTag(
        orgName: string,
        projectName: string,
        envName: string,
        tag: string,
        current_value: string,
        new_tag: string,
        new_value: string,
    ): Promise<void> {
        const updateTag = {
            currentTag: {
                value: current_value,
            },
            newTag: {
                name: new_tag,
                value: new_value,
            },
        };
        const resp = await this.rawApi.updateEnvironmentTag(orgName, projectName, envName, tag, updateTag);
        if (resp.status === 200) {
            return;
        }

        throw new Error(`Failed to update environment tag: ${resp.statusText}`);
    }

    /**
     * deleteEnvironmentTag deletes the environment tag.
     * @summary Delete environment tag
     * @param {string} orgName Organization name
     * @param {string} projectName Project name
     * @param {string} envName Environment name
     * @param {string} tag The tag name
     * @returns {Promise<void>} A promise that resolves when the tag is deleted
     */
    async deleteEnvironmentTag(orgName: string, projectName: string, envName: string, tag: string): Promise<void> {
        const resp = await this.rawApi.deleteEnvironmentTag(orgName, projectName, envName, tag);
        if (resp.status === 204) {
            return;
        }

        throw new Error(`Failed to delete environment tag: ${resp.statusText}`);
    }
}

function convertEnvPropertiesToValues(env: { [key: string]: Value } | undefined): KeyValueMap {
    if (!env) {
        return {};
    }

    const values: KeyValueMap = {};
    for (const key in env) {
        const value = env[key];

        values[key] = convertPropertyToValue(value);
    }

    return values;
}

function convertPropertyToValue(property: any): any {
    if (!property) {
        return property;
    }

    let value = property;
    if (typeof property === "object" && "value" in property) {
        value = convertPropertyToValue(property.value);
    }

    if (!value) {
        return value;
    }

    if (Array.isArray(value)) {
        const array = value as Value[];
        return array.map((v) => convertPropertyToValue(v));
    }

    if (typeof value === "object") {
        const result: any = {};
        for (const key in value) {
            result[key] = convertPropertyToValue(value[key]);
        }

        return result;
    }

    return value;
}

export function DefaultConfiguration(config?: Configuration): Configuration {
    if (!config) {
        config = new Configuration()
    }
    config.accessToken ??= process.env.PULUMI_ACCESS_TOKEN;
    config.basePath ??= process.env.PULUMI_BACKEND_URL;

    if (!config.accessToken || !config.basePath) {
        const {account, backendUrl} = getCurrentAccount();
        config.accessToken ??= account?.accessToken;
        config.basePath ??= backendUrl;
    }

    return config;
}

export function DefaultClient(config?: Configuration) {
    return new EscApi(DefaultConfiguration(config));
}