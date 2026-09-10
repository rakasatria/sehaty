package tools

import (
	"io"
	"net/http"
)

// readAll is a small helper used by tests to confirm the middleware restores the body.
func readAll(r *http.Request) ([]byte, error) { return io.ReadAll(r.Body) }
