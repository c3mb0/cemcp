package clearthought

// Session represents a collection of thoughts with related context that lives
// only in memory. Mental models and debugging sessions are kept separate from
// the trimming logic.
type Session struct {
	Thoughts      []string `json:"thoughts"`
	MentalModels  []string `json:"mental_models"`
	DebugSessions []string `json:"debug_sessions"`
}

// TrimStats reports how many thoughts were removed and how many remain after
// trimming a session.
type TrimStats struct {
	Removed   int `json:"removed"`
	Remaining int `json:"remaining"`
}

// TrimSession keeps only the most recent keepLast thoughts. Mental models and
// debugging sessions are left untouched. A negative keepLast results in all
// thoughts being removed. The returned TrimStats include counts of removed and
// remaining thoughts.
func TrimSession(s *Session, keepLast int) TrimStats {
	if s == nil {
		return TrimStats{}
	}
	if keepLast < 0 {
		keepLast = 0
	}
	total := len(s.Thoughts)
	removed := 0
	if total > keepLast {
		removed = total - keepLast
		s.Thoughts = append([]string(nil), s.Thoughts[total-keepLast:]...)
	}
	return TrimStats{Removed: removed, Remaining: len(s.Thoughts)}
}
