package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
	_ "time/tzdata" // a session's zone resolves without the host's zoneinfo

	"gopkg.in/yaml.v3"
)

// A dataset of the catalog (config/catalog.yaml; docs/impl/engineering/plugins.md, 3), what acquisition reads of
// it: its source, its plugin's command, its series, its history, its session, how far back each run asks again
// (reread), how old a bar must be before it is first read (delay), its cleaning plugin's command (cleaning) and how
// many damaged minutes a clean topic keeps before it is made again (tolerance). Any other key of an entry is not
// read here.
type dataset struct {
	Name        string   `yaml:"name"`
	Source      string   `yaml:"source"`
	Command     []string `yaml:"command"`
	Instruments []string `yaml:"instruments"`
	Timeframes  []string `yaml:"timeframes"`
	History     string   `yaml:"history"`
	Session     *session `yaml:"session"`
	Reread      string   `yaml:"reread"`
	Delay       string   `yaml:"delay"`
	Cleaning    []string `yaml:"cleaning"`
	Tolerance   *int     `yaml:"tolerance"`

	reread, delay uint64 // Reread and Delay, milliseconds
}

// loadDataset reads the catalog at path and gives its dataset name: its command's and its cleaning's programs, when a
// relative path, resolved against the catalog's folder; its session compiled; its reread and delay, each a duration as
// the catalog writes one (30d, 20h), held in milliseconds; its tolerance, a number of minutes from 0; each required.
func loadDataset(path, name string) (dataset, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return dataset{}, fmt.Errorf("catalog: %w", err)
	}
	var c struct {
		Datasets []dataset `yaml:"datasets"`
	}
	if err := yaml.Unmarshal(b, &c); err != nil {
		return dataset{}, fmt.Errorf("catalog %s: %w", path, err)
	}
	for _, d := range c.Datasets {
		if d.Name != name {
			continue
		}
		if len(d.Command) == 0 || d.Command[0] == "" {
			return dataset{}, fmt.Errorf("catalog %s: %s: no command, the dataset's source plugin", path, name)
		}
		if len(d.Cleaning) == 0 || d.Cleaning[0] == "" {
			return dataset{}, fmt.Errorf("catalog %s: %s: no cleaning, the dataset's cleaning plugin", path, name)
		}
		if d.Tolerance == nil || *d.Tolerance < 0 {
			return dataset{}, fmt.Errorf("catalog %s: %s: no tolerance, the damaged minutes a clean topic keeps "+
				"before it is made again, from 0", path, name)
		}
		for _, c := range [][]string{d.Command, d.Cleaning} {
			if p := c[0]; !filepath.IsAbs(p) && filepath.Base(p) != p {
				c[0] = filepath.Join(filepath.Dir(path), p)
			}
		}
		if len(d.Instruments) == 0 || len(d.Timeframes) == 0 {
			return dataset{}, fmt.Errorf("catalog %s: %s: needs instruments and timeframes", path, name)
		}
		for _, tf := range d.Timeframes {
			if steps[tf] == 0 {
				return dataset{}, fmt.Errorf("catalog %s: %s: timeframe %q: not the catalog's (M1, M5, M15, M30, H1, H4, D1, W1)", path, name, tf)
			}
		}
		if _, err := d.from(time.Now()); err != nil {
			return dataset{}, fmt.Errorf("catalog %s: %s: %w", path, name, err)
		}
		for _, f := range []struct {
			key, v, what string
			ms           *uint64
		}{{"reread", d.Reread, "how far back each run asks again", &d.reread},
			{"delay", d.Delay, "how old a bar must be before it is first read", &d.delay}} {
			if f.v == "" {
				return dataset{}, fmt.Errorf("catalog %s: %s: no %s: %s (30d, 20h)", path, name, f.key, f.what)
			}
			dur, err := duration(f.v)
			if err != nil {
				return dataset{}, fmt.Errorf("catalog %s: %s: %s: %w", path, name, f.key, err)
			}
			*f.ms = uint64(dur.Milliseconds())
		}
		if d.Session != nil {
			if err := d.Session.compile(); err != nil {
				return dataset{}, fmt.Errorf("catalog %s: %s: %w", path, name, err)
			}
		}
		return d, nil
	}
	return dataset{}, fmt.Errorf("catalog %s: no dataset %s", path, name)
}

// steps is each timeframe of the catalog and the length of its bar: a series' next bar begins a step after its last,
// and a bar is closed a step after its begin.
var steps = map[string]time.Duration{
	"M1": time.Minute, "M5": 5 * time.Minute, "M15": 15 * time.Minute, "M30": 30 * time.Minute,
	"H1": time.Hour, "H4": 4 * time.Hour, "D1": 24 * time.Hour, "W1": 7 * 24 * time.Hour,
}

