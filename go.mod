module network_tool

go 1.26.1

require (
	github.com/stretchr/testify v1.11.1
	network_tool/lib v0.0.0-00010101000000-000000000000
	network_tool/tools/performance v0.0.0-00010101000000-000000000000
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace network_tool/utils => ./utils

replace network_tool/tools/performance => ./tools/performance

replace network_tool/lib => ./lib
