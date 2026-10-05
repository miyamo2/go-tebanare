package presets

import (
	"fmt"

	"github.com/miyamo2/go-tebanare/internal/configschema"
)

// Decode reads the settings of p from value, the value that follows the
// preset name in the config after it passed the schema: nil for the
// defaults, or a map[string]any. A setting that is missing or null gets
// its default from the schema. It returns a pointer to the settings type
// of p.
func Decode(p *Preset, value any) (any, error) {
	if p.newSettings == nil {
		return nil, fmt.Errorf("presets: %s has no settings type", p.Name)
	}
	s := p.newSettings()
	if err := configschema.DecodeSettings(p.Name, value, s); err != nil {
		return nil, err
	}
	return s, nil
}
