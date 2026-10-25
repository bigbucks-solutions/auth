package auth

import (
	sessionstore "bigbucks/solution/auth/session_store"
	"bigbucks/solution/auth/settings"
	"time"

	"github.com/oklog/ulid/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Session limit", func() {
	It("evicts the oldest session when sign-ins land in the same second", func() {
		store := sessionstore.NewSessionStore(settings.Current)
		userID := "eviction-" + ulid.Make().String()
		DeferCleanup(func() { _ = store.RevokeAllUserSessions(userID, "") })

		// One more than the cap, as fast as they come: all within one second.
		ids := make([]string, 0, sessionstore.MaxSessionsPerUser+1)
		for range sessionstore.MaxSessionsPerUser + 1 {
			id, err := store.CreateSession(userID, "eviction", "test", "127.0.0.1", time.Hour)
			Expect(err).NotTo(HaveOccurred())
			ids = append(ids, id)
		}

		// An evicted session reads as not found.
		valid, _, _ := store.ValidateSession(ids[0])
		Expect(valid).To(BeFalse(), "the first session is the oldest and is the one evicted")
		for _, id := range ids[1:] {
			valid, _, err := store.ValidateSession(id)
			Expect(err).NotTo(HaveOccurred())
			Expect(valid).To(BeTrue(), "a newer session survives")
		}
		Expect(store.GetUserSessionCount(userID)).To(BeEquivalentTo(sessionstore.MaxSessionsPerUser))
	})
})
