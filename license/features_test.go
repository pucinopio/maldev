package license

import (
	"testing"
	"time"
)

func TestFeaturesRoundTrip(t *testing.T) {
	data, pub, _ := issueFor(t, IssueOptions{
		NotAfter: time.Now().Add(time.Hour),
		Features: []string{"export", "api", "advanced-recon"},
	})
	v, err := Verify(data, trustedFor(pub, "k1"))
	if err != nil {
		t.Fatal(err)
	}
	if !v.HasFeature("export") || !v.HasFeature("api") {
		t.Fatalf("expected features set, got %v", v.Features)
	}
	if v.HasFeature("missing") {
		t.Fatal("HasFeature returned true for absent name")
	}
}

func TestHasFeatureEmpty(t *testing.T) {
	data, pub, _ := issueFor(t, IssueOptions{NotAfter: time.Now().Add(time.Hour)})
	v, _ := Verify(data, trustedFor(pub, "k1"))
	if v.HasFeature("anything") {
		t.Fatal("empty feature list should return false")
	}
}
