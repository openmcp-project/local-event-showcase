# Overview

- this demo sets up openmcp, platform-mesh and gardener on a local setup using kind.

# Configuration

The Taskfile automatically loads environment variables from a `.env` file in the project root (if present).
Copy the example and adjust the values to point to your OCM repository and component:

```
cp .env.example .env
```

| Variable | Description | Default (when `.env` is absent) |
|---|---|---|
| `OPENMCP_OCM_REPOSITORY` | OCI registry hosting the OCM components | `ghcr.io/openmcp-project/components` |
| `OPENMCP_OCM_COMPONENT_NAME` | OCM component name of the openmcp distro | `github.com/openmcp-project/openmcp` |

> **Note:** The file must use plain `KEY=value` syntax (no `export` prefix) so that Taskfile's `dotenv` feature can parse it.

# Setup instructions
0. Delete any existing kind clusters that you may have that could conflict with this setup.
1. Download openmcp distro (takes longer, do when necessary)
  ```
  task openmcp:clone-distro
  ```
3. Install Platform-mesh
  ```
  task platform-mesh:local openmcp:local integrate
  ```


