package wizard

import "github.com/google/uuid"

// parseUUID parses a UUID string, returning a descriptive error on failure.
func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
