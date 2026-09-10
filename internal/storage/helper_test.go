package storage

// testKey is the Adiantum key every test opens its database with. Fixed rather than
// random so a failure is reproducible; it guards nothing, since the database it opens
// lives in a t.TempDir() that is deleted when the test ends.
const testKey = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"
