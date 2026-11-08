package main

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/zhifengzhang-sz/qi-v2-contracts/market/go/market"

	"github.com/zhifengzhang-sz/qi-v2-platform/lib/massive"
)

// scratch is a database of its own on the stack's TimescaleDB, by
// QI_WAREHOUSE_TEST_URL (the server, by a database on it), dropped at the
// test's end; the test skips without it. It reads QI_STACK_STATE, which make
// check sets, so Go's test cache keeps its result by the server's state as
// well (go.toolchain.md, 4).
func scratch(t *testing.T) (serverURL, database string) {
	t.Helper()
	_ = os.Getenv("QI_STACK_STATE")
	serverURL = os.Getenv("QI_WAREHOUSE_TEST_URL")
	if serverURL == "" {
		t.Skip("QI_WAREHOUSE_TEST_URL is not set")
	}
	database = fmt.Sprintf("timescaledb_writer_test_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		ctx := context.Background()
		conn, err := pgx.Connect(ctx, serverURL)
		if err != nil {
			t.Errorf("drop %s: %v", database, err)
			return
		}
		defer conn.Close(ctx)
		if _, err := conn.Exec(ctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{database}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Errorf("drop %s: %v", database, err)
		}
	})
	return serverURL, database
}

func one(t *testing.T, conn *pgx.Conn, q string) string {
	t.Helper()
	var s string
	if err := conn.QueryRow(context.Background(), q).Scan(&s); err != nil {
		t.Fatalf("%s: %v", q, err)
	}
	return s
}

