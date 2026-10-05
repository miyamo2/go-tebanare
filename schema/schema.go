// Package schema holds the JSON Schema of the configuration file,
// gotebanare.schema.json. The schema is the source of truth for the keys,
// types, defaults, and constraints of the configuration: the engine
// validates every configuration against it, and the Go types that hold a
// decoded configuration are generated from it (see dev/gogen).
package schema

import _ "embed"

// JSON is the content of gotebanare.schema.json.
//
//go:embed gotebanare.schema.json
var JSON []byte

// ID is the $id of the schema: the URL where it is published, the file
// schema/gotebanare.schema.json on the main branch.
const ID = "https://raw.githubusercontent.com/miyamo2/go-tebanare/main/schema/gotebanare.schema.json"
