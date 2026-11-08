package main

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"time"
	_ "time/tzdata" // a zone resolves without the host's zoneinfo

	"gopkg.in/yaml.v3"
)

// series is a series of the core's topics, its key's three names: <source>:<instrument>:<timeframe>.
type series struct{ key, source, instrument, timeframe string }

// parseSeries reads a series' key: three names, none empty, parted by colons (a name holds none, core/README.md).
func parseSeries(key string) (series, error) {
	n := strings.Split(key, ":")
	if len(n) != 3 || n[0] == "" || n[1] == "" || n[2] == "" {
		return series{}, fmt.Errorf("series %q: its key, <source>:<instrument>:<timeframe>, as massive:SPY:M1", key)
	}
	return series{key: key, source: n[0], instrument: n[1], timeframe: n[2]}, nil
}

// catalog is what query reads of the catalog (config/catalog.yaml): each dataset's source, instruments, timeframes and
// session's zone; the rest is acquisition's.
type catalog struct {
	path     string
	datasets []dataset
}

type dataset struct {
	Name        string   `yaml:"name"`
	Source      string   `yaml:"source"`
	Instruments []string `yaml:"instruments"`
	Timeframes  []string `yaml:"timeframes"`
	Session     *struct {
		Zone string `yaml:"zone"`
	} `yaml:"session"`
}

func loadCatalog(path string) (*catalog, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("catalog: %w", err)
	}
	var c struct {
		Datasets []dataset `yaml:"datasets"`
	}
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("catalog %s: %w", path, err)
	}
	return &catalog{path: path, datasets: c.Datasets}, nil
}

// zone is the series' zone: the session's zone of the catalog's dataset of its source, instrument and timeframe, UTC
// where none names one. Two datasets of the series naming two zones are refused.
func (c *catalog) zone(s series) (*time.Location, error) {
	name, from := "", ""
	for _, d := range c.datasets {
		if d.Source != s.source || !slices.Contains(d.Instruments, s.instrument) ||
			!slices.Contains(d.Timeframes, s.timeframe) || d.Session == nil || d.Session.Zone == "" {
			continue
		}
		if name != "" && d.Session.Zone != name {
			return nil, fmt.Errorf("catalog %s: %s in %s, zone %s, and in %s, zone %s", c.path, s.key, from, name,
				d.Name, d.Session.Zone)
		}
		name, from = d.Session.Zone, d.Name
	}
	if name == "" {
		return time.UTC, nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, fmt.Errorf("catalog %s: %s: zone %q: %w", c.path, from, name, err)
	}
	return loc, nil
}

// The forms of a minute: minuteLayout, T as given, in a zone or with an offset or Z (minuteForm); readLayout, a read,
// to the millisecond, in UTC.
const (
	minuteLayout = "2006-01-02T15:04"
	minuteForm   = "2006-01-02T15:04Z07:00"
	readLayout   = "2006-01-02T15:04:05.000Z07:00"
)

// parseMinute reads T: a minute with an offset or Z (2026-11-04T14:30Z, 2026-11-04T09:30-05:00), or without one, a
// minute in the zone (2026-11-04T09:30), refused when the zone's clock passes it twice or never (a change of its
// offset), which an offset names.
func parseMinute(v string, loc *time.Location) (time.Time, error) {
	if t, err := time.Parse(minuteForm, v); err == nil {
		return t, nil
	}
	wall, err := time.Parse(minuteLayout, v)
	if err != nil {
		return time.Time{}, fmt.Errorf("minute %q: 2026-11-04T09:30 in the series' zone, or with an offset or Z "+
			"(2026-11-04T14:30Z)", v)
	}
	var at []time.Time
	for _, probe := range []time.Time{wall.Add(-24 * time.Hour), wall, wall.Add(24 * time.Hour)} {
		_, off := probe.In(loc).Zone()
		t := wall.Add(-time.Duration(off) * time.Second)
		if t.In(loc).Format(minuteLayout) == v && !slices.ContainsFunc(at, t.Equal) {
			at = append(at, t)
		}
	}
	switch len(at) {
	case 0:
		return time.Time{}, fmt.Errorf("minute %q: not a time of %s, whose clock passes it by; give an offset", v, loc)
	case 1:
		return at[0], nil
	}
	return time.Time{}, fmt.Errorf("minute %q: twice a time of %s (%s and %s); give an offset", v, loc,
		at[0].In(loc).Format(minuteForm), at[1].In(loc).Format(minuteForm))
}

// minute is a bar's date (milliseconds since 1970 UTC) in a zone, in the form T takes.
func minute(date uint64, loc *time.Location) string {
	return time.UnixMilli(int64(date)).In(loc).Format(minuteForm)
}

// readTime is a version's read in UTC to the millisecond; "-" for none (0.1.0 and 0.2.0 carry none).
func readTime(read uint64) string {
	if read == 0 {
		return "-"
	}
	return time.UnixMilli(int64(read)).UTC().Format(readLayout)
}
