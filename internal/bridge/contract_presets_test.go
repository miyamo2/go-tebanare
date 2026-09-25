package bridge

import (
	"encoding/json"
	"testing"
)

func TestContractPresets(t *testing.T) {
	checkContract(t, "presets.json", json.RawMessage(New().Presets()))
}
