package toolcrds

import "embed"

//go:embed flux/*/*.yaml
var FluxCRDs embed.FS

//go:embed kro/*/*.yaml
var KROCRDs embed.FS

//go:embed ocm/*/*.yaml
var OCMCRDs embed.FS
