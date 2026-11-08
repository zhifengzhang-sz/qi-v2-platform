package main

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

// line is a K-line's canonical text at minute i of a day: open, high, low, close and volume as given, Massive's vw and
// n beside them, which the validation reads not.
func line(i int, open, high, low, cls, volume string) string {
	date := 1764167400000 + uint64(i)*60_000
	return `{"close":` + cls + `,"date":` + itoa(date) + `,"high":` + high + `,"index":` + itoa(uint64(i)) +
		`,"low":` + low + `,"n":3,"open":` + open + `,"volume":` + volume + `,"vw":600.25}`
}

func itoa(n uint64) string { return strconv.FormatUint(n, 10) }

// answers runs the protocol over the lines and gives its lines out, the first its name and version.
func answers(t *testing.T, after *uint64, lines ...string) []string {
	t.Helper()
	var out bytes.Buffer
	in := strings.Join(lines, "\n")
	if len(lines) > 0 {
		in += "\n"
	}
	if err := clean(strings.NewReader(in), &out, after); err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSuffix(out.String(), "\n"), "\n")
	if got[0] != `{"cleaning":"validation","version":"0.1.0"}` {
		t.Fatalf("the first line %q, not the cleaning's name and version", got[0])
	}
	if len(got)-1 != len(lines) {
		t.Fatalf("%d answers to %d lines: %q", len(got)-1, len(lines), got)
	}
	return got[1:]
}

// TestRules: each rule, a K-line breaking it refused with its reason; one keeping every rule given as it came.
func TestRules(t *testing.T) {
	good := line(0, "600", "601", "599", "600.5", "10")
	for _, c := range []struct{ name, line, reason string }{
		{"every rule kept", good, ""},
		{"a price 0", line(0, "0", "601", "599", "600.5", "10"), "open 0: a price not above 0"},
		{"a price below 0", line(0, "600", "601", "-1", "600.5", "10"), "low -1: a price not above 0"},
		{"a price not finite", line(0, "600", "1e400", "599", "600.5", "10"),
			"high 1e400: a price not a finite decimal of market's"},
		{"a price past market's precision", line(0, "600", "601", "599", "600.0000001", "10"),
			"close 600.0000001: a price not a finite decimal of market's"},
		{"low above min(open, close)", line(0, "600", "601", "600.25", "600.5", "10"),
			"low 600.25 above min(open, close) 600"},
		{"low above min(open, close), the close", line(0, "600.5", "601", "600.25", "600", "10"),
			"low 600.25 above min(open, close) 600"},
		{"max(open, close) above high", line(0, "600", "600.25", "599", "600.5", "10"),
			"max(open, close) 600.5 above high 600.25"},
		{"volume below 0", line(0, "600", "601", "599", "600.5", "-10"), "volume -10 below 0"},
		{"volume not finite", line(0, "600", "601", "599", "600.5", "1e400"),
			"volume 1e400: not a finite decimal of market's"},
		{"volume 0", line(0, "600", "601", "599", "600.5", "0"), ""},
		{"the prices one", line(0, "600", "600", "600", "600", "1"), ""},
	} {
		got := answers(t, nil, c.line)[0]
		want := `{"kline":` + c.line + `}`
		if c.reason != "" {
			want = `{"refused":"` + c.reason + `"}`
		}
		if got != want {
			t.Errorf("%s: %s, want %s", c.name, got, want)
		}
	}
}

// TestZero: the zero K-line, a minute without a bar, refused: its prices 0.
func TestZero(t *testing.T) {
	zero := `{"close":0,"date":1764167460000,"high":0,"index":1,"low":0,"open":0,"volume":0}`
	got := answers(t, nil, line(0, "600", "601", "599", "600.5", "10"), zero, line(2, "600", "601", "599", "600.5", "10"))
	want := []string{`{"kline":` + line(0, "600", "601", "599", "600.5", "10") + `}`,
		`{"refused":"open 0: a price not above 0"}`, `{"kline":` + line(2, "600", "601", "599", "600.5", "10") + `}`}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("%q, want %q", got, want)
	}
}

// TestDates: a date not after the last accepted refused, a refused K-line's date not the last; --after the last before.
func TestDates(t *testing.T) {
	a, b, c := line(0, "600", "601", "599", "600.5", "10"), line(1, "600", "601", "599", "600.5", "10"),
		line(2, "600", "601", "599", "600.5", "10")
	bad := line(2, "0", "601", "599", "600.5", "10")
	got := answers(t, nil, a, a, b, bad, c, b)
	want := []string{`{"kline":` + a + `}`, `{"refused":"date 1764167400000 not after 1764167400000, the last accepted"}`,
		`{"kline":` + b + `}`, `{"refused":"open 0: a price not above 0"}`, `{"kline":` + c + `}`,
		`{"refused":"date 1764167460000 not after 1764167520000, the last accepted"}`}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("%q, want %q", got, want)
	}
	after := uint64(1764167460000) // b's: the last the clean topic holds
	got = answers(t, &after, b, c)
	want = []string{`{"refused":"date 1764167460000 not after 1764167460000, the last accepted"}`, `{"kline":` + c + `}`}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("--after: %q, want %q", got, want)
	}
	if got := answers(t, nil); len(got) != 0 {
		t.Fatalf("no input: %q", got)
	}
}

// TestFault: a line not a K-line's JSON object stops it.
func TestFault(t *testing.T) {
	for name, l := range map[string]string{
		"not JSON":         "SPY 600",
		"no date":          `{"close":1,"high":1,"index":0,"low":1,"open":1,"volume":1}`,
		"no volume":        `{"close":1,"date":1,"high":1,"index":0,"low":1,"open":1}`,
		"a date of text":   `{"close":1,"date":"1","high":1,"index":0,"low":1,"open":1,"volume":1}`,
		"an index below 0": `{"close":1,"date":1,"high":1,"index":-1,"low":1,"open":1,"volume":1}`,
	} {
		var out bytes.Buffer
		if err := clean(strings.NewReader(l+"\n"), &out, nil); err == nil || !strings.Contains(err.Error(), "line 1") {
			t.Errorf("%s: %v", name, err)
		}
	}
}
