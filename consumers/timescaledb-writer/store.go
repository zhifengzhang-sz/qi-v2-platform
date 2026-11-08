package main

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/zhifengzhang-sz/qi-v2-contracts/market/go/market"

	"github.com/zhifengzhang-sz/qi-v2-platform/lib/massive"
)

//go:embed migrations/*.sql
var migrations embed.FS

// pg is the store in a database of TimescaleDB.
type pg struct{ conn *pgx.Conn }

// open connects to the database on the server serverURL names (its own
// database the one it connects to first, to make the store's), making it
// when it is not there, and migrates it.
func open(ctx context.Context, serverURL, database string) (*pg, error) {
	cfg, err := pgx.ParseConfig(serverURL)
	if err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	at := cfg.Copy()
	at.Database = database
	conn, err := pgx.ConnectConfig(ctx, at)
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "3D000" { // invalid_catalog_name: no such database
		if err := create(ctx, cfg, database); err != nil {
			return nil, err
		}
		conn, err = pgx.ConnectConfig(ctx, at)
	}
	if err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	s := &pg{conn: conn}
	if err := s.migrate(ctx); err != nil {
		conn.Close(ctx)
		return nil, err
	}
	return s, nil
}

// create makes the database, from the server's database cfg names.
func create(ctx context.Context, cfg *pgx.ConnConfig, database string) error {
	server, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("store: %w", err)
	}
	defer server.Close(ctx)
	_, err = server.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{database}.Sanitize())
	var pe *pgconn.PgError
	if err != nil && !(errors.As(err, &pe) && pe.Code == "42P04") { // duplicate_database: made meanwhile
		return fmt.Errorf("store: create database %s: %w", database, err)
	}
	return nil
}

func (s *pg) Close(ctx context.Context) error { return s.conn.Close(ctx) }

// migrationLock: the session's advisory lock while it migrates, so that two
// writers starting at once migrate one after the other.
const migrationLock = 0x6b6c696e6573 // "klines"

// migrate applies each migration not yet applied, in the order of its
// number, each in a transaction with its row in schema_migrations; a
// database migrated past the migrations this writer knows is refused.
func (s *pg) migrate(ctx context.Context) error {
	if _, err := s.conn.Exec(ctx, "SELECT pg_advisory_lock($1)", migrationLock); err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	defer s.conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", migrationLock)
	if _, err := s.conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version integer PRIMARY KEY, name text NOT NULL, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	all, err := list()
	if err != nil {
		return err
	}
	rows, err := s.conn.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	applied, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return fmt.Errorf("store: migrate: %w", err)
	}
	for _, v := range applied {
		if !slices.ContainsFunc(all, func(m migration) bool { return m.version == v }) {
			return fmt.Errorf("store: migrate: the database has migration %d, which this writer does not know", v)
		}
	}
	for _, m := range all {
		if slices.Contains(applied, m.version) {
			continue
		}
		err := pgx.BeginFunc(ctx, s.conn, func(tx pgx.Tx) error {
			if _, err := tx.Exec(ctx, m.sql); err != nil {
				return err
			}
			_, err := tx.Exec(ctx, "INSERT INTO schema_migrations (version, name) VALUES ($1, $2)", m.version, m.name)
			return err
		})
		if err != nil {
			return fmt.Errorf("store: migrate %s: %w", m.name, err)
		}
	}
	return nil
}

type migration struct {
	version int32
	name    string
	sql     string
}

// list: the migrations, NNNN_name.sql, by their number.
func list() ([]migration, error) {
	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return nil, err
	}
	var out []migration
	for _, p := range names {
		name := strings.TrimPrefix(p, "migrations/")
		n, _, ok := strings.Cut(name, "_")
		v, err := strconv.ParseInt(n, 10, 32)
		if !ok || err != nil || v < 1 {
			return nil, fmt.Errorf("store: migration %s: not NNNN_name.sql", name)
		}
		b, err := migrations.ReadFile(p)
		if err != nil {
			return nil, err
		}
		out = append(out, migration{version: int32(v), name: name, sql: string(b)})
	}
	slices.SortFunc(out, func(a, b migration) int { return int(a.version - b.version) })
	for i := 1; i < len(out); i++ {
		if out[i].version == out[i-1].version {
			return nil, fmt.Errorf("store: migrations %s and %s: one number", out[i-1].name, out[i].name)
		}
	}
	return out, nil
}

