package test

import "embed"

//go:embed configs/*json
var TestCustomChainConfigFS embed.FS

//go:embed configs_empty/*json
var TestCustomChainConfigEmptyFS embed.FS

//go:embed configs_typo/*json
var TestCustomChainConfigTypoFS embed.FS
