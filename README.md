[![REUSE status](https://api.reuse.software/badge/github.com/openmcp-project/local-event-showcase)](https://api.reuse.software/info/github.com/openmcp-project/local-event-showcase)

# local-event-showcase

## About this project

This repo contains scripts and documentation on how to setup a local platform of Gardener, OpenMCP and Platform Mesh to use in a showcase on conferences or events.

## Requirements

A machine with at least 8 CPU Cores and 32 GB RAM.

### OpenMCP

First you need to setup the local-registry:

```bash
task local-registry
```

Then you need to clone the distro for openmcp

```bash
task openmcp:clone-distro
```

After thats finished, you can run the local installation:

```bash
task openmcp:local
```

Now you have the running openmcp platform. To create an mcp with crossplane and the kubernetes provider installed:

```bash
task openmcp:create-mcp
```

## Support, Feedback, Contributing

This project is open to feature requests/suggestions, bug reports etc. via [GitHub issues](https://github.com/openmcp-project/local-event-showcase/issues). Contribution and feedback are encouraged and always welcome. For more information about how to contribute, the project structure, as well as additional contribution information, see our [Contribution Guidelines](CONTRIBUTING.md).

## Security / Disclosure
If you find any bug that may be a security problem, please follow our instructions at [in our security policy](https://github.com/openmcp-project/local-event-showcase/security/policy) on how to report it. Please do not create GitHub issues for security-related doubts or problems.

## Code of Conduct

We as members, contributors, and leaders pledge to make participation in our community a harassment-free experience for everyone. By participating in this project, you agree to abide by its [Code of Conduct](https://github.com/SAP/.github/blob/main/CODE_OF_CONDUCT.md) at all times.

## Licensing

Copyright 2026 SAP SE or an SAP affiliate company and local-event-showcase contributors. Please see our [LICENSE](LICENSE) for copyright and license information. Detailed information including third-party components and their licensing/copyright information is available [via the REUSE tool](https://api.reuse.software/info/github.com/openmcp-project/local-event-showcase).
