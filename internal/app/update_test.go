package app

import "testing"

func TestIsTextEditingFocus_Response(t *testing.T) {
	// focusResponse is treated as text-editing only when the jq filter or
	// in-body search bar is actively capturing keystrokes (responseInputActive=true).
	// Otherwise letters like `u` (undo) and `s` (save response) must remain
	// available as global shortcuts.
	if got := isTextEditingFocus(focusResponse, false, false, false, false); got {
		t.Errorf("focusResponse with no input active should NOT be text-editing, got %v", got)
	}
	if got := isTextEditingFocus(focusResponse, false, false, false, true); !got {
		t.Errorf("focusResponse with input active SHOULD be text-editing, got %v", got)
	}
}

func TestIsTextEditingFocus_Preserved(t *testing.T) {
	// Ensure adding the responseInputActive arg didn't shift the meaning of
	// other focus values.
	cases := []struct {
		name string
		f    focusArea
		args [4]bool // bodyInEditor, historyFilter, collFilter, responseInput
		want bool
	}{
		{"URL always editing", focusURL, [4]bool{false, false, false, false}, true},
		{"Query always editing", focusQuery, [4]bool{false, false, false, false}, true},
		{"Auth always editing", focusAuth, [4]bool{false, false, false, false}, true},
		{"Headers always editing", focusHeaders, [4]bool{false, false, false, false}, true},
		{"Body not editing without editor", focusBody, [4]bool{false, false, false, false}, false},
		{"Body editing with editor", focusBody, [4]bool{true, false, false, false}, true},
		{"History not editing without filter", focusHistory, [4]bool{false, false, false, false}, false},
		{"History editing with filter", focusHistory, [4]bool{false, true, false, false}, true},
		{"Collections not editing without filter", focusCollections, [4]bool{false, false, false, false}, false},
		{"Collections editing with filter", focusCollections, [4]bool{false, false, true, false}, true},
		{"Method never editing", focusMethod, [4]bool{false, false, false, false}, false},
	}
	for _, c := range cases {
		got := isTextEditingFocus(c.f, c.args[0], c.args[1], c.args[2], c.args[3])
		if got != c.want {
			t.Errorf("%s: got %v want %v", c.name, got, c.want)
		}
	}
}
