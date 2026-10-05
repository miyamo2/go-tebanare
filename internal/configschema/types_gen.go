// Code generated from JSON Schema using quicktype. DO NOT EDIT.
// To parse and unparse this JSON data, add this code to your project and do:
//
//    config, err := UnmarshalConfig(bytes)
//    bytes, err = config.Marshal()

package configschema

import "bytes"
import "errors"

import "encoding/json"

func UnmarshalConfig(data []byte) (Config, error) {
	var r Config
	err := json.Unmarshal(data, &r)
	return r, err
}

func (r *Config) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

// Configuration of go-tebanare, read from `.gotebanare.yml` or `.gotebanare.yaml` at the
// repository root. See
// https://github.com/miyamo2/go-tebanare/blob/main/docs/configuration.md.
type Config struct {
	// The files to analyze.
	Files *Files `json:"files"`
	// Built-in presets to enable. Listing a preset twice is an error.
	Presets []PresetElement `json:"presets"`
	// Version of the configuration format. Must be `1`.
	Version int64 `json:"version"`
}

type Files struct {
	// Files to skip even when `include` matches them.
	Exclude []string `json:"exclude"`
	// Files to analyze, as doublestar globs relative to the repository root. The default, also
	// used for an empty list, is `["**/*.go"]`.
	Include []string `json:"include"`
}

// A preset name as the only key, with its settings as the value.
type PresetClass struct {
	Getter *GetterSettings `json:"getter"`
	Iferr  *IferrSettings  `json:"iferr"`
	Noop   *NoopSettings   `json:"noop"`
}

type GetterSettings struct {
	// Globs of files this preset skips, on top of `files.exclude`. Default: none.
	ExcludePaths []string `json:"exclude_paths"`
	// Hide the doc comment together with the function. Default: true.
	IncludeDoc *bool `json:"include_doc"`
	// The maximum number of fields in the returned chain: with 1, `u.name` matches and
	// `u.cfg.timeout` does not. Default: unlimited.
	MaxDepth *int64 `json:"max_depth"`
	// Globs that limit the files this preset applies to, on top of `files.include` and
	// `files.exclude`. Default: none.
	Paths []string `json:"paths"`
}

type IferrSettings struct {
	// Also hide a bare return. Default: false.
	AllowBareReturn *bool `json:"allow_bare_return"`
	// Allow function calls in the results before the error; function literals and channel
	// receives still do not match. Default: false.
	AllowCallsInResults *bool `json:"allow_calls_in_results"`
	// Hide the statement even when a comment is on the hidden lines. Default: false.
	AllowComments *bool `json:"allow_comments"`
	// Globs of files this preset skips, on top of `files.exclude`. Default: none.
	ExcludePaths []string `json:"exclude_paths"`
	// How to treat if statements with an init statement: `exclude` keeps them visible;
	// `fold-body` hides the lines from `return` to the closing brace and keeps the header line
	// visible. Default: exclude.
	Init *Init `json:"init"`
	// Names of error variables, as globs matched against the whole name, where `*` matches any
	// run of characters and `?` matches one. Default: [err].
	Names []string `json:"names"`
	// Globs that limit the files this preset applies to, on top of `files.include` and
	// `files.exclude`. Default: none.
	Paths []string `json:"paths"`
}

type NoopSettings struct {
	// Hide the method even when a comment other than the doc comment is on its lines. Default:
	// true.
	AllowComments *bool `json:"allow_comments"`
	// Globs of files this preset skips, on top of `files.exclude`. Default: none.
	ExcludePaths []string `json:"exclude_paths"`
	// Hide the doc comment together with the function. Default: true.
	IncludeDoc *bool `json:"include_doc"`
	// Also hide empty functions that are not methods, such as `func noop() {}`. Default: false.
	IncludeFunctions *bool `json:"include_functions"`
	// Globs that limit the files this preset applies to, on top of `files.include` and
	// `files.exclude`. Default: none.
	Paths []string `json:"paths"`
}

