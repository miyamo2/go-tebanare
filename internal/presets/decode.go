package presets

import "fmt"

// Decode reads the settings of p from value, the value that follows the
// preset name in the config after it passed the schema: nil for the
// defaults, or a map[string]any. A setting whose value is nil keeps its
// default. It returns a pointer to the settings struct of p.
func Decode(p *Preset, value any) (any, error) {
	spec, ok := schemaPresets[p.Name]
	if !ok {
		return nil, fmt.Errorf("presets: the schema has no settings for preset %s", p.Name)
	}
	switch v := value.(type) {
	case nil:
		return spec.new(), nil
	case map[string]any:
		return spec.decode(v), nil
	}
	return nil, fmt.Errorf("presets: %s: settings have type %T, want a mapping", p.Name, value)
}
