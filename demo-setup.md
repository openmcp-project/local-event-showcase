# Overview

- this demo sets up openmcp, platform-mesh and gardener on a local setup using kind.

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


