package showroom

import (
	"fmt"
	"testing"
)

func TestNextLiveChanged(t *testing.T) {
	var (
		oldEpoch int64 = 1700000000
		newEpoch int64 = 1700001000
	)

	oldText := "2024-01-01 20:00"

	changed := isNextLiveChanged(&oldEpoch, &oldText, &newEpoch, oldText)
	if !changed {
		t.Error("expected changed=true when epoch differs")
	}
}

func TestNextLiveUnchanged(t *testing.T) {
	var epoch int64 = 1700000000

	text := "2024-01-01 20:00"

	changed := isNextLiveChanged(&epoch, &text, &epoch, text)
	if changed {
		t.Error("expected changed=false when both match")
	}
}

func TestNextLiveTBDSuppressed(t *testing.T) {
	var oldEpoch int64 = 1700000000

	oldText := "2024-01-01 20:00"

	// Values differ but new text is TBD — changed is true but caller checks TBD
	changed := isNextLiveChanged(&oldEpoch, &oldText, nil, "TBD")
	if !changed {
		t.Error("expected changed=true (values differ)")
	}
}

func TestNextLiveNilToValue(t *testing.T) {
	var newEpoch int64 = 1700000000

	newText := "2024-01-01 20:00"

	changed := isNextLiveChanged(nil, nil, &newEpoch, newText)
	if !changed {
		t.Error("expected changed=true when going from nil to value")
	}
}

func TestIsNextLiveChanged_BothNil(t *testing.T) {
	changed := isNextLiveChanged(nil, nil, nil, "")
	if changed {
		t.Error("expected changed=false when all values are nil/empty")
	}
}

func TestIsNextLiveChanged_TextOnlyChange(t *testing.T) {
	var epoch int64 = 1700000000

	oldText := "2024-01-01 20:00"

	changed := isNextLiveChanged(&epoch, &oldText, &epoch, "2024-01-01 21:00")
	if !changed {
		t.Error("expected changed=true when text differs")
	}
}

func TestParseChatID(t *testing.T) {
	tests := []struct {
		input string
		want  int64
	}{
		{"12345", 12345},
		{"-100123", -100123},
		{"0", 0},
		{"invalid", 0},
		{"", 0},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("input=%q", tt.input), func(t *testing.T) {
			got := parseChatID(tt.input)
			if got != tt.want {
				t.Errorf("parseChatID(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
