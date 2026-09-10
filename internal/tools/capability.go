package tools

import (
	"github.com/rakasatria/sehaty/internal/capability"
	"github.com/rakasatria/sehaty/internal/storage"
)

// Have answers what is on file about someone, for the capability registry.
//
// This is the ONLY place that reads the weight log to answer that question. The log is
// the difference between a fact and an answer: a weight is measured, not volunteered, so
// it cannot come from the answered set and must not be inferred from the profile.
func Have(d Deps, p storage.Profile) capability.Have {
	weighed := false
	if d.DB != nil {
		if ws, err := d.DB.Weights(p.ID, 3650); err == nil && len(ws) > 0 {
			weighed = true
		}
	}
	return p.Have(weighed)
}
