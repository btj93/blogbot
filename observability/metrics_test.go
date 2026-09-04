package observability

import (
	"testing"
)

func TestExpvarCounters(t *testing.T) {
	before := ScrapeRunsTotal.Value()
	ScrapeRunsTotal.Add(1)
	after := ScrapeRunsTotal.Value()

	if after != before+1 {
		t.Errorf("ScrapeRunsTotal after Add(1) = %d, want %d", after, before+1)
	}
}
