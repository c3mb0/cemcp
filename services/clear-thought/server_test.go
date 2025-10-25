package clearthought

import "testing"

func TestTrimSession(t *testing.T) {
	s := &Session{
		Thoughts:      []string{"a", "b", "c", "d"},
		MentalModels:  []string{"m1"},
		DebugSessions: []string{"d1"},
	}

	stats := TrimSession(s, 2)
	if stats.Removed != 2 {
		t.Fatalf("expected 2 removed, got %d", stats.Removed)
	}
	if stats.Remaining != 2 {
		t.Fatalf("expected 2 remaining, got %d", stats.Remaining)
	}
	if len(s.Thoughts) != 2 || s.Thoughts[0] != "c" || s.Thoughts[1] != "d" {
		t.Fatalf("thoughts not trimmed correctly: %v", s.Thoughts)
	}
	if len(s.MentalModels) != 1 || s.MentalModels[0] != "m1" {
		t.Fatalf("mental models were modified: %v", s.MentalModels)
	}
	if len(s.DebugSessions) != 1 || s.DebugSessions[0] != "d1" {
		t.Fatalf("debug sessions were modified: %v", s.DebugSessions)
	}
}
