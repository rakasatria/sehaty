package telegram

import "github.com/rakasatria/sehaty/internal/storage"

// profileFixture is a minimal profile for copy tests that do not touch a database.
func profileFixture() storage.Profile {
	return storage.Profile{ID: "abc", DisplayName: "Raka", Goal: "fat_loss"}
}
