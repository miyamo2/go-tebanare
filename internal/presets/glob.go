package presets

import "unicode/utf8"

// matchGlob reports whether the whole of name matches the glob g, where
// "*" matches any run of runes and "?" matches one rune.
func matchGlob(g, name string) bool {
	// Iterative matching with one backtrack point: the last "*" seen.
	gi, ni := 0, 0
	starG, starN := -1, 0
	for ni < len(name) {
		if gi < len(g) {
			switch gc, gw := utf8.DecodeRuneInString(g[gi:]); gc {
			case '*':
				starG, starN = gi, ni
				gi += gw
				continue
			case '?':
				_, nw := utf8.DecodeRuneInString(name[ni:])
				gi, ni = gi+gw, ni+nw
				continue
			default:
				if nc, nw := utf8.DecodeRuneInString(name[ni:]); nc == gc {
					gi, ni = gi+gw, ni+nw
					continue
				}
			}
		}
		if starG < 0 {
			return false
		}
		_, nw := utf8.DecodeRuneInString(name[starN:])
		starN += nw
		gi, ni = starG+1, starN
	}
	for gi < len(g) && g[gi] == '*' {
		gi++
	}
	return gi == len(g)
}