func (s *pg) Empty(ctx context.Context) (bool, error) {
	var empty bool
	if err := s.conn.QueryRow(ctx, "SELECT NOT EXISTS (SELECT 1 FROM klines)").Scan(&empty); err != nil {
		return false, fmt.Errorf("store: %w", err)
	}
	return empty, nil
}

// columns: the K-line's, then read, as each statement below has them
const columns = `instrument, timeframe, "index", date, open, high, low, close, volume, vw, n, read`

// unnest: the rows given as arrays, $1 to $12, each column of its type, a decimal from its text, NULL where vw, n or
// read is none; $13, a correction's fields, comma-separated.
const (
	arrays = `$1::text[], $2::text[], $3::bigint[], $4::bigint[], $5::text[], $6::text[], $7::text[], $8::text[],
       $9::text[], $10::text[], $11::bigint[], $12::bigint[]`
	typed = `u.instrument, u.timeframe, u.idx, u.date, u.open::numeric, u.high::numeric, u.low::numeric,
       u.close::numeric, u.volume::numeric, u.vw::numeric, u.n, u.read`
	names = `instrument, timeframe, idx, date, open, high, low, close, volume, vw, n, read`
)

// The writes (README.md, The store), each ON CONFLICT with its reason there. The raw and the corrections: a row once,
// the first version and each correction by its read, inserted; a row put meanwhile by another member of the group, in
// a rebalance, is of the same message, and is kept (DO NOTHING). The corrected raw: a minute's row inserted, or
// replaced by a version of a later read alone, so that the row of the greatest read wins whichever member writes
// last. RETURNING gives each row written.
const (
	insertRaw = `INSERT INTO klines (` + columns + `)
SELECT ` + typed + ` FROM unnest(` + arrays + `) AS u(` + names + `)
ON CONFLICT (instrument, timeframe, date) DO NOTHING
RETURNING 1`
	insertCorrections = `INSERT INTO corrections (` + columns + `, corrected)
SELECT ` + typed + `, string_to_array(u.corrected, ',')
FROM unnest(` + arrays + `, $13::text[]) AS u(` + names + `, corrected)
ON CONFLICT (instrument, timeframe, date, read) DO NOTHING
RETURNING 1`
	putCorrected = `INSERT INTO klines_corrected AS k (` + columns + `)
SELECT ` + typed + ` FROM unnest(` + arrays + `) AS u(` + names + `)
ON CONFLICT (instrument, timeframe, date) DO UPDATE
SET "index" = excluded."index", open = excluded.open, high = excluded.high, low = excluded.low, close = excluded.close,
    volume = excluded.volume, vw = excluded.vw, n = excluded.n, read = excluded.read
WHERE coalesce(excluded.read, -1) > coalesce(k.read, -1)
RETURNING 1`
)

// args: the rows as unnest's twelve arrays.
func args(rs []row) []any {
	n := len(rs)
	ins, tfs := make([]string, n), make([]string, n)
	idx, dates := make([]int64, n), make([]int64, n)
	open, high, low, cls, vol := make([]string, n), make([]string, n), make([]string, n), make([]string, n),
		make([]string, n)
	vw, ns, reads := make([]*string, n), make([]*int64, n), make([]*int64, n) // NULL where none
	for i, r := range rs {
		k := r.v.k
		ins[i], tfs[i] = r.m.instrument, r.m.timeframe
		idx[i], dates[i] = int64(k.Index), int64(k.Date) // each within market's Bound, 2⁵³ − 1
		open[i], high[i], low[i], cls[i], vol[i] = k.Open.String(), k.High.String(), k.Low.String(), k.Close.String(),
			k.Volume.String()
		if k.HasVw {
			v := k.Vw.String()
			vw[i] = &v
		}
		if k.HasN {
			v := int64(k.N) // within market's Bound
			ns[i] = &v
		}
		if r.v.read != 0 {
			v := int64(r.v.read) // within market's Bound
			reads[i] = &v
		}
	}
	return []any{ins, tfs, idx, dates, open, high, low, cls, vol, vw, ns, reads}
}

