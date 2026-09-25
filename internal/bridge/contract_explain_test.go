package bridge

import (
	"encoding/json"
	"testing"
)

const explainSource = `package p

import "context"

// Name returns the name.
func (u *User) Name() string { return u.name }

func (s *Service) Do(ctx context.Context, id string) error {
	log.Debug("do", "id", id)
	err := s.save(ctx)
	if err != nil {
		return err
	}
	return nil
}
`

type explainCase struct {
	Path string          `json:"path"`
	Line int             `json:"line"`
	Want json.RawMessage `json:"want"`
}

func TestContractExplain(t *testing.T) {
	yaml := sampleConfig(t, "plan-example.yml")
	b := New()
	h := compile(t, b, string(yaml))
	sources := map[string]string{"p.go": explainSource, "bad.go": "package"}
	var explains []explainCase
	for _, c := range []struct {
		path string
		line int
	}{{"p.go", 6}, {"p.go", 9}, {"p.go", 11}, {"p.go", 2}, {"bad.go", 1}} {
		meta, err := json.Marshal(explainMeta{Path: c.path, Line: c.line})
		if err != nil {
			t.Fatal(err)
		}
		out := b.Explain(h, meta, []byte(sources[c.path]))
		explains = append(explains, explainCase{c.path, c.line, out})
	}
	checkContract(t, "explain.json", map[string]any{
		"yaml":    string(yaml),
		"sources": sources,
		"cases":   explains,
	})
}
