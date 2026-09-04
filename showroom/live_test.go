package showroom

import (
	"encoding/json"
	"strings"
	"testing"
)

// Real WebSocket messages captured from Showroom on 2026-03-30.
// Format: MSG\t<broadcast_key>\t<json_payload>
var capturedMessages = []struct {
	name     string
	raw      string
	wantType int
}{
	{
		name:     "stream_start_104",
		raw:      "MSG\t48291876c2526f267f9371f85f5d35799dfec8d800873d55b7f3c7bf593539a1\t{\"t\":104,\"created_at\":1774835763}",
		wantType: 104,
	},
	{
		name:     "stream_end_101",
		raw:      "MSG\t484b6a44787a7137:22715088\t{\"t\":101,\"created_at\":1774834800}",
		wantType: 101,
	},
	{
		name:     "comment_type_1",
		raw:      "MSG\t484b6a44787a7137:22715088\t{\"t\":1,\"u\":5026851,\"ac\":\"おかき\",\"cm\":\"こんにちは\",\"created_at\":1774834801}",
		wantType: 1,
	},
	{
		name:     "gift_type_2",
		raw:      "MSG\t484b6a44787a7137:22715088\t{\"t\":2,\"u\":3710290,\"ac\":\"kota\",\"g\":3000421,\"n\":10,\"created_at\":1774834804}",
		wantType: 2,
	},
	{
		name:     "telop_type_8",
		raw:      "MSG\t484b6a44787a7137:22715088\t{\"telops\":[],\"telop\":\"イベント中\",\"interval\":6000,\"t\":8}",
		wantType: 8,
	},
	{
		name:     "visit_type_18",
		raw:      "MSG\t484b6a44787a7137:22715088\t{\"t\":18,\"u\":6275250,\"m\":\"ヒノキさんが初訪問\",\"created_at\":1774834800}",
		wantType: 18,
	},
}

func TestParseWSMessage(t *testing.T) {
	for _, tc := range capturedMessages {
		t.Run(tc.name, func(t *testing.T) {
			parts := strings.Split(tc.raw, "\t")
			if len(parts) < 2 {
				t.Fatal("expected at least 2 tab-separated parts")
			}

			var msg wsMessage
			if err := json.Unmarshal([]byte(parts[len(parts)-1]), &msg); err != nil {
				t.Fatalf("json unmarshal failed: %v", err)
			}

			if msg.T != tc.wantType {
				t.Errorf("got type %d, want %d", msg.T, tc.wantType)
			}
		})
	}
}

// TestParseWSMessage_OldBug verifies that the old SplitN(2) parsing fails
// on the real 3-field message format, documenting why it was changed.
func TestParseWSMessage_OldBug(t *testing.T) {
	for _, tc := range capturedMessages {
		t.Run(tc.name, func(t *testing.T) {
			parts := strings.SplitN(tc.raw, "\t", 2)
			if len(parts) < 2 {
				t.Fatal("expected at least 2 parts")
			}

			var msg wsMessage

			err := json.Unmarshal([]byte(parts[len(parts)-1]), &msg)
			if err == nil {
				t.Error("expected old SplitN(2) parsing to fail on 3-field message, but it succeeded")
			}
		})
	}
}

func TestParseWSMessage_SingleTab(t *testing.T) {
	// Hypothetical 2-field message — both old and new parsing should work.
	raw := "104\t{\"t\":104}"

	parts := strings.Split(raw, "\t")
	if len(parts) < 2 {
		t.Fatal("expected 2 parts")
	}

	var msg wsMessage
	if err := json.Unmarshal([]byte(parts[len(parts)-1]), &msg); err != nil {
		t.Fatalf("json unmarshal failed: %v", err)
	}

	if msg.T != 104 {
		t.Errorf("got type %d, want 104", msg.T)
	}
}

func TestParseWSMessage_NoTab(t *testing.T) {
	raw := "notabhere"
	parts := strings.Split(raw, "\t")
	// Only 1 part — should be skipped (len < 2)
	if len(parts) >= 2 {
		t.Error("expected < 2 parts for message with no tab")
	}
}
