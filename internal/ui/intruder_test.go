package ui

import (
	"strings"
	"testing"

	"github.com/lea-151107/pollen/internal/intruder"
)

func TestParsePayloadInput_Range(t *testing.T) {
	cfg, err := parsePayloadInput(intruder.PayloadRange, "1-100")
	if err != "" {
		t.Fatalf("err: %s", err)
	}
	if cfg.From != 1 || cfg.To != 100 || cfg.Step != 1 {
		t.Errorf("range: %+v", cfg)
	}
}

func TestParsePayloadInput_RangeWithStep(t *testing.T) {
	cfg, err := parsePayloadInput(intruder.PayloadRange, "1-100/5")
	if err != "" {
		t.Fatalf("err: %s", err)
	}
	if cfg.From != 1 || cfg.To != 100 || cfg.Step != 5 {
		t.Errorf("range: %+v", cfg)
	}
}

func TestParsePayloadInput_RangeInvalid(t *testing.T) {
	if _, err := parsePayloadInput(intruder.PayloadRange, "abc"); err == "" {
		t.Errorf("expected error for non-range input")
	}
	if _, err := parsePayloadInput(intruder.PayloadRange, "1-100/0"); err == "" {
		t.Errorf("expected error for step < 1")
	}
}

func TestParsePayloadInput_List(t *testing.T) {
	cfg, err := parsePayloadInput(intruder.PayloadList, "a, b ,c")
	if err != "" {
		t.Fatalf("err: %s", err)
	}
	if strings.Join(cfg.Words, "|") != "a|b|c" {
		t.Errorf("list words: %v", cfg.Words)
	}
}

func TestParsePayloadInput_BruteValid(t *testing.T) {
	cfg, err := parsePayloadInput(intruder.PayloadBrute, "abc 1-3")
	if err != "" {
		t.Fatalf("err: %s", err)
	}
	if cfg.Alphabet != "abc" || cfg.MinLen != 1 || cfg.MaxLen != 3 {
		t.Errorf("brute: %+v", cfg)
	}
}

func TestParsePayloadInput_BruteMalformed(t *testing.T) {
	if _, err := parsePayloadInput(intruder.PayloadBrute, "abc"); err == "" {
		t.Errorf("expected error for missing length range")
	}
}

func TestParsePayloadInput_CaseToggle(t *testing.T) {
	cfg, err := parsePayloadInput(intruder.PayloadCaseToggle, "admin")
	if err != "" {
		t.Fatalf("err: %s", err)
	}
	if cfg.Base != "admin" {
		t.Errorf("base: %q", cfg.Base)
	}
}

func TestParsePositiveInt(t *testing.T) {
	if n, err := parsePositiveInt("5", "x"); err != "" || n != 5 {
		t.Errorf("5: n=%d err=%s", n, err)
	}
	if _, err := parsePositiveInt("0", "x"); err == "" {
		t.Errorf("expected error for 0")
	}
	if _, err := parsePositiveInt("abc", "x"); err == "" {
		t.Errorf("expected error for non-int")
	}
}

func TestNewIntruder_DefaultsPopulated(t *testing.T) {
	in := NewIntruder(5, 0, 1000)
	if in.concInput.Value() != "5" {
		t.Errorf("concurrency default: %q", in.concInput.Value())
	}
	if in.delayInput.Value() != "0" {
		t.Errorf("delay default: %q", in.delayInput.Value())
	}
	if in.maxInput.Value() != "1000" {
		t.Errorf("max default: %q", in.maxInput.Value())
	}
	if in.State() != IntruderHidden {
		t.Errorf("initial state: %v", in.State())
	}
}