// dump is a table's rows, a line each, by key: its K-line field for field, date its milliseconds, then read, a
// NULL as null; of the corrections, their fields; of the clean, no read (0005).
func dump(t *testing.T, conn *pgx.Conn, table string) string {
	t.Helper()
	fields, order := ", coalesce(read::text, 'null')", "instrument, timeframe, date"
	switch table {
	case "corrections":
		fields, order = fields+", array_to_string(corrected, ',')", order+", read"
	case "klines_clean":
		fields = ""
	}
	rs, err := conn.Query(context.Background(), `SELECT concat_ws(' ', instrument, timeframe, "index", date,
		trim_scale(open), trim_scale(high), trim_scale(low), trim_scale(close), trim_scale(volume),
		coalesce(trim_scale(vw)::text, 'null'), coalesce(n::text, 'null')`+fields+`)
		FROM `+table+` ORDER BY `+order)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pgx.CollectRows(rs, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(got, "\n")
}

// versions: each row's ctid and xmin, by which a row not written is seen
func versions(t *testing.T, conn *pgx.Conn, table string) string {
	return one(t, conn, "SELECT coalesce(string_agg(ctid::text || '/' || xmin::text, ',' ORDER BY ctid), '') FROM "+table)
}

func TestStore(t *testing.T) {
	serverURL, database := scratch(t)
	ctx := context.Background()
	st, err := open(ctx, serverURL, database) // the database made, then migrated
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close(ctx)
	again, err := open(ctx, serverURL, database) // there, and migrated: nothing to do
	if err != nil {
		t.Fatal(err)
	}
	again.Close(ctx)
	if v := one(t, st.conn, "SELECT count(*)::text FROM schema_migrations"); v != "5" {
		t.Fatalf("schema_migrations: %s", v)
	}
	// The four series: each table's columns, those that may be NULL, and a hypertable on date; the clean without read.
	kc := "instrument text, timeframe text, index bigint, date bigint, open numeric(21,6), high numeric(21,6), " +
		"low numeric(21,6), close numeric(21,6), volume numeric(21,6), vw numeric(21,6), n bigint"
	k := kc + ", read bigint"
	for table, want := range map[string][2]string{
		"klines": {k, "vw,n,read"}, "klines_corrected": {k, "vw,n,read"}, "klines_clean": {kc, "vw,n"},
		"corrections": {k + ", corrected text[]", "vw,n"},
	} {
		if c := one(t, st.conn, `SELECT string_agg(attname || ' ' || format_type(atttypid, atttypmod), ', ' ORDER BY attnum)
			FROM pg_attribute WHERE attrelid = '`+table+`'::regclass AND attnum > 0 AND NOT attisdropped`); c != want[0] {
			t.Fatalf("%s: %s", table, c)
		}
		if c := one(t, st.conn, `SELECT string_agg(attname, ',' ORDER BY attnum) FROM pg_attribute
			WHERE attrelid = '`+table+`'::regclass AND attnum > 0 AND NOT attisdropped AND NOT attnotnull`); c != want[1] {
			t.Fatalf("%s's columns that may be NULL: %s", table, c)
		}
		if h := one(t, st.conn, `SELECT EXISTS (SELECT 1 FROM timescaledb_information.dimensions
			WHERE hypertable_name = '`+table+`' AND column_name = 'date' AND integer_interval = 604800000)::text`); h != "true" {
			t.Fatalf("%s a hypertable on date: %s", table, h)
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
	h, err := st.Held(ctx, []minute{spy0, spy1, qqq0, {"SPY", "M1", 1}}, nil)
	if err != nil || len(h.raw) != 3 || len(h.corrected) != 3 {
		t.Fatalf("held: %+v, %v", h, err)
	}
	for _, r := range rs {
		if h.raw[r.m] != r.v || h.corrected[r.m] != r.v {
			t.Fatalf("held %+v: %+v, %+v", r.m, h.raw[r.m], h.corrected[r.m])
		}
	}
	before := versions(t, st.conn, "klines") + ";" + versions(t, st.conn, "klines_corrected")
	// The same rows again: none written, not a row touched.
	if p, err := st.Write(ctx, rows{raw: rs, corrected: rs}); err != nil || p != (put{}) {
		t.Fatalf("write again: %+v, %v", p, err)
	}
	if after := versions(t, st.conn, "klines") + ";" + versions(t, st.conn, "klines_corrected"); after != before {
		t.Fatalf("rows written again: %s, before %s", after, before)
	}
	// A correction of SPY's bar 1, read later: inserted, the corrected raw's row replaced; a row of the corrected raw
	// of an earlier read: not written.
	fixed := version{kline(t, 1, "600.25"), R2}
	c := crow{stamp{spy1, R2}, correction{fixed, []string{"close"}}}
	if p, err := st.Write(ctx, rows{corrections: []crow{c}, corrected: []row{{spy1, fixed}}}); err != nil ||
		p != (put{corrections: 1, corrected: 1}) {
		t.Fatalf("a correction: %+v, %v", p, err)
	}
	if p, err := st.Write(ctx, rows{corrections: []crow{c}, corrected: []row{{spy1, version{kline(t, 1, "600.3"), R1}}}}); err != nil ||
		p != (put{}) {
		t.Fatalf("again, the corrected raw of an earlier read: %+v, %v", p, err)
	}
	h, err = st.Held(ctx, []minute{spy1}, []stamp{c.s, {spy1, R3}})
	if err != nil || len(h.corrections) != 1 || !h.corrections[c.s].same(c.c) || h.corrected[spy1] != fixed ||
		h.raw[spy1] != rs[1].v {
		t.Fatalf("held after the correction: %+v, %v", h, err)
	}
	for table, want := range map[string]string{
		"klines": "QQQ M1 0 1764167400000 600 601 599 600 10 null null null\n" +
			"SPY M1 0 1764167400000 600 601 599 600.1 0.000001 999999999.999999 9007199254740991 1764280800000\n" +
			"SPY M1 1 1764167460000 600 601 599 600.2 10 600.5 3 1764280800000",
		"klines_corrected": "QQQ M1 0 1764167400000 600 601 599 600 10 null null null\n" +
			"SPY M1 0 1764167400000 600 601 599 600.1 0.000001 999999999.999999 9007199254740991 1764280800000\n" +
			"SPY M1 1 1764167460000 600 601 599 600.25 10 600.5 3 1764367200000",
		"corrections":  "SPY M1 1 1764167460000 600 601 599 600.25 10 600.5 3 1764367200000 close",
		"klines_clean": "",
	} {
		if got := dump(t, st.conn, table); got != want {
			t.Fatalf("%s:\n%s\nwant:\n%s", table, got, want)
		}
	}

	// The corrected raw of a series, in the order of its dates; none of a series not there.
	var got []string
	if err := st.Corrected(ctx, "SPY", "M1", func(k massive.KLine) error {
		text, err := massive.Text(k)
		got = append(got, strings.TrimSpace(text))
		return err
	}); err != nil || len(got) != 2 || !strings.Contains(got[0], `"close":600.1,`) || !strings.Contains(got[1], `"close":600.25,`) {
		t.Fatalf("the corrected raw: %q, %v", got, err)
	}
	if err := st.Corrected(ctx, "SPY", "H1", func(k massive.KLine) error { return fmt.Errorf("a row") }); err != nil {
		t.Fatalf("the corrected raw of none: %v", err)
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
	if got, want := dump(t, st.conn, "klines_clean"), "QQQ M1 0 1764167400000 600 601 599 600 10 600.5 3\n"+
		"SPY M1 0 1764167460000 600 601 599 600.25 10 600.5 3"; got != want {
		t.Fatalf("klines_clean:\n%s\nwant:\n%s", got, want)
	}
	// a minute twice: refused by the primary key, the series as it was
	if _, err := st.Clean(ctx, "SPY", "M1", []row{{spy0, rs[0].v}, {spy0, rs[0].v}}); err == nil {
		t.Fatalf("clean, a minute twice: no error")
	}
	if n, err := st.Clean(ctx, "SPY", "M1", nil); err != nil || n != 0 || !strings.HasPrefix(dump(t, st.conn, "klines_clean"), "QQQ") {
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

// TestMigration0004: a store at 0003 holding bars migrated by 0004: not a row of klines changed (each row's ctid and
// xmin the same), each row read NULL, the corrected raw its rows, the corrections and the clean empty.
func TestMigration0004(t *testing.T) {
	serverURL, database := scratch(t)
	ctx := context.Background()
	if err := create(ctx, mustConfig(t, serverURL), database); err != nil {
		t.Fatal(err)
	}
	cfg := mustConfig(t, serverURL)
	cfg.Database = database
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	ms, err := list()
	if err != nil || len(ms) != 5 {
		t.Fatalf("migrations: %d, %v", len(ms), err)
	}
	if _, err := conn.Exec(ctx, `CREATE TABLE schema_migrations (version integer PRIMARY KEY, name text NOT NULL,
		applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		t.Fatal(err)
	}
	for _, m := range ms[:3] {
		if _, err := conn.Exec(ctx, m.sql); err != nil {
			t.Fatalf("%s: %v", m.name, err)
		}
		if _, err := conn.Exec(ctx, "INSERT INTO schema_migrations (version, name) VALUES ($1, $2)", m.version, m.name); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := conn.Exec(ctx, `INSERT INTO klines VALUES ('SPY', 'M1', 0, 1764167400000, 600, 601, 599, 600.1, 10, 600.5, 3),
		('SPY', 'M1', 1, 1764167460000, 600, 601, 599, 600.2, 10, NULL, NULL), ('QQQ', 'M1', 0, 1764167400000, 600, 601, 599, 600, 10, 600.5, 3)`); err != nil {
		t.Fatal(err)
	}
	before := versions(t, conn, "klines")
	st, err := open(ctx, serverURL, database)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close(ctx)
	if v := one(t, conn, "SELECT string_agg(name, ',' ORDER BY version) FROM schema_migrations"); !strings.HasSuffix(v, ",0004_series.sql,0005_clean.sql") {
		t.Fatalf("schema_migrations: %s", v)
	}
	if after := versions(t, conn, "klines"); after != before {
		t.Fatalf("klines' rows written: %s, before %s", after, before)
	}
	raw := dump(t, conn, "klines")
	if !strings.Contains(raw, "SPY M1 1 1764167460000 600 601 599 600.2 10 null null null") || strings.Count(raw, "\n") != 2 {
		t.Fatalf("klines: %s", raw)
	}
	if c := dump(t, conn, "klines_corrected"); c != raw {
		t.Fatalf("the corrected raw:\n%s\nthe raw:\n%s", c, raw)
	}
	for _, table := range []string{"corrections", "klines_clean"} {
		if d := dump(t, conn, table); d != "" {
			t.Fatalf("%s: %s", table, d)
		}
	}
	if !slices.Equal(strings.Split(one(t, conn, `SELECT string_agg(hypertable_name, ',' ORDER BY hypertable_name)
		FROM timescaledb_information.hypertables`), ","), []string{"corrections", "klines", "klines_clean", "klines_corrected"}) {
		t.Fatalf("hypertables")
	}
}

// TestMigration0005: a store at 0004 holding rows in each series but the clean, which is empty, migrated by 0005: not a
// row of the other three changed (each row's ctid and xmin the same), the clean without read.
func TestMigration0005(t *testing.T) {
	serverURL, database := scratch(t)
	ctx := context.Background()
	if err := create(ctx, mustConfig(t, serverURL), database); err != nil {
		t.Fatal(err)
	}
	cfg := mustConfig(t, serverURL)
	cfg.Database = database
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	ms, err := list()
	if err != nil || len(ms) != 5 || ms[4].name != "0005_clean.sql" {
		t.Fatalf("migrations: %d, %v", len(ms), err)
	}
	if _, err := conn.Exec(ctx, `CREATE TABLE schema_migrations (version integer PRIMARY KEY, name text NOT NULL,
		applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		t.Fatal(err)
	}
	for _, m := range ms[:4] {
		if _, err := conn.Exec(ctx, m.sql); err != nil {
			t.Fatalf("%s: %v", m.name, err)
		}
		if _, err := conn.Exec(ctx, "INSERT INTO schema_migrations (version, name) VALUES ($1, $2)", m.version, m.name); err != nil {
			t.Fatal(err)
		}
	}
	for _, q := range []string{
		`INSERT INTO klines VALUES ('SPY', 'M1', 0, 1764167400000, 600, 601, 599, 600.1, 10, 600.5, 3, 1764280800000)`,
		`INSERT INTO klines_corrected VALUES ('SPY', 'M1', 0, 1764167400000, 600, 601, 599, 600.2, 10, 600.5, 3, 1764367200000)`,
		`INSERT INTO corrections VALUES ('SPY', 'M1', 0, 1764167400000, 600, 601, 599, 600.2, 10, 600.5, 3, 1764367200000, '{close}')`,
	} {
		if _, err := conn.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	tables := []string{"klines", "klines_corrected", "corrections"}
	before := map[string]string{}
	for _, tb := range tables {
		before[tb] = versions(t, conn, tb) + " " + dump(t, conn, tb)
	}
	st, err := open(ctx, serverURL, database)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close(ctx)
	if v := one(t, conn, "SELECT max(version)::text FROM schema_migrations"); v != "5" {
		t.Fatalf("schema_migrations: %s", v)
	}
	for _, tb := range tables {
		if after := versions(t, conn, tb) + " " + dump(t, conn, tb); after != before[tb] {
			t.Fatalf("%s's rows changed: %s, before %s", tb, after, before[tb])
		}
	}
	if c := one(t, conn, `SELECT string_agg(attname, ',' ORDER BY attnum) FROM pg_attribute
		WHERE attrelid = 'klines_clean'::regclass AND attnum > 0 AND NOT attisdropped`); c != "instrument,timeframe,index,date,open,high,low,close,volume,vw,n" {
		t.Fatalf("klines_clean: %s", c)
	}
}

func mustConfig(t *testing.T, url string) *pgx.ConnConfig {
	t.Helper()
	cfg, err := pgx.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}
