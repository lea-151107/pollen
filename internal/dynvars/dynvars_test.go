package dynvars

import (
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestExpand_Timestamp(t *testing.T) {
	before := time.Now().Unix()
	got := Expand("{{$timestamp}}")
	after := time.Now().Unix()
	n, err := strconv.ParseInt(got, 10, 64)
	if err != nil {
		t.Fatalf("expected integer epoch, got %q", got)
	}
	if n < before || n > after {
		t.Errorf("timestamp %d outside [%d, %d]", n, before, after)
	}
}

func TestExpand_TimestampMs(t *testing.T) {
	before := time.Now().UnixMilli()
	got := Expand("{{$timestamp_ms}}")
	after := time.Now().UnixMilli()
	n, err := strconv.ParseInt(got, 10, 64)
	if err != nil {
		t.Fatalf("expected integer ms epoch, got %q", got)
	}
	if n < before || n > after {
		t.Errorf("timestamp_ms %d outside [%d, %d]", n, before, after)
	}
}

func TestExpand_Datetime(t *testing.T) {
	got := Expand("{{$datetime}}")
	if _, err := time.Parse(time.RFC3339, got); err != nil {
		t.Errorf("not a valid RFC3339 timestamp: %q (%v)", got, err)
	}
}

func TestExpand_UUID(t *testing.T) {
	got := Expand("{{$uuid}}")
	// UUID v4 pattern: 8-4-4-4-12 hex, version nibble 4
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !re.MatchString(got) {
		t.Errorf("not a UUID v4: %q", got)
	}
}

func TestExpand_RandomDefault(t *testing.T) {
	got := Expand("{{$random}}")
	if _, err := strconv.ParseUint(got, 10, 32); err != nil {
		t.Errorf("expected uint32, got %q", got)
	}
}

func TestExpand_RandomMax(t *testing.T) {
	for i := 0; i < 100; i++ {
		got := Expand("{{$random:10}}")
		n, err := strconv.Atoi(got)
		if err != nil {
			t.Fatalf("expected int, got %q", got)
		}
		if n < 0 || n >= 10 {
			t.Errorf("random:10 out of [0,10): %d", n)
		}
	}
}

func TestExpand_RandomRange(t *testing.T) {
	for i := 0; i < 100; i++ {
		got := Expand("{{$random:100-200}}")
		n, err := strconv.Atoi(got)
		if err != nil {
			t.Fatalf("expected int, got %q", got)
		}
		if n < 100 || n > 200 {
			t.Errorf("random:100-200 out of [100,200]: %d", n)
		}
	}
}

func TestExpand_Base64(t *testing.T) {
	got := Expand("{{$base64:hello}}")
	if got != "aGVsbG8=" {
		t.Errorf("got %q", got)
	}
}

func TestExpand_URLEncode(t *testing.T) {
	got := Expand("{{$urlencode:a b&c=d}}")
	if got != "a+b%26c%3Dd" {
		t.Errorf("got %q", got)
	}
}

func TestExpand_UnknownLeftAsLiteral(t *testing.T) {
	// {{$payload}} is the intruder marker and must survive dynvars
	// expansion unchanged so the layers can compose.
	in := "/{{$payload}}/{{$payload1}}/{{$nonsense}}"
	got := Expand(in)
	if got != in {
		t.Errorf("expected unchanged, got %q", got)
	}
}

func TestExpand_MultipleInOneString(t *testing.T) {
	got := Expand("ts={{$timestamp}}&id={{$uuid}}")
	if !strings.Contains(got, "ts=") || !strings.Contains(got, "&id=") {
		t.Errorf("structure lost: %q", got)
	}
	// Quick sanity: each token expanded to something non-literal.
	if strings.Contains(got, "{{$timestamp}}") || strings.Contains(got, "{{$uuid}}") {
		t.Errorf("token survived as literal: %q", got)
	}
}

func TestExpand_PerCallFreshUUID(t *testing.T) {
	a := Expand("{{$uuid}}")
	b := Expand("{{$uuid}}")
	if a == b {
		t.Errorf("two successive UUIDs should differ: %q == %q", a, b)
	}
}

func TestExpand_NoTokensFastPath(t *testing.T) {
	in := "no dollar signs here, just plain text"
	if got := Expand(in); got != in {
		t.Errorf("expected unchanged, got %q", got)
	}
}

func TestExpand_RandomInvalidArgFallsBackToUint32(t *testing.T) {
	got := Expand("{{$random:abc}}")
	if _, err := strconv.ParseUint(got, 10, 32); err != nil {
		t.Errorf("invalid arg should fall back to uint32, got %q", got)
	}
}

func TestExpand_PreservesSurroundingText(t *testing.T) {
	got := Expand("prefix-{{$random:5}}-suffix")
	if !strings.HasPrefix(got, "prefix-") || !strings.HasSuffix(got, "-suffix") {
		t.Errorf("surrounding text lost: %q", got)
	}
}
