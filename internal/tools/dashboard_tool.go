package tools

import (
	"fmt"
	"time"

	"github.com/rakasatria/sehaty/internal/dashlink"
)

type DashboardLinkArgs struct {
	Profile string `json:"profile"`
}

type DashboardLinkOut struct {
	Profile   string `json:"profile"`
	URL       string `json:"url"`
	ExpiresAt string `json:"expires_at"`
	ValidFor  string `json:"valid_for"`
	Note      string `json:"note"`
}

// DashboardLink mints a one-hour link to one person's dashboard.
//
// The link IS the credential — there is no login — so it is minted on request, works for
// an hour, and then does not. A link left in a chat log or a screenshot is inert by the
// time anyone finds it.
func DashboardLink(d Deps, a DashboardLinkArgs) (DashboardLinkOut, error) {
	if len(d.DashKey) == 0 || d.DashBase == "" {
		return DashboardLinkOut{}, fmt.Errorf("the dashboard is not configured on this server")
	}
	p, err := requireProfile(d, a.Profile)
	if err != nil {
		return DashboardLinkOut{}, err
	}
	tok, exp := dashlink.Mint(d.DashKey, p.ID, time.Now())
	return DashboardLinkOut{
		Profile:   p.ID,
		URL:       fmt.Sprintf("%s/d/%s", d.DashBase, tok),
		ExpiresAt: exp.Format(time.RFC3339),
		ValidFor:  dashlink.TTL.String(),
		Note: "This link is the only way in and it stops working in one hour. " +
			"Anyone holding it can read this profile until then, so send it directly " +
			"to the person it belongs to and nowhere else.",
	}, nil
}
