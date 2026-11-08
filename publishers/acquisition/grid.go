package main

import (
	"fmt"
	"time"
)

// grid is a series' grid (plugins.md, 3): the begin time of every bar of the dataset's session, a step apart from each
// session day's open while before its close (the catalog's rule, a bar in the session when its begin is in
// [open, close)), or, for a dataset without a session, every multiple of the step. Every bar of the grid, from the
// series' first, is on market.raw, a bar the source gives none for the zero K-line; a bar's index is its place in the
// grid from the series' first, so that it never moves.
type grid struct {
	step uint64   // the timeframe's length, ms
	s    *session // nil: none
}

// horizon is how far the grid is searched for a bar: a session with no day in a year has none.
const horizon = 400

// day is the bars of a session day, the first and the last begin times; ok is false on a day without one.
func (g grid) day(t time.Time) (first, last uint64, ok bool) {
	o, c, ok := g.s.hours(t)
	if !ok {
		return 0, 0, false
	}
	f, end := uint64(o.UnixMilli()), uint64(c.UnixMilli())
	n := (end - f + g.step - 1) / g.step // the begins in [open, close)
	return f, f + (n-1)*g.step, true
}

// noon is the session day of ms, at noon in the session's zone, so that a day's step crosses a change of the clock.
func (g grid) noon(ms uint64) time.Time {
	d := time.UnixMilli(int64(ms)).In(g.s.loc)
	return time.Date(d.Year(), d.Month(), d.Day(), 12, 0, 0, 0, g.s.loc)
}

// within is the grid's bars in [from, to], ascending.
func (g grid) within(from, to uint64) []uint64 {
	var out []uint64
	if from > to {
		return nil
	}
	if g.s == nil {
		for b := (from + g.step - 1) / g.step * g.step; b <= to; b += g.step {
			out = append(out, b)
		}
		return out
	}
	end := g.noon(to)
	for day := g.noon(from); !day.After(end); day = day.AddDate(0, 0, 1) {
		f, l, ok := g.day(day)
		if !ok {
			continue
		}
		for b := f; b <= l; b += g.step {
			if b >= from && b <= to {
				out = append(out, b)
			}
		}
	}
	return out
}

// after is the grid's first bar at or after t.
func (g grid) after(t uint64) (uint64, error) {
	if g.s == nil {
		return (t + g.step - 1) / g.step * g.step, nil
	}
	day := g.noon(t)
	for i := 0; i < horizon; i, day = i+1, day.AddDate(0, 0, 1) {
		f, l, ok := g.day(day)
		if !ok || l < t {
			continue
		}
		if f >= t {
			return f, nil
		}
		return f + (t-f+g.step-1)/g.step*g.step, nil
	}
	return 0, fmt.Errorf("the grid: no session day in the %d days from %s", horizon, stamp(t, g.s))
}

// before is the grid's last bar at or before t; ok is false when there is none (before the grid's first day).
func (g grid) before(t uint64) (uint64, bool) {
	if g.s == nil {
		return t / g.step * g.step, true
	}
	day := g.noon(t)
	for i := 0; i < horizon; i, day = i+1, day.AddDate(0, 0, -1) {
		f, l, ok := g.day(day)
		if !ok || f > t {
			continue
		}
		if l <= t {
			return l, true
		}
		return f + (t-f)/g.step*g.step, true
	}
	return 0, false
}

// last is the last bar of b's session day: b's session (b itself without a session).
func (g grid) last(b uint64) uint64 {
	if g.s == nil {
		return b
	}
	_, l, _ := g.day(g.noon(b))
	return l
}

// stamp is a time of the grid for a line of the report: in the session's zone, else UTC.
func stamp(ms uint64, s *session) string {
	t := time.UnixMilli(int64(ms)).UTC()
	if s != nil {
		t = t.In(s.loc)
	}
	return t.Format("2006-01-02 15:04 MST")
}
