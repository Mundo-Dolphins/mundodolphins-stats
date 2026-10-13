package videos

import "testing"

func TestCacheItemStatusPredicatesDefinePublicationContract(t *testing.T) {
	if !IsCandidateStatus(StatusCandidate) || !IsCandidateStatus(StatusPending) {
		t.Fatal("candidate and pending statuses should remain candidate states")
	}
	if IsCandidateStatus(StatusApproved) {
		t.Fatal("approved status should not be treated as candidate")
	}

	for _, status := range []string{StatusCandidate, StatusApproved, StatusSelected, StatusRejected, StatusNone} {
		if !IsDiscoveryCompleteStatus(status) {
			t.Fatalf("%s should be a completed discovery state", status)
		}
	}
	for _, status := range []string{StatusPending, StatusUnknown, StatusError, ""} {
		if IsDiscoveryCompleteStatus(status) {
			t.Fatalf("%q should not be a completed discovery state", status)
		}
	}

	if !IsPublishedCacheItem(&CacheItem{Status: StatusApproved, Videos: []Video{{ID: "v1"}}}) {
		t.Fatal("approved item with video should be publishable")
	}
	if IsPublishedCacheItem(&CacheItem{Status: StatusApproved}) {
		t.Fatal("approved item without videos should not be publishable")
	}
	if IsPublishedCacheItem(&CacheItem{Status: StatusCandidate, Videos: []Video{{ID: "v1"}}}) {
		t.Fatal("candidate item should be held for review")
	}
	if IsPublishedCacheItem(nil) {
		t.Fatal("nil cache item should not be publishable")
	}
}
