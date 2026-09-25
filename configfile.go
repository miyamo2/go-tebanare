package tebanare

import (
	"fmt"

	"github.com/miyamo2/go-tebanare/internal/result"
)

// ConfigFileNames lists the configuration file names in lookup order.
// Every adapter looks for them at the repository root and uses the first
// one that exists.
var ConfigFileNames = []string{".gotebanare.yml", ".gotebanare.yaml"}

// CodeConfigIgnored is the code of the warning SelectConfigFile returns for
// a configuration file that is ignored because an earlier name exists.
const CodeConfigIgnored = result.CodeConfigIgnored

// SelectConfigFile returns the first name in ConfigFileNames for which
// exists reports true, or "" when none exists. It calls exists for every
// name and returns a warning for each later name that also exists.
func SelectConfigFile(exists func(name string) bool) (name string, diags []Diagnostic) {
	if exists == nil {
		return "", nil
	}
	for _, n := range ConfigFileNames {
		if !exists(n) {
			continue
		}
		if name == "" {
			name = n
			continue
		}
		diags = append(diags, Diagnostic{
			Severity: result.SeverityWarning,
			Code:     CodeConfigIgnored,
			Message:  fmt.Sprintf("%s is ignored because %s exists", n, name),
		})
	}
	return name, diags
}
