# External TypeScript loader fixture

This project runs an ESM TypeScript Pulumi program through `@nubjs/loader` while retaining the Node.js language host.

```yaml
runtime:
  name: nodejs
  options:
    typescript: false
    nodeargs: "--require @nubjs/loader"
```

The `--require` preload requires Node 22.15.0 or later for this fixture. The package script runs `tsc --noEmit` before deployment because `typescript: false` disables Pulumi's built-in `ts-node` registration. The program, providers, package manager, and deployment model otherwise remain unchanged.
