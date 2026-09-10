package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The ratings that matter for SAFETY. If any of these regress, a beginner gets shown a
// movement that can hurt them — which is the entire reason difficulty filtering exists.
func TestEliteMovementsAreNeverBeginnerFriendly(t *testing.T) {
	for _, name := range []string{
		"single leg squat (pistol)",
		"muscle up",
		"barbell snatch",
		"clean and jerk",
		"handstand push-up",
		"front lever raise",
		"dragon flag",
		"turkish get-up",
		"barbell overhead squat",
	} {
		if got := rate(name, "body weight"); got != 5 {
			t.Errorf("rate(%q) = %d, want 5 — a beginner could be prescribed this", name, got)
		}
	}
}

func TestEverydayMovementsAreAccessible(t *testing.T) {
	cases := []struct {
		name, equipment string
		wantMax         int
	}{
		{"push-up", "body weight", 2},
		{"standing hamstring stretch", "body weight", 1},
		// 2, not 1: assistance makes a dip accessible, but it still needs a machine
		// and some pressing strength. Rating it alongside a hamstring stretch
		// would be the opposite error.
		{"assisted chest dip (kneeling)", "assisted", 2},
		{"cable seated row", "cable", 2},
		{"lever leg extension", "leverage machine", 1},
		{"stationary bike walk", "stationary bike", 1},
		{"barbell curl", "barbell", 2}, // isolation pulls a free-weight base down
	}
	for _, tc := range cases {
		if got := rate(tc.name, tc.equipment); got > tc.wantMax {
			t.Errorf("rate(%q, %q) = %d, want <= %d — too restrictive, a normal person "+
				"should be able to do this", tc.name, tc.equipment, got, tc.wantMax)
		}
	}
}

// "one arm" alone must not promote an ordinary isolation lift to elite. This was a real
// risk in the phrasing: a one-arm dumbbell curl is not a one-arm pull-up.
func TestOneArmIsolationIsNotTreatedAsElite(t *testing.T) {
	if got := rate("dumbbell one arm curl", "dumbbell"); got >= 5 {
		t.Errorf("rate(one arm curl) = %d — 'one arm' is matching too broadly", got)
	}
	if got := rate("one arm pull-up", "body weight"); got != 5 {
		t.Errorf("rate(one arm pull-up) = %d, want 5", got)
	}
}

func TestEveryRatingIsInRange(t *testing.T) {
	raw, err := os.ReadFile("../../internal/catalog/difficulty.json")
	if err != nil {
		t.Skipf("difficulty.json not generated: %v", err)
	}
	var m map[string]int
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if len(m) < 1000 {
		t.Fatalf("only %d ratings — the generator did not run against the full dataset", len(m))
	}
	for name, d := range m {
		if d < 1 || d > 5 {
			t.Errorf("%q rated %d, outside 1..5", name, d)
		}
		if name != strings.ToLower(name) {
			t.Errorf("%q is not lowercased — the catalog looks up by lowercase name and "+
				"would treat this as unrated", name)
		}
	}
}

// A beginner (max_difficulty 3) must end up with a usable number of exercises. This is the
// regression that started all of this: the profile could see exactly ONE.
func TestABeginnerHasRealChoice(t *testing.T) {
	raw, err := os.ReadFile("../../internal/catalog/difficulty.json")
	if err != nil {
		t.Skip("difficulty.json not generated")
	}
	var m map[string]int
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	var usable int
	for _, d := range m {
		if d <= 3 {
			usable++
		}
	}
	if usable < 500 {
		t.Fatalf("only %d exercises at difficulty <= 3; a beginner needs real choice", usable)
	}
	t.Logf("%d of %d exercises available at difficulty <= 3", usable, len(m))
}
