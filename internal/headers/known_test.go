package headers

import "testing"

func TestSuggest_EmptyReturnsNil(t *testing.T) {
	if got := Suggest(""); got != nil {
		t.Errorf("empty prefix should return nil, got %v", got)
	}
}

func TestSuggest_PrefixCaseInsensitive(t *testing.T) {
	got := Suggest("cont")
	if len(got) == 0 {
		t.Fatal("expected matches for 'cont'")
	}
	for _, h := range got {
		if h != "Content-Disposition" && h != "Content-Encoding" &&
			h != "Content-Language" && h != "Content-Length" &&
			h != "Content-Location" && h != "Content-Range" &&
			h != "Content-Security-Policy" && h != "Content-Type" {
			t.Errorf("unexpected match: %q", h)
		}
	}
}

func TestSuggest_UnknownPrefix(t *testing.T) {
	if got := Suggest("Zzz"); got != nil {
		t.Errorf("unknown prefix should return nil, got %v", got)
	}
}

func TestSuggest_FullMatchStillReturned(t *testing.T) {
	got := Suggest("Authorization")
	if len(got) != 1 || got[0] != "Authorization" {
		t.Errorf("want [Authorization], got %v", got)
	}
}
