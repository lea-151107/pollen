package intruder

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/lea-151107/pollen/internal/history"
)

// markerRE matches {{$payload}} (position 1, legacy) and {{$payloadK}}
// for K >= 1. The numeric capture is empty for the legacy form and
// otherwise the position index as written.
var markerRE = regexp.MustCompile(`\{\{\$payload(\d*)\}\}`)

// ApplyPayloads returns req with every {{$payload}}, {{$payload1}}, ...
// {{$payloadN}} marker replaced according to payloads. {{$payload}} and
// {{$payload1}} both map to payloads[0]; {{$payloadK}} (K>=2) maps to
// payloads[K-1]. Markers referencing a position out of range are left
// untouched (the caller should have validated with PositionsUsed first).
// The input req is not mutated; Headers are deep-copied.
func ApplyPayloads(req history.Request, payloads []string) history.Request {
	apply := func(s string) string {
		return markerRE.ReplaceAllStringFunc(s, func(match string) string {
			sub := markerRE.FindStringSubmatch(match)
			pos := 1
			if sub[1] != "" {
				n, err := strconv.Atoi(sub[1])
				if err != nil || n < 1 {
					return match
				}
				pos = n
			}
			if pos > len(payloads) {
				return match
			}
			return payloads[pos-1]
		})
	}
	out := req
	out.URL = apply(req.URL)
	out.Body = apply(req.Body)
	if len(req.Headers) > 0 {
		out.Headers = make([]history.Header, len(req.Headers))
		for i, h := range req.Headers {
			out.Headers[i] = history.Header{
				Key:   apply(h.Key),
				Value: apply(h.Value),
			}
		}
	}
	return out
}

// PositionsUsed scans every text-bearing field of req and returns the
// set of marker positions referenced, as a sorted slice. {{$payload}}
// and {{$payload1}} both count as position 1. The returned slice is
// useful both for validation and for letting the UI hint at the number
// of payload lists the user needs to provide.
func PositionsUsed(req history.Request) []int {
	seen := map[int]struct{}{}
	scan := func(s string) {
		for _, m := range markerRE.FindAllStringSubmatch(s, -1) {
			pos := 1
			if m[1] != "" {
				n, err := strconv.Atoi(m[1])
				if err != nil || n < 1 {
					continue
				}
				pos = n
			}
			seen[pos] = struct{}{}
		}
	}
	scan(req.URL)
	scan(req.Body)
	for _, h := range req.Headers {
		scan(h.Key)
		scan(h.Value)
	}
	out := make([]int, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	// Bubble sort is fine — positions are bounded by the small N.
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[i] > out[j] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// HasMarkers validates that req references the marker positions the
// runner needs for mode with nPositions payload lists. Returns nil on
// success; an explanatory error otherwise.
//
//   - Sniper: nPositions must be 1, and the template must reference
//     position 1 ({{$payload}} or {{$payload1}}) at least once.
//   - Pitchfork / ClusterBomb: nPositions must be >= 2, and the
//     template must reference positions 1..nPositions; gaps are
//     rejected so the user doesn't silently get default payloads for
//     missing positions.
func HasMarkers(req history.Request, mode AttackMode, nPositions int) error {
	used := PositionsUsed(req)
	switch mode {
	case Sniper:
		if nPositions != 1 {
			return fmt.Errorf("sniper: expected 1 payload list, got %d", nPositions)
		}
		for _, p := range used {
			if p == 1 {
				return nil
			}
		}
		return fmt.Errorf("sniper: request template has no %s or {{$payload1}} marker", PayloadMarker)
	case Pitchfork, ClusterBomb:
		if nPositions < 2 {
			return fmt.Errorf("%s: requires at least 2 payload lists, got %d", modeName(mode), nPositions)
		}
		need := map[int]bool{}
		for k := 1; k <= nPositions; k++ {
			need[k] = true
		}
		for _, p := range used {
			delete(need, p)
		}
		if len(need) > 0 {
			missing := []int{}
			for k := range need {
				missing = append(missing, k)
			}
			for i := 0; i < len(missing); i++ {
				for j := i + 1; j < len(missing); j++ {
					if missing[i] > missing[j] {
						missing[i], missing[j] = missing[j], missing[i]
					}
				}
			}
			parts := []string{}
			for _, k := range missing {
				parts = append(parts, fmt.Sprintf("{{$payload%d}}", k))
			}
			return fmt.Errorf("%s: missing marker(s): %s", modeName(mode), strings.Join(parts, ", "))
		}
		return nil
	}
	return fmt.Errorf("unknown attack mode: %d", mode)
}

func modeName(m AttackMode) string {
	switch m {
	case Sniper:
		return "sniper"
	case Pitchfork:
		return "pitchfork"
	case ClusterBomb:
		return "cluster bomb"
	}
	return "unknown"
}
