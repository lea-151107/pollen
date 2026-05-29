package intruder

import (
	"strings"
	"testing"
)

func collectVectors(t *testing.T, it VectorIterator) [][]string {
	t.Helper()
	var out [][]string
	for {
		v, ok := it.Next()
		if !ok {
			break
		}
		out = append(out, v)
	}
	return out
}

func TestSniperVector_OneListYieldsSingletonVectors(t *testing.T) {
	it, err := NewVectorIterator(Sniper, []PayloadConfig{
		{Kind: PayloadList, Words: []string{"a", "b", "c"}},
	})
	if err != nil {
		t.Fatalf("NewVectorIterator: %v", err)
	}
	got := collectVectors(t, it)
	want := [][]string{{"a"}, {"b"}, {"c"}}
	if !equalVecs(got, want) {
		t.Errorf("sniper: got %v, want %v", got, want)
	}
}

func TestNewVectorIterator_SniperRejectsMultipleConfigs(t *testing.T) {
	_, err := NewVectorIterator(Sniper, []PayloadConfig{
		{Kind: PayloadList, Words: []string{"a"}},
		{Kind: PayloadList, Words: []string{"b"}},
	})
	if err == nil {
		t.Errorf("sniper with 2 configs should error")
	}
}

func TestPitchfork_TwoListsZip(t *testing.T) {
	it, err := NewVectorIterator(Pitchfork, []PayloadConfig{
		{Kind: PayloadList, Words: []string{"a", "b"}},
		{Kind: PayloadList, Words: []string{"x", "y"}},
	})
	if err != nil {
		t.Fatalf("NewVectorIterator: %v", err)
	}
	got := collectVectors(t, it)
	want := [][]string{{"a", "x"}, {"b", "y"}}
	if !equalVecs(got, want) {
		t.Errorf("pitchfork zip: got %v, want %v", got, want)
	}
}

func TestPitchfork_StopsAtShortestList(t *testing.T) {
	it, err := NewVectorIterator(Pitchfork, []PayloadConfig{
		{Kind: PayloadList, Words: []string{"a", "b", "c"}},
		{Kind: PayloadList, Words: []string{"x"}},
	})
	if err != nil {
		t.Fatalf("NewVectorIterator: %v", err)
	}
	got := collectVectors(t, it)
	want := [][]string{{"a", "x"}}
	if !equalVecs(got, want) {
		t.Errorf("pitchfork shortest: got %v, want %v", got, want)
	}
}

func TestPitchfork_RejectsSingleConfig(t *testing.T) {
	_, err := NewVectorIterator(Pitchfork, []PayloadConfig{
		{Kind: PayloadList, Words: []string{"a"}},
	})
	if err == nil {
		t.Errorf("pitchfork with 1 list should error")
	}
}

func TestClusterBomb_TwoListsProduct(t *testing.T) {
	it, err := NewVectorIterator(ClusterBomb, []PayloadConfig{
		{Kind: PayloadList, Words: []string{"a", "b"}},
		{Kind: PayloadList, Words: []string{"x", "y"}},
	})
	if err != nil {
		t.Fatalf("NewVectorIterator: %v", err)
	}
	got := collectVectors(t, it)
	// Rightmost odometer position increments first.
	want := [][]string{
		{"a", "x"}, {"a", "y"},
		{"b", "x"}, {"b", "y"},
	}
	if !equalVecs(got, want) {
		t.Errorf("cluster bomb product: got %v, want %v", got, want)
	}
}

func TestClusterBomb_ThreeListsOdometer(t *testing.T) {
	it, err := NewVectorIterator(ClusterBomb, []PayloadConfig{
		{Kind: PayloadList, Words: []string{"a", "b"}},
		{Kind: PayloadList, Words: []string{"1", "2"}},
		{Kind: PayloadList, Words: []string{"x", "y"}},
	})
	if err != nil {
		t.Fatalf("NewVectorIterator: %v", err)
	}
	got := collectVectors(t, it)
	if len(got) != 8 {
		t.Fatalf("expected 2*2*2=8 vectors, got %d", len(got))
	}
	// Verify the rightmost-first odometer order.
	want := [][]string{
		{"a", "1", "x"}, {"a", "1", "y"},
		{"a", "2", "x"}, {"a", "2", "y"},
		{"b", "1", "x"}, {"b", "1", "y"},
		{"b", "2", "x"}, {"b", "2", "y"},
	}
	if !equalVecs(got, want) {
		t.Errorf("odometer order: got %v, want %v", got, want)
	}
}

func TestClusterBomb_RejectsSingleConfig(t *testing.T) {
	_, err := NewVectorIterator(ClusterBomb, []PayloadConfig{
		{Kind: PayloadList, Words: []string{"a"}},
	})
	if err == nil {
		t.Errorf("cluster bomb with 1 list should error")
	}
}

func TestNewVectorIterator_UnknownMode(t *testing.T) {
	_, err := NewVectorIterator(AttackMode(99), []PayloadConfig{
		{Kind: PayloadList, Words: []string{"a"}},
	})
	if err == nil {
		t.Errorf("unknown mode should error")
	}
}

func TestNewVectorIterator_PitchforkPropagatesPayloadConfigError(t *testing.T) {
	// Range with From > To is rejected by NewIterator; that error should
	// bubble up through NewVectorIterator with a clear position number.
	_, err := NewVectorIterator(Pitchfork, []PayloadConfig{
		{Kind: PayloadList, Words: []string{"a"}},
		{Kind: PayloadRange, From: 5, To: 1},
	})
	if err == nil {
		t.Fatalf("expected error for invalid range in position 2")
	}
	if !strings.Contains(err.Error(), "payload 2") {
		t.Errorf("error should name position 2; got %v", err)
	}
}

func equalVecs(a, b [][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for j := range a[i] {
			if a[i][j] != b[i][j] {
				return false
			}
		}
	}
	return true
}
