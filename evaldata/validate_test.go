package testdata

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type CorpusEntry struct {
	ID             string   `json:"id"`
	Source         string   `json:"source"`
	Text           string   `json:"text"`
	IsDecision     bool     `json:"is_decision"`
	ExpectedTopic  string   `json:"expected_topic"`
	ExpectedImpact string   `json:"expected_impact"`
	MinConfidence  float64  `json:"min_confidence"`
	Difficulty     string   `json:"difficulty"`
	Notes          string   `json:"notes"`
	AntiSignals    []string `json:"anti_signal_categories"`
}

type ClassificationEntry struct {
	ID                   string  `json:"id"`
	Decision             string  `json:"decision"`
	ExpectedTopic        string  `json:"expected_topic"`
	ConfidenceLowerBound float64 `json:"confidence_lower_bound"`
	Difficulty           string  `json:"difficulty"`
}

type ConflictEntry struct {
	ID                         string     `json:"id"`
	DecisionA                  string     `json:"decision_a"`
	DecisionB                  string     `json:"decision_b"`
	ExpectedContradictionRange [2]float64 `json:"expected_contradiction_range"`
	ExpectedType               string     `json:"expected_type"`
	Notes                      string     `json:"notes"`
}

type CrossTopicEntry struct {
	ID                     string   `json:"id"`
	Title                  string   `json:"title"`
	Decision               string   `json:"decision"`
	CandidateTopics        []string `json:"candidate_topics"`
	ExpectedIsCrossTopic   bool     `json:"expected_is_cross_topic"`
	ExpectedAffectedTopics []string `json:"expected_affected_topics"`
	Notes                  string   `json:"notes"`
}

func loadJSONL[T any](t *testing.T, path string) []T {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var entries []T
	lines := 0
	for _, line := range splitLines(string(data)) {
		if line == "" {
			continue
		}
		lines++
		var entry T
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("line %d parse error: %v", lines, err)
			continue
		}
		entries = append(entries, entry)
	}
	return entries
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func TestLoadCorpus(t *testing.T) {
	path := filepath.Join("corpus", "decisions.jsonl")
	entries := loadJSONL[CorpusEntry](t, path)

	if len(entries) < 80 {
		t.Errorf("corpus: got %d entries, want >= 80", len(entries))
	}
	t.Logf("corpus: %d entries", len(entries))

	for _, e := range entries {
		if e.ID == "" {
			t.Error("entry missing id")
		}
		if e.Text == "" {
			t.Errorf("%s: missing text", e.ID)
		}
		if e.Difficulty != "easy" && e.Difficulty != "medium" && e.Difficulty != "hard" {
			t.Errorf("%s: invalid difficulty %q", e.ID, e.Difficulty)
		}
	}

	decisions, nonDecisions := 0, 0
	easy, medium, hard := 0, 0, 0
	for _, e := range entries {
		if e.IsDecision {
			decisions++
		} else {
			nonDecisions++
		}
		switch e.Difficulty {
		case "easy":
			easy++
		case "medium":
			medium++
		case "hard":
			hard++
		}
	}
	t.Logf("decisions: %d, non-decisions: %d", decisions, nonDecisions)
	t.Logf("difficulty - easy: %d, medium: %d, hard: %d", easy, medium, hard)
}

func TestLoadClassification(t *testing.T) {
	path := filepath.Join("classification", "classification.jsonl")
	entries := loadJSONL[ClassificationEntry](t, path)

	if len(entries) < 30 {
		t.Errorf("classification: got %d entries, want >= 30", len(entries))
	}
	t.Logf("classification: %d entries", len(entries))

	for _, e := range entries {
		if e.ID == "" {
			t.Error("entry missing id")
		}
		if e.Decision == "" {
			t.Errorf("%s: missing decision", e.ID)
		}
		if e.ExpectedTopic == "" {
			t.Errorf("%s: missing expected_topic", e.ID)
		}
	}
}

func TestLoadConflicts(t *testing.T) {
	path := filepath.Join("conflicts", "conflicts.jsonl")
	entries := loadJSONL[ConflictEntry](t, path)

	if len(entries) < 15 {
		t.Errorf("conflicts: got %d entries, want >= 15", len(entries))
	}
	t.Logf("conflicts: %d pairs", len(entries))

	for _, e := range entries {
		if e.ID == "" {
			t.Error("entry missing id")
		}
		if e.DecisionA == "" || e.DecisionB == "" {
			t.Errorf("%s: missing decision content", e.ID)
		}
		lo, hi := e.ExpectedContradictionRange[0], e.ExpectedContradictionRange[1]
		if lo < 0 || hi > 1.0 || lo > hi {
			t.Errorf("%s: invalid range [%f, %f]", e.ID, lo, hi)
		}
	}
}

func TestLoadCrossTopic(t *testing.T) {
	path := filepath.Join("crosstopic", "crosstopic.jsonl")
	entries := loadJSONL[CrossTopicEntry](t, path)

	if len(entries) < 10 {
		t.Errorf("crosstopic: got %d entries, want >= 10", len(entries))
	}
	t.Logf("crosstopic: %d entries", len(entries))

	cross, single := 0, 0
	for _, e := range entries {
		if e.ID == "" {
			t.Error("entry missing id")
		}
		if e.Title == "" || e.Decision == "" {
			t.Errorf("%s: missing title or decision", e.ID)
		}
		if len(e.CandidateTopics) < 2 {
			t.Errorf("%s: need >= 2 candidate topics", e.ID)
		}
		if e.ExpectedIsCrossTopic {
			cross++
		} else {
			single++
		}
	}
	t.Logf("cross-topic: %d, single-topic: %d", cross, single)
}
