package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-contracts/market/go/market"
)

// scratch is a database of its own on the stack's ClickHouse, by QI_CLICKHOUSE_TEST_URL (the server's HTTP
// interface), dropped at the test's end; the test skips without it. It reads QI_STACK_STATE, which make check sets,
// so Go's test cache keeps its result by the server's state as well (go.toolchain.md, 4).
func scratch(t *testing.T) (serverURL, database string) {
	t.Helper()
	_ = os.Getenv("QI_STACK_STATE")
	serverURL = os.Getenv("QI_CLICKHOUSE_TEST_URL")
	if serverURL == "" {
		t.Skip("QI_CLICKHOUSE_TEST_URL is not set")
	}
	database = fmt.Sprintf("clickhouse_writer_test_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		s, err := open(context.Background(), serverURL, "default")
		if err == nil {
			_, err = s.do(context.Background(), false, "DROP DATABASE IF EXISTS `"+database+"` SYNC", nil)
		}
		if err != nil {
			t.Errorf("drop %s: %v", database, err)
		}
	})
	return serverURL, database
}

func query(t *testing.T, s *ch, q string) string {
	t.Helper()
	out, err := s.do(context.Background(), true, q+" FORMAT TabSeparated", nil)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

// dump is a table's rows read with FINAL, a line each, by key, as TabSeparated writes them, a NULL \N; of the
// corrections, their fields; of the clean, no read (0005).
func dump(t *testing.T, s *ch, table string) string {
	t.Helper()
	cols, order := columns, "instrument, timeframe, date"
	switch table {
	case "corrections":
		cols, order = columns+", corrected", order+", `read`"
	case "klines_clean":
		cols = cleanColumns
	}
	return query(t, s, "SELECT "+cols+" FROM "+table+" FINAL ORDER BY "+order)
}

func TestStore(t *testing.T) {
	serverURL, database := scratch(t)
	ctx := context.Background()
	st, err := open(ctx, serverURL, database) // the database made, then migrated
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close(ctx)
	if _, err := open(ctx, serverURL, database); err != nil { // there, and migrated: nothing to do
		t.Fatal(err)
	}
	if v := query(t, st, "SELECT count() FROM schema_migrations"); v != "5" {
		t.Fatalf("schema_migrations: %s rows", v)
	}
	// The four series: each table's columns, its engine and its key.
	k := "'instrument String','timeframe String','index Int64','date Int64','open Decimal(21, 6)','high Decimal(21, 6)'," +
		"'low Decimal(21, 6)','close Decimal(21, 6)','volume Decimal(21, 6)','vw Nullable(Decimal(21, 6))','n Nullable(Int64)'"
	for table, want := range map[string][2]string{
		"klines":           {"[" + k + ",'read Nullable(Int64)']", "instrument, timeframe, date"},
		"klines_corrected": {"[" + k + ",'read Nullable(Int64)']", "instrument, timeframe, date"},
		"klines_clean":     {"[" + k + "]", "instrument, timeframe, date"},
		"corrections":      {"[" + k + ",'read Int64','corrected Array(String)']", "instrument, timeframe, date, read"},
	} {
		if c := query(t, st, "SELECT groupArray(name || ' ' || type) FROM (SELECT name, type FROM system.columns WHERE database = currentDatabase() AND table = '"+table+"' ORDER BY position)"); c != want[0] {
			t.Fatalf("%s: %s", table, c)
		}
		if e := query(t, st, "SELECT engine, sorting_key FROM system.tables WHERE database = currentDatabase() AND name = '"+table+"'"); e != "ReplacingMergeTree\t"+want[1] {
			t.Fatalf("%s: %q", table, e)
		}
	}
	if e, err := st.Empty(ctx); err != nil || !e {
		t.Fatalf("empty: %v, %v", e, err)
	}

	spy0, spy1, qqq0 := minute{"SPY", "M1", 1764167400000}, minute{"SPY", "M1", 1764167460000}, minute{"QQQ", "M1", 1764167400000}
	rs := []row{{spy0, version{kline(t, 0, "600.1"), R1}}, {spy1, version{kline(t, 1, "600.2"), R1}},
		{qqq0, version{kline(t, 0, "600"), 0}}}
	rs[0].v.k.Vw, rs[0].v.k.Volume, rs[0].v.k.N = dec(t, "999999999.999999"), dec(t, "0.000001"), 1<<53-1
	rs[2].v.k.Vw, rs[2].v.k.HasVw, rs[2].v.k.N, rs[2].v.k.HasN = market.Decimal{}, false, 0, false // neither vw nor n
	p, err := st.Write(ctx, rows{raw: rs, corrected: rs})
	if err != nil || p != (put{raw: 3, corrected: 3}) {
		t.Fatalf("write: %+v, %v", p, err)
	}
	if e, err := st.Empty(ctx); err != nil || e {
		t.Fatalf("empty after a write: %v, %v", e, err)
	}
	h, err := st.Held(ctx, []minute{spy0, spy1, qqq0}, nil)
	if err != nil || len(h.raw) != 3 || len(h.corrected) != 3 {
		t.Fatalf("held: %+v, %v", h, err)
	}
	for _, r := range rs {
		if h.raw[r.m] != r.v || h.corrected[r.m] != r.v {
			t.Fatalf("held %+v: %+v, %+v", r.m, h.raw[r.m], h.corrected[r.m])
		}
	}
	// A correction of SPY's bar 1, read later, and its minute's corrected raw: inserted, the latter replacing.
	fixed := version{kline(t, 1, "600.25"), R2}
	c := crow{stamp{spy1, R2}, correction{fixed, []string{"close", "vw"}}}
	if p, err := st.Write(ctx, rows{corrections: []crow{c}, corrected: []row{{spy1, fixed}}}); err != nil ||
		p != (put{corrections: 1, corrected: 1}) {
		t.Fatalf("a correction: %+v, %v", p, err)
	}
	h, err = st.Held(ctx, []minute{spy1}, []stamp{c.s, {spy1, R3}})
	if err != nil || len(h.corrections) != 1 || !h.corrections[c.s].same(c.c) || h.corrected[spy1] != fixed ||
		h.raw[spy1] != rs[1].v {
		t.Fatalf("held after the correction: %+v, %v", h, err)
	}
	for table, want := range map[string]string{
		"klines": "QQQ\tM1\t0\t1764167400000\t600\t601\t599\t600\t10\t\\N\t\\N\t\\N\n" +
			"SPY\tM1\t0\t1764167400000\t600\t601\t599\t600.1\t0.000001\t999999999.999999\t9007199254740991\t1764280800000\n" +
			"SPY\tM1\t1\t1764167460000\t600\t601\t599\t600.2\t10\t600.5\t3\t1764280800000",
		"klines_corrected": "QQQ\tM1\t0\t1764167400000\t600\t601\t599\t600\t10\t\\N\t\\N\t\\N\n" +
			"SPY\tM1\t0\t1764167400000\t600\t601\t599\t600.1\t0.000001\t999999999.999999\t9007199254740991\t1764280800000\n" +
			"SPY\tM1\t1\t1764167460000\t600\t601\t599\t600.25\t10\t600.5\t3\t1764367200000",
		"corrections":  "SPY\tM1\t1\t1764167460000\t600\t601\t599\t600.25\t10\t600.5\t3\t1764367200000\t['close','vw']",
		"klines_clean": "",
	} {
		if got := dump(t, st, table); got != want {
			t.Fatalf("%s:\n%s\nwant:\n%s", table, got, want)
		}
	}

	// The clean series: written, then written again, the series emptied first; another series' kept.
	if n, err := st.Clean(ctx, "QQQ", "M1", []row{{qqq0, version{kline(t, 0, "600"), 0}}}); err != nil || n != 1 {
		t.Fatalf("clean QQQ: %d, %v", n, err)
	}
	if n, err := st.Clean(ctx, "SPY", "M1", []row{{spy0, rs[0].v}, {spy1, fixed}}); err != nil || n != 2 {
		t.Fatalf("clean SPY: %d, %v", n, err)
	}
	head := version{kline(t, 1, "600.25"), 0} // spy0 refused: spy1 the clean series' first
	head.k.Index = 0
	if n, err := st.Clean(ctx, "SPY", "M1", []row{{spy1, head}}); err != nil || n != 1 {
		t.Fatalf("clean SPY again: %d, %v", n, err)
	}
	if got, want := dump(t, st, "klines_clean"), "QQQ\tM1\t0\t1764167400000\t600\t601\t599\t600\t10\t600.5\t3\n"+
		"SPY\tM1\t0\t1764167460000\t600\t601\t599\t600.25\t10\t600.5\t3"; got != want {
		t.Fatalf("klines_clean:\n%s\nwant:\n%s", got, want)
	}
	if n, err := st.Clean(ctx, "SPY", "M1", nil); err != nil || n != 0 || !strings.HasPrefix(dump(t, st, "klines_clean"), "QQQ") ||
		strings.Contains(dump(t, st, "klines_clean"), "SPY") {
		t.Fatalf("clean SPY of none: %d, %v", n, err)
	}

	// market's widest decimals, 15 digits before the point, and 9 before it and 6 after: stored and read back as they
	// are.
	wide := version{kline(t, 3, "999999999999999"), R1}
	wide.k.Open, wide.k.High, wide.k.Low = dec(t, "999999999999999"), dec(t, "999999999999999"), dec(t, "999999999999999")
	wide.k.Volume, wide.k.Vw = dec(t, "999999999.999999"), dec(t, "-999999999999999")
	at := minute{"SPY", "M1", wide.k.Date}
	if p, err := st.Write(ctx, rows{raw: []row{{at, wide}}}); err != nil || p.raw != 1 {
		t.Fatalf("write wide: %+v, %v", p, err)
	}
	if h, err := st.Held(ctx, []minute{at}, nil); err != nil || h.raw[at] != wide {
		t.Fatalf("wide: %+v, %v", h.raw[at], err)
	}
}

// TestMigration0004: a store at 0003 holding bars migrated by 0004: not a part of klines rewritten (its parts the
// same), each row read NULL, the corrected raw its rows, the corrections and the clean empty.
func TestMigration0004(t *testing.T) {
	serverURL, database := scratch(t)
	ctx := context.Background()
	s := &ch{server: mustURL(t, serverURL), database: database, client: &http.Client{}}
	if _, err := s.do(ctx, false, "CREATE DATABASE `"+database+"`", nil); err != nil {
		t.Fatal(err)
	}
	ms, err := list()
	if err != nil || len(ms) != 5 {
		t.Fatalf("migrations: %d, %v", len(ms), err)
	}
	if _, err := s.do(ctx, true, `CREATE TABLE schema_migrations (version UInt32, name String,
		applied_at DateTime64(3, 'UTC') DEFAULT now64(3)) ENGINE = MergeTree ORDER BY version`, nil); err != nil {
		t.Fatal(err)
	}
	for _, m := range ms[:3] {
		for _, q := range statements(m.sql) {
			if _, err := s.do(ctx, true, q, nil); err != nil {
				t.Fatalf("%s: %v", m.name, err)
			}
		}
		if _, err := s.do(ctx, true, "INSERT INTO schema_migrations (version, name) VALUES ("+strconv.Itoa(int(m.version))+
			", '"+m.name+"')", nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.do(ctx, true, `INSERT INTO klines VALUES ('SPY', 'M1', 0, 1764167400000, 600, 601, 599, 600.1, 10, 600.5, 3),
		('SPY', 'M1', 1, 1764167460000, 600, 601, 599, 600.2, 10, NULL, NULL), ('QQQ', 'M1', 0, 1764167400000, 600, 601, 599, 600, 10, 600.5, 3)`, nil); err != nil {
		t.Fatal(err)
	}
	parts := "SELECT groupArray(name) FROM (SELECT name FROM system.parts WHERE database = currentDatabase() AND table = 'klines' AND active ORDER BY name)"
	before := query(t, s, parts)
	st, err := open(ctx, serverURL, database)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close(ctx)
	if v := query(t, st, "SELECT count() FROM schema_migrations"); v != "5" {
		t.Fatalf("schema_migrations: %s", v)
	}
	if after := query(t, st, parts); after != before {
		t.Fatalf("klines' parts: %s, before %s", after, before)
	}
	raw := dump(t, st, "klines")
	if !strings.Contains(raw, "SPY\tM1\t1\t1764167460000\t600\t601\t599\t600.2\t10\t\\N\t\\N\t\\N") ||
		strings.Count(raw, "\n") != 2 {
		t.Fatalf("klines: %s", raw)
	}
	if c := dump(t, st, "klines_corrected"); c != raw {
		t.Fatalf("the corrected raw:\n%s\nthe raw:\n%s", c, raw)
	}
	for _, table := range []string{"corrections", "klines_clean"} {
		if d := dump(t, st, table); d != "" {
			t.Fatalf("%s: %s", table, d)
		}
	}
}

// TestMigration0005: a store at 0004 holding rows in each series but the clean, which is empty, migrated by 0005: not a
// part of the other three rewritten (their parts the same), the clean without read; run again, nothing changed.
func TestMigration0005(t *testing.T) {
	serverURL, database := scratch(t)
	ctx := context.Background()
	s := &ch{server: mustURL(t, serverURL), database: database, client: &http.Client{}}
	if _, err := s.do(ctx, false, "CREATE DATABASE `"+database+"`", nil); err != nil {
		t.Fatal(err)
	}
	ms, err := list()
	if err != nil || len(ms) != 5 || ms[4].name != "0005_clean.sql" {
		t.Fatalf("migrations: %d, %v", len(ms), err)
	}
	if _, err := s.do(ctx, true, `CREATE TABLE schema_migrations (version UInt32, name String,
		applied_at DateTime64(3, 'UTC') DEFAULT now64(3)) ENGINE = MergeTree ORDER BY version`, nil); err != nil {
		t.Fatal(err)
	}
	for _, m := range ms[:4] {
		for _, q := range statements(m.sql) {
			if _, err := s.do(ctx, true, q, nil); err != nil {
				t.Fatalf("%s: %v", m.name, err)
			}
		}
		if _, err := s.do(ctx, true, "INSERT INTO schema_migrations (version, name) VALUES ("+strconv.Itoa(int(m.version))+
			", '"+m.name+"')", nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, q := range []string{
		`INSERT INTO klines VALUES ('SPY', 'M1', 0, 1764167400000, 600, 601, 599, 600.1, 10, 600.5, 3, 1764280800000)`,
		`INSERT INTO klines_corrected VALUES ('SPY', 'M1', 0, 1764167400000, 600, 601, 599, 600.2, 10, 600.5, 3, 1764367200000)`,
		`INSERT INTO corrections VALUES ('SPY', 'M1', 0, 1764167400000, 600, 601, 599, 600.2, 10, 600.5, 3, 1764367200000, ['close'])`,
	} {
		if _, err := s.do(ctx, true, q, nil); err != nil {
			t.Fatal(err)
		}
	}
	parts := "SELECT groupArray(table || '/' || name) FROM (SELECT table, name FROM system.parts WHERE database = " +
		"currentDatabase() AND table IN ('klines', 'klines_corrected', 'corrections') AND active ORDER BY table, name)"
	before := query(t, s, parts)
	st, err := open(ctx, serverURL, database)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close(ctx)
	if v := query(t, st, "SELECT max(version) FROM schema_migrations"); v != "5" {
		t.Fatalf("schema_migrations: %s", v)
	}
	if after := query(t, st, parts); after != before {
		t.Fatalf("parts: %s, before %s", after, before)
	}
	if c := query(t, st, "SELECT groupArray(name) FROM (SELECT name FROM system.columns WHERE database = currentDatabase() AND table = 'klines_clean' ORDER BY position)"); c != "['instrument','timeframe','index','date','open','high','low','close','volume','vw','n']" {
		t.Fatalf("klines_clean: %s", c)
	}
	for _, q := range statements(ms[4].sql) { // again: nothing to drop
		if _, err := s.do(ctx, true, q, nil); err != nil {
			t.Fatalf("0005 again: %v", err)
		}
	}
}

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}
