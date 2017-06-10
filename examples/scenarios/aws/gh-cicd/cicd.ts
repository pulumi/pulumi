
// export interface Builders {
//     docker: DockerBuilder 
//     go: GoBuilder
// }

// export interface DockerBuilder {
//     build(opts?: BuildOpts): void;
//     push(image: string): void;
// }

// export interface BuildOpts {
//     tag?: string;
// }

// export interface GoBuilder {
//     test(): void;
//     golint(): void;
// }

// export interface Providers {
//     gcloud: GCloudProvider
// }

// export interface GCloudProvider {
//     rollingUpdate(opts: UpdateOpts): void;
// }

// export interface UpdateOpts {
//     name: string;
// }

// export let builders: Builders = {
//     docker: {
//         build: (opts: BuildOpts) => {},
//         push: (image: string) => {},
//     },
//     go: {
//         test: () => {},
//         golint: () => {},
//     },
// }

// export let providers: Providers = {
//     gcloud: {
//         rollingUpdate: (opts: UpdateOpts) => {},
//     }
// }