// Write: the three statements in one transaction, so all of them or none.
func (s *pg) Write(ctx context.Context, w rows) (put, error) {
	var p put
	err := pgx.BeginFunc(ctx, s.conn, func(tx pgx.Tx) error {
		count := func(n *int, sql string, a []any) error {
			rs, err := tx.Query(ctx, sql, a...)
			if err != nil {
				return err
			}
			var one int
			_, err = pgx.ForEachRow(rs, []any{&one}, func() error { *n++; return nil })
			return err
		}
		if len(w.raw) > 0 {
			if err := count(&p.raw, insertRaw, args(w.raw)); err != nil {
				return err
			}
		}
		if len(w.corrections) > 0 {
			rs := make([]row, len(w.corrections))
			fields := make([]string, len(w.corrections))
			for i, c := range w.corrections {
				rs[i] = row{m: c.s.minute, v: c.c.version}
				fields[i] = strings.Join(c.c.corrected, ",")
			}
			if err := count(&p.corrections, insertCorrections, append(args(rs), fields)); err != nil {
				return err
			}
		}
		if len(w.corrected) > 0 {
			if err := count(&p.corrected, putCorrected, args(w.corrected)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return put{}, fmt.Errorf("store: write: %w", err)
	}
	return p, nil
}

// The clean series, its K-line's columns, no read (0005): the series' rows deleted, then the rows inserted, in one
// transaction, the primary key enforcing each minute once; then its rows counted.
const (
	deleteClean = `DELETE FROM klines_clean WHERE instrument = $1 AND timeframe = $2`
	insertClean = `INSERT INTO klines_clean (instrument, timeframe, "index", date, open, high, low, close, volume, vw, n)
SELECT u.instrument, u.timeframe, u.idx, u.date, u.open::numeric, u.high::numeric, u.low::numeric, u.close::numeric,
       u.volume::numeric, u.vw::numeric, u.n
FROM unnest($1::text[], $2::text[], $3::bigint[], $4::bigint[], $5::text[], $6::text[], $7::text[], $8::text[],
            $9::text[], $10::text[], $11::bigint[]) AS u(instrument, timeframe, idx, date, open, high, low, close,
            volume, vw, n)`
	countClean = `SELECT count(*) FROM klines_clean WHERE instrument = $1 AND timeframe = $2`
)

// Clean: the series' clean rows deleted and the rows inserted, in one transaction, so the old series or the new.
func (s *pg) Clean(ctx context.Context, instrument, timeframe string, rs []row) (int, error) {
	var n int
	err := pgx.BeginFunc(ctx, s.conn, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, deleteClean, instrument, timeframe); err != nil {
			return err
		}
		if len(rs) > 0 {
			if _, err := tx.Exec(ctx, insertClean, args(rs)[:11]...); err != nil {
				return err
			}
		}
		return tx.QueryRow(ctx, countClean, instrument, timeframe).Scan(&n)
	})
	if err != nil {
		return 0, fmt.Errorf("store: klines_clean: %w", err)
	}
	return n, nil
}

// Corrected gives each row of the series' corrected raw, klines_corrected's, in the order of their dates.
func (s *pg) Corrected(ctx context.Context, instrument, timeframe string, each func(massive.KLine) error) error {
	rs, err := s.conn.Query(ctx, `SELECT `+stored+` FROM klines_corrected t WHERE instrument = $1 AND timeframe = $2
ORDER BY date`, instrument, timeframe)
	if err != nil {
		return fmt.Errorf("store: klines_corrected: %w", err)
	}
	defer rs.Close()
	for rs.Next() {
		_, v, err := scan(rs, nil)
		if err != nil {
			return fmt.Errorf("store: klines_corrected: %w", err)
		}
		if err := each(v.k); err != nil {
			return err
		}
	}
	if err := rs.Err(); err != nil {
		return fmt.Errorf("store: klines_corrected: %w", err)
	}
	return nil
}

// keys: the minutes as unnest's three arrays
func keys(ms []minute) []any {
	ins, tfs, dates := make([]string, len(ms)), make([]string, len(ms)), make([]int64, len(ms))
	for i, m := range ms {
		ins[i], tfs[i], dates[i] = m.instrument, m.timeframe, int64(m.date)
	}
	return []any{ins, tfs, dates}
}

// stored: a table's K-line columns and read, each decimal in market's digits (trim_scale: no 0 at its end)
const stored = `t.instrument, t.timeframe, t.date, t."index", trim_scale(t.open)::text, trim_scale(t.high)::text,
       trim_scale(t.low)::text, trim_scale(t.close)::text, trim_scale(t.volume)::text, trim_scale(t.vw)::text, t.n,
       t.read`

// Held: the raw and the corrected raw of the minutes, and the corrections of the stamps, each read by its key.
func (s *pg) Held(ctx context.Context, minutes []minute, stamps []stamp) (held, error) {
	h := held{raw: map[minute]version{}, corrections: map[stamp]correction{}, corrected: map[minute]version{}}
	for _, t := range []struct {
		table string
		into  map[minute]version
	}{{"klines", h.raw}, {"klines_corrected", h.corrected}} {
		if len(minutes) == 0 {
			break
		}
		rs, err := s.conn.Query(ctx, `SELECT `+stored+` FROM `+t.table+` t
JOIN unnest($1::text[], $2::text[], $3::bigint[]) AS u(instrument, timeframe, date) USING (instrument, timeframe, date)`,
			keys(minutes)...)
		if err != nil {
			return held{}, fmt.Errorf("store: %s: %w", t.table, err)
		}
		for rs.Next() {
			m, v, err := scan(rs, nil)
			if err != nil {
				rs.Close()
				return held{}, fmt.Errorf("store: %s: %w", t.table, err)
			}
			t.into[m] = v
		}
		if err := rs.Err(); err != nil {
			return held{}, fmt.Errorf("store: %s: %w", t.table, err)
		}
	}
	if len(stamps) > 0 {
		ms := make([]minute, len(stamps))
		reads := make([]int64, len(stamps))
		for i, st := range stamps {
			ms[i], reads[i] = st.minute, int64(st.read)
		}
		rs, err := s.conn.Query(ctx, `SELECT `+stored+`, t.corrected FROM corrections t
JOIN unnest($1::text[], $2::text[], $3::bigint[], $4::bigint[]) AS u(instrument, timeframe, date, read)
USING (instrument, timeframe, date, read)`, append(keys(ms), reads)...)
		if err != nil {
			return held{}, fmt.Errorf("store: corrections: %w", err)
		}
		for rs.Next() {
			var fields []string
			m, v, err := scan(rs, &fields)
			if err != nil {
				rs.Close()
				return held{}, fmt.Errorf("store: corrections: %w", err)
			}
			h.corrections[stamp{minute: m, read: v.read}] = correction{version: v, corrected: fields}
		}
		if err := rs.Err(); err != nil {
			return held{}, fmt.Errorf("store: corrections: %w", err)
		}
	}
	return h, nil
}

// scan: a stored row, its minute and its version; a correction's fields into fields when not nil.
func scan(rs pgx.Rows, fields *[]string) (minute, version, error) {
	var m minute
	var v version
	var date, index int64
	var o, hi, lo, c, vol string
	var vw *string
	var n, read *int64
	dest := []any{&m.instrument, &m.timeframe, &date, &index, &o, &hi, &lo, &c, &vol, &vw, &n, &read}
	if fields != nil {
		dest = append(dest, fields)
	}
	if err := rs.Scan(dest...); err != nil {
		return m, v, err
	}
	m.date, v.k.Date, v.k.Index = uint64(date), uint64(date), uint64(index)
	for _, d := range []struct {
		to *market.Decimal
		s  string
	}{{&v.k.Open, o}, {&v.k.High, hi}, {&v.k.Low, lo}, {&v.k.Close, c}, {&v.k.Volume, vol}} {
		x, err := market.ParseDecimal(d.s)
		if err != nil {
			return m, v, fmt.Errorf("%q: %w", d.s, err)
		}
		*d.to = x
	}
	if vw != nil {
		x, err := market.ParseDecimal(*vw)
		if err != nil {
			return m, v, fmt.Errorf("%q: %w", *vw, err)
		}
		v.k.Vw, v.k.HasVw = x, true
	}
	if n != nil {
		v.k.N, v.k.HasN = uint64(*n), true
	}
	if read != nil {
		v.read = uint64(*read)
	}
	return m, v, nil
}