var (
	durationRe = regexp.MustCompile(`^(\d+)([mhdy])$`)
	dateRe     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

// from is the history's first moment: a duration behind now (15m, 36h, 90d, 2y, a year 365 days) or a date (UTC),
// as the catalog has it.
func (d dataset) from(now time.Time) (time.Time, error) {
	if dateRe.MatchString(d.History) {
		t, err := time.Parse(time.DateOnly, d.History)
		if err != nil {
			return time.Time{}, fmt.Errorf("history %q: %w", d.History, err)
		}
		return t, nil
	}
	dur, err := duration(d.History)
	if err != nil {
		return time.Time{}, fmt.Errorf("history %q: want 15m, 36h, 90d, 2y or a date", d.History)
	}
	return now.Add(-dur), nil
}

// duration is a duration as the catalog writes one: a number and its unit, m, h, d or y (a year 365 days).
func duration(s string) (time.Duration, error) {
	m := durationRe.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("%q: want a number and m, h, d or y (15m, 36h, 90d, 2y)", s)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n > 1000*365*24*60 {
		return 0, fmt.Errorf("%q: too long", s)
	}
	unit := map[string]time.Duration{"m": time.Minute, "h": time.Hour, "d": 24 * time.Hour, "y": 365 * 24 * time.Hour}[m[2]]
	return time.Duration(n) * unit, nil
}

// session is a dataset's trading session, the catalog's rule: a bar is in it when its begin time is in [open, close)
// of a session day in the zone, Monday to Friday, except the closed dates, an early date closing at its own time.
type session struct {
	Zone   string            `yaml:"zone"`
	Open   string            `yaml:"open"`
	Close  string            `yaml:"close"`
	Closed []string          `yaml:"closed"`
	Early  map[string]string `yaml:"early"`

	loc         *time.Location
	open, close int // minutes after midnight
	closed      map[string]bool
	early       map[string]int
}

func clock(s string) (int, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, fmt.Errorf("time %q: want HH:MM", s)
	}
	return t.Hour()*60 + t.Minute(), nil
}

func (s *session) compile() error {
	loc, err := time.LoadLocation(s.Zone)
	if err != nil || s.Zone == "" || s.Zone == "Local" {
		return fmt.Errorf("session: zone %q: want an IANA name", s.Zone)
	}
	o, err := clock(s.Open)
	if err != nil {
		return fmt.Errorf("session: open: %w", err)
	}
	c, err := clock(s.Close)
	if err != nil {
		return fmt.Errorf("session: close: %w", err)
	}
	if o >= c {
		return fmt.Errorf("session: open %s not before close %s", s.Open, s.Close)
	}
	closed := map[string]bool{}
	for _, d := range s.Closed {
		if _, err := time.Parse(time.DateOnly, d); err != nil {
			return fmt.Errorf("session: closed %q: want YYYY-MM-DD", d)
		}
		closed[d] = true
	}
	early := map[string]int{}
	for d, at := range s.Early {
		if _, err := time.Parse(time.DateOnly, d); err != nil {
			return fmt.Errorf("session: early %q: want YYYY-MM-DD", d)
		}
		e, err := clock(at)
		if err != nil {
			return fmt.Errorf("session: early %s: %w", d, err)
		}
		if e <= o || e >= c || closed[d] {
			return fmt.Errorf("session: early %s at %s: not inside the session, or the day closed", d, at)
		}
		early[d] = e
	}
	s.loc, s.open, s.close, s.closed, s.early = loc, o, c, closed, early
	return nil
}

// hours is a day's session in the zone, [open, close) as times; ok is false on a day without one.
func (s *session) hours(day time.Time) (open, close time.Time, ok bool) {
	if wd := day.Weekday(); wd == time.Saturday || wd == time.Sunday {
		return open, close, false
	}
	key := day.Format(time.DateOnly)
	if s.closed[key] {
		return open, close, false
	}
	c := s.close
	if e, ok := s.early[key]; ok {
		c = e
	}
	y, m, d := day.Date()
	at := func(min int) time.Time { return time.Date(y, m, d, min/60, min%60, 0, 0, s.loc) }
	return at(s.open), at(c), true
}

// contains reports whether a bar beginning at ms (milliseconds since 1970) is in the session.
func (s *session) contains(ms uint64) bool {
	t := time.UnixMilli(int64(ms)).In(s.loc)
	o, c, ok := s.hours(t)
	return ok && !t.Before(o) && t.Before(c)
}

// spans is the session in [from, to], ms: each session day's [open, close), as the closed span of begin times
// [open, close − 1 ms], cut to [from, to]; ascending.
func (s *session) spans(from, to uint64) [][2]uint64 {
	var out [][2]uint64
	noon := func(ms uint64) time.Time {
		d := time.UnixMilli(int64(ms)).In(s.loc)
		return time.Date(d.Year(), d.Month(), d.Day(), 12, 0, 0, 0, s.loc)
	}
	end := noon(to)
	for day := noon(from); !day.After(end); day = day.AddDate(0, 0, 1) {
		o, c, ok := s.hours(day)
		if !ok {
			continue
		}
		f, t := max(uint64(o.UnixMilli()), from), min(uint64(c.UnixMilli())-1, to)
		if f <= t {
			out = append(out, [2]uint64{f, t})
		}
	}
	return out
}
