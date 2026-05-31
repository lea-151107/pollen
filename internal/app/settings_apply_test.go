package app

import (
	"testing"
	"time"

	"github.com/lea-151107/pollen/internal/collections"
	"github.com/lea-151107/pollen/internal/env"
	"github.com/lea-151107/pollen/internal/history"
	"github.com/lea-151107/pollen/internal/httpx"
	"github.com/lea-151107/pollen/internal/settings"
	"github.com/lea-151107/pollen/internal/ui"
	"github.com/lea-151107/pollen/internal/userconfig"
)

func newApplyTestModel(t *testing.T) Model {
	t.Helper()
	userconfig.SetOverride(t.TempDir())
	t.Cleanup(func() { userconfig.SetOverride("") })

	store, _ := history.Open()
	collStore, _ := collections.Open()
	return New(store, collStore, env.New(), Options{})
}

func TestApplySettings_PropagatesRuntimeGlobals(t *testing.T) {
	m := newApplyTestModel(t)
	s := &settings.Settings{
		SkipTLSVerify:              true,
		RequestTimeoutSecs:         120,
		MaxResponseMiB:             64,
		HistoryLimit:               500,
		TextPreviewKiB:             200,
		SidebarMaxWidth:            60,
		HexDumpKiB:                 8,
		ProxyURL:                   "http://proxy.example.com:3128",
		DisableRedirects:           true,
		IntruderConcurrency:        10,
		IntruderDelayMs:            50,
		IntruderMaxRequests:        5000,
		IntruderResponseBodyCapKiB: 128,
		OAuthPersistTokens:         false,
		ResponsePanelRatio:         0.6,
	}
	m.applySettings(s)

	if !httpx.SkipTLSVerify.Load() {
		t.Errorf("SkipTLSVerify should be applied")
	}
	if httpx.RequestTimeout != 120*time.Second {
		t.Errorf("RequestTimeout = %v, want 120s", httpx.RequestTimeout)
	}
	if httpx.MaxResponseBytes != 64*1024*1024 {
		t.Errorf("MaxResponseBytes = %d, want %d", httpx.MaxResponseBytes, 64*1024*1024)
	}
	if httpx.ProxyURL != "http://proxy.example.com:3128" {
		t.Errorf("ProxyURL = %q", httpx.ProxyURL)
	}
	if !httpx.DisableRedirects {
		t.Errorf("DisableRedirects should be true")
	}
	if ui.TextPreviewLimit != 200*1024 {
		t.Errorf("TextPreviewLimit = %d", ui.TextPreviewLimit)
	}
	if ui.DefaultHexDumpLimit != 8*1024 {
		t.Errorf("DefaultHexDumpLimit = %d", ui.DefaultHexDumpLimit)
	}
	if m.sidebarMaxWidth != 60 {
		t.Errorf("sidebarMaxWidth = %d", m.sidebarMaxWidth)
	}
	if m.responsePanelRatio != 0.6 {
		t.Errorf("responsePanelRatio = %v", m.responsePanelRatio)
	}
	if m.intruderBodyCapBytes != 128*1024 {
		t.Errorf("intruderBodyCapBytes = %d", m.intruderBodyCapBytes)
	}
	if m.persistTokens != false {
		t.Errorf("persistTokens = %v", m.persistTokens)
	}
	if !m.tlsInsecure {
		t.Errorf("tlsInsecure mirror should reflect SkipTLSVerify")
	}
}

func TestApplySettings_SavesToDisk(t *testing.T) {
	m := newApplyTestModel(t)
	s := &settings.Settings{
		SkipTLSVerify:      true,
		RequestTimeoutSecs: 90,
		OAuthPersistTokens: true,
	}
	m.applySettings(s)
	reloaded, err := settings.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reloaded.SkipTLSVerify {
		t.Errorf("applySettings should persist to disk")
	}
	if reloaded.RequestTimeoutSecs != 90 {
		t.Errorf("RequestTimeoutSecs not persisted, got %d", reloaded.RequestTimeoutSecs)
	}
}
