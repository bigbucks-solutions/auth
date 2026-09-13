package grpc_auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"strings"
)

// MinServiceKeyLength rejects keys short enough to guess.
const MinServiceKeyLength = 32

// ServiceKeys holds the keys backend services authenticate with. Only SHA-256
// digests are kept, and a lookup compares every entry in constant time.
type ServiceKeys struct {
	entries []serviceKey
}

type serviceKey struct {
	name   string
	digest [sha256.Size]byte
}

// ParseServiceKeys reads "name=key,other=key". The name identifies the caller
// in logs; a key may itself contain "=". An empty string configures no keys.
func ParseServiceKeys(raw string) (ServiceKeys, error) {
	var keys ServiceKeys
	names := map[string]struct{}{}
	digests := map[[sha256.Size]byte]struct{}{}

	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		name, key, found := strings.Cut(entry, "=")
		name, key = strings.TrimSpace(name), strings.TrimSpace(key)
		if !found || name == "" {
			// The entry is not echoed: it may be a bare key.
			return ServiceKeys{}, errors.New("each service key entry must be written as name=key")
		}
		if len(key) < MinServiceKeyLength {
			return ServiceKeys{}, fmt.Errorf("service key for %q must be at least %d characters", name, MinServiceKeyLength)
		}
		if _, duplicate := names[name]; duplicate {
			return ServiceKeys{}, fmt.Errorf("service %q is listed more than once", name)
		}
		digest := sha256.Sum256([]byte(key))
		if _, duplicate := digests[digest]; duplicate {
			return ServiceKeys{}, fmt.Errorf("service %q reuses another service's key", name)
		}
		names[name] = struct{}{}
		digests[digest] = struct{}{}
		keys.entries = append(keys.entries, serviceKey{name: name, digest: digest})
	}
	return keys, nil
}

// Len reports how many services are configured.
func (keys ServiceKeys) Len() int { return len(keys.entries) }

// Match returns the service a presented key belongs to.
func (keys ServiceKeys) Match(presented string) (string, bool) {
	digest := sha256.Sum256([]byte(presented))
	matched := ""
	for _, entry := range keys.entries {
		if subtle.ConstantTimeCompare(digest[:], entry.digest[:]) == 1 {
			matched = entry.name
		}
	}
	return matched, matched != ""
}
