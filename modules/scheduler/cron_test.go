package scheduler

import (
	"testing"
	"time"
)

// ── ParseCron tests ─────────────────────────────────────────────────────

func TestParseCronEveryMinute(t *testing.T) {
	c, err := ParseCron("* * * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Every minute of every hour of every day should match
	now := time.Date(2026, 3, 9, 14, 30, 0, 0, time.UTC)
	if !c.Matches(now) {
		t.Errorf("expected * * * * * to match %v", now)
	}
}

func TestParseCronExactMinute(t *testing.T) {
	c, err := ParseCron("30 * * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	yes := time.Date(2026, 1, 1, 0, 30, 0, 0, time.UTC)
	no := time.Date(2026, 1, 1, 0, 15, 0, 0, time.UTC)
	if !c.Matches(yes) {
		t.Errorf("expected match at minute 30")
	}
	if c.Matches(no) {
		t.Errorf("expected no match at minute 15")
	}
}

func TestParseCronStep(t *testing.T) {
	c, err := ParseCron("*/15 * * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, m := range []int{0, 15, 30, 45} {
		tm := time.Date(2026, 1, 1, 12, m, 0, 0, time.UTC)
		if !c.Matches(tm) {
			t.Errorf("expected */15 to match minute %d", m)
		}
	}
	for _, m := range []int{1, 14, 16, 29, 31, 44, 59} {
		tm := time.Date(2026, 1, 1, 12, m, 0, 0, time.UTC)
		if c.Matches(tm) {
			t.Errorf("expected */15 NOT to match minute %d", m)
		}
	}
}

func TestParseCronRange(t *testing.T) {
	c, err := ParseCron("0 9-17 * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Hour 9 should match
	yes := time.Date(2026, 6, 15, 9, 0, 0, 0, time.UTC)
	if !c.Matches(yes) {
		t.Error("expected 0 9-17 to match 09:00")
	}
	// Hour 17 should match
	yes2 := time.Date(2026, 6, 15, 17, 0, 0, 0, time.UTC)
	if !c.Matches(yes2) {
		t.Error("expected 0 9-17 to match 17:00")
	}
	// Hour 8 should not match
	no := time.Date(2026, 6, 15, 8, 0, 0, 0, time.UTC)
	if c.Matches(no) {
		t.Error("expected 0 9-17 NOT to match 08:00")
	}
}

func TestParseCronList(t *testing.T) {
	c, err := ParseCron("0,30 * * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	yes0 := time.Date(2026, 1, 1, 5, 0, 0, 0, time.UTC)
	yes30 := time.Date(2026, 1, 1, 5, 30, 0, 0, time.UTC)
	no15 := time.Date(2026, 1, 1, 5, 15, 0, 0, time.UTC)
	if !c.Matches(yes0) {
		t.Error("expected 0,30 to match minute 0")
	}
	if !c.Matches(yes30) {
		t.Error("expected 0,30 to match minute 30")
	}
	if c.Matches(no15) {
		t.Error("expected 0,30 NOT to match minute 15")
	}
}

func TestParseCronWeekday(t *testing.T) {
	// 0 9 * * 1-5 → 9 AM on Mon-Fri
	c, err := ParseCron("0 9 * * 1-5")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 2026-03-09 is Monday
	mon := time.Date(2026, 3, 9, 9, 0, 0, 0, time.UTC)
	if !c.Matches(mon) {
		t.Errorf("expected 1-5 to match Monday (%v, weekday=%d)", mon, mon.Weekday())
	}
	// 2026-03-08 is Sunday
	sun := time.Date(2026, 3, 8, 9, 0, 0, 0, time.UTC)
	if c.Matches(sun) {
		t.Errorf("expected 1-5 NOT to match Sunday (%v, weekday=%d)", sun, sun.Weekday())
	}
}

func TestParseCronRangeWithStep(t *testing.T) {
	c, err := ParseCron("0-30/10 * * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, m := range []int{0, 10, 20, 30} {
		tm := time.Date(2026, 1, 1, 0, m, 0, 0, time.UTC)
		if !c.Matches(tm) {
			t.Errorf("expected 0-30/10 to match minute %d", m)
		}
	}
	for _, m := range []int{5, 15, 25, 35, 40} {
		tm := time.Date(2026, 1, 1, 0, m, 0, 0, time.UTC)
		if c.Matches(tm) {
			t.Errorf("expected 0-30/10 NOT to match minute %d", m)
		}
	}
}

func TestParseCronAliases(t *testing.T) {
	tests := []struct {
		alias  string
		expect string
	}{
		{"@hourly", "0 * * * *"},
		{"@daily", "0 0 * * *"},
		{"@midnight", "0 0 * * *"},
		{"@weekly", "0 0 * * 0"},
		{"@monthly", "0 0 1 * *"},
		{"@yearly", "0 0 1 1 *"},
		{"@annually", "0 0 1 1 *"},
	}
	for _, tt := range tests {
		c, err := ParseCron(tt.alias)
		if err != nil {
			t.Errorf("ParseCron(%q) error: %v", tt.alias, err)
			continue
		}
		expected, err := ParseCron(tt.expect)
		if err != nil {
			t.Fatalf("ParseCron(%q) error: %v", tt.expect, err)
		}
		if c.Minute != expected.Minute || c.Hour != expected.Hour ||
			c.DayMonth != expected.DayMonth || c.Month != expected.Month ||
			c.DayWeek != expected.DayWeek {
			t.Errorf("alias %q != %q bitsets", tt.alias, tt.expect)
		}
	}
}

func TestParseCronInvalid(t *testing.T) {
	invalids := []string{
		"",
		"* * *",
		"* * * * * *",
		"60 * * * *",
		"* 24 * * *",
		"* * 0 * *",
		"* * 32 * *",
		"* * * 13 *",
		"* * * 0 *",
		"* * * * 7",
		"abc * * * *",
		"*/0 * * * *",
	}
	for _, expr := range invalids {
		_, err := ParseCron(expr)
		if err == nil {
			t.Errorf("expected error for %q, got nil", expr)
		}
	}
}

func TestCronExprString(t *testing.T) {
	expr := "*/5 9-17 * * 1-5"
	c, err := ParseCron(expr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.String() != expr {
		t.Errorf("String() = %q, want %q", c.String(), expr)
	}
}

func TestCronNextAfter(t *testing.T) {
	c, err := ParseCron("0 * * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// If it's 14:30, next should be 15:00
	base := time.Date(2026, 3, 9, 14, 30, 0, 0, time.UTC)
	next := c.NextAfter(base)
	expected := time.Date(2026, 3, 9, 15, 0, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("NextAfter(%v) = %v, want %v", base, next, expected)
	}
}

func TestCronNextAfterExact(t *testing.T) {
	c, err := ParseCron("30 14 * * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// If it's exactly 14:30, next should be tomorrow 14:30
	base := time.Date(2026, 3, 9, 14, 30, 0, 0, time.UTC)
	next := c.NextAfter(base)
	expected := time.Date(2026, 3, 10, 14, 30, 0, 0, time.UTC)
	if !next.Equal(expected) {
		t.Errorf("NextAfter(%v) = %v, want %v", base, next, expected)
	}
}

func TestCronMonthlyFirstDay(t *testing.T) {
	c, err := ParseCron("0 0 1 * *")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Jan 1 midnight should match
	jan1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if !c.Matches(jan1) {
		t.Error("expected 0 0 1 * * to match Jan 1 midnight")
	}
	// Jan 2 should not match
	jan2 := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	if c.Matches(jan2) {
		t.Error("expected 0 0 1 * * NOT to match Jan 2")
	}
}

// ── Benchmark ───────────────────────────────────────────────────────────

func BenchmarkCronMatches(b *testing.B) {
	c, _ := ParseCron("*/5 9-17 * * 1-5")
	tm := time.Date(2026, 3, 10, 10, 15, 0, 0, time.UTC) // Monday 10:15
	b.ResetTimer()
	for range b.N {
		c.Matches(tm)
	}
}

func BenchmarkParseCron(b *testing.B) {
	for range b.N {
		_, _ = ParseCron("*/15 9-17 1,15 1-6 1-5")
	}
}
