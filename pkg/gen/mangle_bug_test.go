package gen

import (
	"fmt"
	"testing"
)

// TestMangleBug_WastedChar demonstrates that when maxLen == len(hashPrefix)+1
// (i.e., maxLen=7), the function returns only 6 characters instead of 7.
// The guard `available <= 0` fires because available = 7-6-1 = 0, and we
// return just the 6-char hashPrefix, wasting one character of budget.
//
// This is a minor inefficiency: we could either include just the separator
// (7 chars) or use a 7-char hash prefix. Instead, we return a 6-char result
// when we were allowed 7.
func TestMangleBug_WastedChar(t *testing.T) {
	longName := "com_example_very_long_service_name_that_exceeds_any_limit"

	for maxLen := 1; maxLen <= 10; maxLen++ {
		result := MangleHeadIfTooLong(longName, maxLen)
		if len(result) != maxLen {
			t.Errorf("maxLen=%d: got result %q (len=%d), want len=%d",
				maxLen, result, len(result), maxLen)
		}
	}
}

// TestMangleBug_CollisionSameTail constructs two different long names that
// share the same tail portion. If the 6-char hash prefix also collides,
// the mangled names are identical. We brute-force search for such a pair
// by varying a prefix segment while keeping the tail fixed.
//
// With 6 base36 chars (~31 bits), the birthday bound is ~2^15.5 ≈ 46k.
// We search up to 200k candidates which gives us a very high probability
// of finding a collision in the hash prefix.
func TestMangleBug_CollisionSameTail(t *testing.T) {
	const maxLen = 20
	// The tail portion that will be kept is the last (maxLen - 6 - 1) = 13 chars.
	// We fix that suffix and vary the head.
	const suffix = "_DoSomething" // 12 chars; available=13 so tail = last 13 chars

	seen := make(map[string]string) // mangled -> original

	for i := 0; i < 200_000; i++ {
		// Build a name longer than maxLen with varying prefix but same tail.
		name := fmt.Sprintf("svc_%06d%s", i, suffix)
		if len(name) <= maxLen {
			// Needs to be long enough to trigger mangling.
			name = fmt.Sprintf("some_very_long_prefix_%06d%s", i, suffix)
		}

		mangled := MangleHeadIfTooLong(name, maxLen)
		if prev, ok := seen[mangled]; ok {
			t.Errorf("COLLISION: MangleHeadIfTooLong(%q, %d) == MangleHeadIfTooLong(%q, %d) == %q",
				name, maxLen, prev, maxLen, mangled)
			return
		}
		seen[mangled] = name
	}

	t.Log("no collision found in 200k candidates (lucky, but the birthday bound says it's possible)")
}
