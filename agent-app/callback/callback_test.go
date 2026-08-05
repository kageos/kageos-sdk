package callback

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSelectFuzzyItemRichContentJSON(t *testing.T) {
	item := SelectFuzzyItem{
		Value:    12,
		Label:    "团建地点投票",
		RichText: "<p>请选择候选地点。</p>",
		Files:    "vote/cover.jpg,vote/rules.pdf",
	}

	raw, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("marshal SelectFuzzyItem: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal SelectFuzzyItem JSON: %v", err)
	}
	if got["rich_text"] != item.RichText {
		t.Fatalf("rich_text = %#v, want %#v", got["rich_text"], item.RichText)
	}
	if got["files"] != item.Files {
		t.Fatalf("files = %#v, want %#v", got["files"], item.Files)
	}
}

func TestSelectFuzzyItemOmitsEmptyOptionalDisplayFields(t *testing.T) {
	raw, err := json.Marshal(SelectFuzzyItem{Value: 1, Label: "选项"})
	if err != nil {
		t.Fatalf("marshal SelectFuzzyItem: %v", err)
	}
	got := string(raw)
	for _, unwanted := range []string{`"rich_text"`, `"files"`, `"display_info"`} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("SelectFuzzyItem JSON %s unexpectedly contains %s", got, unwanted)
		}
	}
}
