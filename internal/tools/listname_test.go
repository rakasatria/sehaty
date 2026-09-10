package tools

import "testing"

// An opaque id with no human name beside it is unusable in conversation.
func TestListProfilesShowsDisplayNames(t *testing.T) {
	d := testDeps(t)
	p, err := Register(d, "telegram", "8412", "Raka", pass, pass)
	if err != nil {
		t.Fatal(err)
	}
	all, err := d.DB.ListProfiles()
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, x := range all {
		if x.ID == p.ID {
			found = true
			if x.DisplayName != "Raka" {
				t.Errorf("display name = %q, want Raka", x.DisplayName)
			}
		}
	}
	if !found {
		t.Fatal("registered profile not listed")
	}
}