type Init string

const (
	Exclude  Init = "exclude"
	FoldBody Init = "fold-body"
)

// A preset with its default settings.
type PresetEnum string

const (
	Getter PresetEnum = "getter"
	Iferr  PresetEnum = "iferr"
	Noop   PresetEnum = "noop"
)

// A preset name, or `name: settings` to change its settings.
type PresetElement struct {
	Enum        *PresetEnum
	PresetClass *PresetClass
}

func (x *PresetElement) UnmarshalJSON(data []byte) error {
	x.PresetClass = nil
	x.Enum = nil
	var c PresetClass
	object, err := unmarshalUnion(data, nil, nil, nil, nil, false, nil, true, &c, false, nil, true, &x.Enum, false)
	if err != nil {
		return err
	}
	if object {
		x.PresetClass = &c
	}
	return nil
}

func (x *PresetElement) MarshalJSON() ([]byte, error) {
	return marshalUnion(nil, nil, nil, nil, false, nil, x.PresetClass != nil, x.PresetClass, false, nil, x.Enum != nil, x.Enum, false)
}

func unmarshalUnion(data []byte, pi **int64, pf **float64, pb **bool, ps **string, haveArray bool, pa interface{}, haveObject bool, pc interface{}, haveMap bool, pm interface{}, haveEnum bool, pe interface{}, nullable bool) (bool, error) {
	if pi != nil {
		*pi = nil
	}
	if pf != nil {
		*pf = nil
	}
	if pb != nil {
		*pb = nil
	}
	if ps != nil {
		*ps = nil
	}

	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	tok, err := dec.Token()
	if err != nil {
		return false, err
	}

	switch v := tok.(type) {
	case json.Number:
		if pi != nil {
			i, err := v.Int64()
			if err == nil {
				*pi = &i
				return false, nil
			}
		}
		if pf != nil {
			f, err := v.Float64()
			if err == nil {
				*pf = &f
				return false, nil
			}
			return false, errors.New("Unparsable number")
		}
		return false, errors.New("Union does not contain number")
	case float64:
		return false, errors.New("Decoder should not return float64")
	case bool:
		if pb != nil {
			*pb = &v
			return false, nil
		}
		return false, errors.New("Union does not contain bool")
	case string:
		if haveEnum {
			return false, json.Unmarshal(data, pe)
		}
		if ps != nil {
			*ps = &v
			return false, nil
		}
		return false, errors.New("Union does not contain string")
	case nil:
		if nullable {
			return false, nil
		}
		return false, errors.New("Union does not contain null")
	case json.Delim:
		if v == '{' {
			if haveObject {
				return true, json.Unmarshal(data, pc)
			}
			if haveMap {
				return false, json.Unmarshal(data, pm)
			}
			return false, errors.New("Union does not contain object")
		}
		if v == '[' {
			if haveArray {
				return false, json.Unmarshal(data, pa)
			}
			return false, errors.New("Union does not contain array")
		}
		return false, errors.New("Cannot handle delimiter")
	}
	return false, errors.New("Cannot unmarshal union")
}

func marshalUnion(pi *int64, pf *float64, pb *bool, ps *string, haveArray bool, pa interface{}, haveObject bool, pc interface{}, haveMap bool, pm interface{}, haveEnum bool, pe interface{}, nullable bool) ([]byte, error) {
	if pi != nil {
		return json.Marshal(*pi)
	}
	if pf != nil {
		return json.Marshal(*pf)
	}
	if pb != nil {
		return json.Marshal(*pb)
	}
	if ps != nil {
		return json.Marshal(*ps)
	}
	if haveArray {
		return json.Marshal(pa)
	}
	if haveObject {
		return json.Marshal(pc)
	}
	if haveMap {
		return json.Marshal(pm)
	}
	if haveEnum {
		return json.Marshal(pe)
	}
	if nullable {
		return json.Marshal(nil)
	}
	return nil, errors.New("Union must not be null")
}
