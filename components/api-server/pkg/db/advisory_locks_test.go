package db

import "testing"

// The retention job's lock must never share a key with a per-event lock or
// the migrations lock. The second advisory lock key is hash(lockType), so the
// lock types must hash to different values.
func TestEventRetentionLockTypeDoesNotCollide(t *testing.T) {
	retention := hash(string(EventRetention))
	for _, other := range []LockType{Events, Migrations} {
		if hash(string(other)) == retention {
			t.Fatalf("lock type %q hashes to the same key as %q", other, EventRetention)
		}
	}
}
