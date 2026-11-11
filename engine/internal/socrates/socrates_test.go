package socrates_test

import (
	"testing"

	"elpanteon/engine/internal/galileo/contract"
	"elpanteon/engine/internal/socrates"
	"elpanteon/engine/internal/testutil"
)

func TestReviewAcceptsSolidCase(t *testing.T) {
	report := socrates.Review(testutil.Case())
	if !report.Passed {
		t.Fatalf("Passed = false, score = %d, findings = %+v", report.Score, report.Findings)
	}
	if report.Score != 100 {
		t.Fatalf("score = %d, want 100", report.Score)
	}
}

func TestReviewRejectsHypeCase(t *testing.T) {
	engineeringCase := testutil.Case()
	engineeringCase.HypeFilter.Verdict = contract.VerdictReject
	engineeringCase.HypeFilter.Score = 10
	engineeringCase.Solution.Actions = nil
	engineeringCase.Solution.Verification = ""

	report := socrates.Review(engineeringCase)
	if report.Passed {
		t.Fatalf("Passed = true, score = %d", report.Score)
	}
	if len(report.Findings) == 0 {
		t.Fatal("debía producir hallazgos")
	}
}
