// Package scheduler implements silent background task scheduling with
// cron-like syntax native to AetherCore, enabling proactive agent behavior
// without a user prompt.
//
// Layer 1 Capability Module — uses Go stdlib only.
package scheduler

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Standard cron field boundaries.
const (
	minMinute    = 0
	maxMinute    = 59
	minHour      = 0
	maxHour      = 23
	minDayMonth  = 1
	maxDayMonth  = 31
	minMonth     = 1
	maxMonth     = 12
	minDayWeek   = 0
	maxDayWeek   = 6
	fieldCount   = 5
	bitsPerField = 64 // uint64 bitset covers 0-63
)

// Sentinel errors for cron expression parsing.
var (
	ErrInvalidCron = errors.New("scheduler: invalid cron expression")
	ErrFieldRange  = errors.New("scheduler: field value out of range")
	ErrEmptyExpr   = errors.New("scheduler: empty cron expression")
)

// CronExpr is a parsed 5-field cron expression stored as bitsets for O(1) matching.
//
// Fields (standard cron order):
//
//	minute (0-59) | hour (0-23) | day-of-month (1-31) | month (1-12) | day-of-week (0-6, Sun=0)
//
// Supported syntax per field:
//
//   - — match all values
//     5        — exact value
//     1,15,30  — list of values
//     1-5      — inclusive range
//     */10     — step (every N from start)
//     1-30/5   — range with step
type CronExpr struct {
	Minute   uint64 // bits 0-59
	Hour     uint64 // bits 0-23
	DayMonth uint64 // bits 1-31
	Month    uint64 // bits 1-12
	DayWeek  uint64 // bits 0-6
	raw      string // original expression string for display
}

// ParseCron parses a 5-field cron expression into a CronExpr bitset.
//
// Examples:
//
//	"* * * * *"       — every minute
//	"0 * * * *"       — every hour at :00
//	"*/15 * * * *"    — every 15 minutes
//	"0 9 * * 1-5"    — 9 AM on weekdays
//	"30 2 1 * *"     — 2:30 AM on the 1st of every month
func ParseCron(expr string) (CronExpr, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return CronExpr{}, ErrEmptyExpr
	}

	// Handle common aliases
	switch expr {
	case "@yearly", "@annually":
		expr = "0 0 1 1 *"
	case "@monthly":
		expr = "0 0 1 * *"
	case "@weekly":
		expr = "0 0 * * 0"
	case "@daily", "@midnight":
		expr = "0 0 * * *"
	case "@hourly":
		expr = "0 * * * *"
	}

	fields := strings.Fields(expr)
	if len(fields) != fieldCount {
		return CronExpr{}, fmt.Errorf("%w: expected %d fields, got %d", ErrInvalidCron, fieldCount, len(fields))
	}

	minute, err := parseField(fields[0], minMinute, maxMinute)
	if err != nil {
		return CronExpr{}, fmt.Errorf("%w: minute: %w", ErrInvalidCron, err)
	}

	hour, err := parseField(fields[1], minHour, maxHour)
	if err != nil {
		return CronExpr{}, fmt.Errorf("%w: hour: %w", ErrInvalidCron, err)
	}

	dom, err := parseField(fields[2], minDayMonth, maxDayMonth)
	if err != nil {
		return CronExpr{}, fmt.Errorf("%w: day-of-month: %w", ErrInvalidCron, err)
	}

	month, err := parseField(fields[3], minMonth, maxMonth)
	if err != nil {
		return CronExpr{}, fmt.Errorf("%w: month: %w", ErrInvalidCron, err)
	}

	dow, err := parseField(fields[4], minDayWeek, maxDayWeek)
	if err != nil {
		return CronExpr{}, fmt.Errorf("%w: day-of-week: %w", ErrInvalidCron, err)
	}

	return CronExpr{
		Minute:   minute,
		Hour:     hour,
		DayMonth: dom,
		Month:    month,
		DayWeek:  dow,
		raw:      expr,
	}, nil
}

// Matches returns true if t satisfies this cron expression.
func (c CronExpr) Matches(t time.Time) bool {
	return hasBit(c.Minute, uint(t.Minute())) &&
		hasBit(c.Hour, uint(t.Hour())) &&
		hasBit(c.DayMonth, uint(t.Day())) &&
		hasBit(c.Month, uint(t.Month())) &&
		hasBit(c.DayWeek, uint(t.Weekday()))
}

// NextAfter returns the next time after t that matches this expression.
// Searches up to 366 days ahead to handle all edge cases. Returns the zero
// time if no match is found (should never happen for valid expressions).
func (c CronExpr) NextAfter(t time.Time) time.Time {
	// Start from the next minute boundary
	next := t.Truncate(time.Minute).Add(time.Minute)

	// Search up to 366 days × 24 hours × 60 minutes = 527,040 iterations max
	const maxIter = 366 * 24 * 60
	for i := range maxIter {
		_ = i
		if c.Matches(next) {
			return next
		}
		next = next.Add(time.Minute)
	}
	return time.Time{} // should be unreachable for valid expressions
}

// String returns the original cron expression.
func (c CronExpr) String() string {
	return c.raw
}

// parseField parses a single cron field (e.g., "*/5", "1-10", "1,5,10")
// and returns a uint64 bitset representing the matched values.
func parseField(field string, lo, hi int) (uint64, error) {
	var bits uint64

	parts := strings.Split(field, ",")
	for _, part := range parts {
		b, err := parsePart(part, lo, hi)
		if err != nil {
			return 0, err
		}
		bits |= b
	}
	return bits, nil
}

func parsePart(part string, lo, hi int) (uint64, error) {
	var bits uint64

	// Check for step: "*/2", "1-10/3", etc.
	rangeStr, stepStr, hasStep := strings.Cut(part, "/")

	step := 1
	if hasStep {
		s, err := strconv.Atoi(stepStr)
		if err != nil || s <= 0 {
			return 0, fmt.Errorf("%w: invalid step %q", ErrFieldRange, stepStr)
		}
		step = s
	}

	// Parse range or wildcard
	if rangeStr == "*" {
		for v := lo; v <= hi; v += step {
			bits = setBit(bits, uint(v))
		}
		return bits, nil
	}

	// Check for range: "1-5"
	startStr, endStr, isRange := strings.Cut(rangeStr, "-")
	if isRange {
		start, err := strconv.Atoi(startStr)
		if err != nil {
			return 0, fmt.Errorf("%w: invalid range start %q", ErrFieldRange, startStr)
		}
		end, err := strconv.Atoi(endStr)
		if err != nil {
			return 0, fmt.Errorf("%w: invalid range end %q", ErrFieldRange, endStr)
		}
		if start < lo || end > hi || start > end {
			return 0, fmt.Errorf("%w: range %d-%d outside [%d, %d]", ErrFieldRange, start, end, lo, hi)
		}
		for v := start; v <= end; v += step {
			bits = setBit(bits, uint(v))
		}
		return bits, nil
	}

	// Single value
	val, err := strconv.Atoi(rangeStr)
	if err != nil {
		return 0, fmt.Errorf("%w: invalid value %q", ErrFieldRange, rangeStr)
	}
	if val < lo || val > hi {
		return 0, fmt.Errorf("%w: value %d outside [%d, %d]", ErrFieldRange, val, lo, hi)
	}
	bits = setBit(bits, uint(val))
	return bits, nil
}

func setBit(bits uint64, pos uint) uint64 { return bits | (1 << pos) }
func hasBit(bits uint64, pos uint) bool   { return bits&(1<<pos) != 0 }
