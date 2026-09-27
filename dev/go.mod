// Module dev holds development tools that are not part of the product.
// It is a separate module so that their dependencies stay out of the
// root go.mod.
module github.com/miyamo2/go-tebanare/dev

go 1.25.0

require (
	github.com/miyamo2/go-tebanare v0.0.0-00010101000000-000000000000
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2
	go.yaml.in/yaml/v3 v3.0.5
)

require (
	github.com/bmatcuk/doublestar/v4 v4.10.2 // indirect
	golang.org/x/text v0.14.0 // indirect
)

replace github.com/miyamo2/go-tebanare => ../
