package main

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/zhifengzhang-sz/qi-v2-contracts/market/go/market"
)

//go:embed migrations/*.sql
var migrations embed.FS

// ch is the store in a database of ClickHouse, through its HTTP interface.
type ch struct {
	server   *url.URL
	database string
	client   *http.Client
}

// identifier: a database's name, written as it is in a query
var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// open reaches the ClickHouse server serverURL names (http://user:password@host:8123), makes the database when it
// is not there, and migrates it.
func open(ctx context.Context, serverURL, database string) (*ch, error) {
	u, err := url.Parse(serverURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("store: %q: a ClickHouse server's HTTP URL, http://user:password@host:8123", serverURL)
	}
	if !identifier.MatchString(database) {
		return nil, fmt.Errorf("store: database %q: letters, digits and '_', from a letter or '_'", database)
	}
	s := &ch{server: u, database: database, client: &http.Client{}}
	if _, err := s.do(ctx, false, "CREATE DATABASE IF NOT EXISTS `"+database+"`", nil); err != nil {
		return nil, err
	}
	if err := s.migrate(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *ch) Close(ctx context.Context) error { s.client.CloseIdleConnections(); return nil }

// do runs one statement, in the store's database when inDB; with data, the statement is an INSERT whose rows data
// holds. A statement ClickHouse refuses is an error carrying its answer.
func (s *ch) do(ctx context.Context, inDB bool, statement string, data io.Reader) ([]byte, error) {
	u := *s.server
	q := url.Values{"wait_end_of_query": {"1"}} // the answer's status once the statement has run, not before
	if inDB {
		q.Set("database", s.database)
	}
	body := data
	if data == nil {
		body = strings.NewReader(statement)
	} else {
		q.Set("query", statement)
	}
	u.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("store: %s: %s", first(statement), bytes.TrimSpace(out))
	}
	return out, nil
}

// first: a statement's first line, for an error
func first(statement string) string {
	l, _, _ := strings.Cut(strings.TrimSpace(statement), "\n")
	return l
}

// migrate applies each migration not yet applied, in the order of its number, then its row in schema_migrations; a
// database migrated past the migrations this writer knows is refused. A migration's statements may run again, so
// one cut short is applied again whole.
func (s *ch) migrate(ctx context.Context) error {
	if _, err := s.do(ctx, true, `CREATE TABLE IF NOT EXISTS schema_migrations (version UInt32, name String,
		applied_at DateTime64(3, 'UTC') DEFAULT now64(3)) ENGINE = MergeTree ORDER BY version`, nil); err != nil {
		return err
	}
	all, err := list()
	if err != nil {
		return err
	}
	out, err := s.do(ctx, true, "SELECT DISTINCT version FROM schema_migrations FORMAT TabSeparated", nil)
	if err != nil {
		return err
	}
	var applied []int32
	for _, l := range strings.Fields(string(out)) {
		v, err := strconv.ParseInt(l, 10, 32)
		if err != nil {
			return fmt.Errorf("store: migrate: schema_migrations: %q", l)
		}
		if !slices.ContainsFunc(all, func(m migration) bool { return m.version == int32(v) }) {
			return fmt.Errorf("store: migrate: the database has migration %d, which this writer does not know", v)
		}
		applied = append(applied, int32(v))
	}
	for _, m := range all {
		if slices.Contains(applied, m.version) {
			continue
		}
		for _, st := range statements(m.sql) {
			if _, err := s.do(ctx, true, st, nil); err != nil {
				return fmt.Errorf("store: migrate %s: %w", m.name, err)
			}
		}
		row := strconv.Itoa(int(m.version)) + "\t" + m.name + "\n"
		if _, err := s.do(ctx, true, "INSERT INTO schema_migrations (version, name) FORMAT TabSeparated",
			strings.NewReader(row)); err != nil {
			return fmt.Errorf("store: migrate %s: %w", m.name, err)
		}
	}
	return nil
}

// statements: a migration's statements, each ending with ';' at the end of a line; its lines of comment left out.
func statements(sql string) []string {
	var out []string
	var b strings.Builder
	for _, l := range strings.Split(sql, "\n") {
		if t := strings.TrimSpace(l); t == "" || strings.HasPrefix(t, "--") {
			continue
		}
		b.WriteString(l + "\n")
		if strings.HasSuffix(strings.TrimSpace(l), ";") {
			out = append(out, strings.TrimSuffix(strings.TrimSpace(b.String()), ";"))
			b.Reset()
		}
	}
	if t := strings.TrimSpace(b.String()); t != "" {
		out = append(out, t)
	}
	return out
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

func (s *ch) Empty(ctx context.Context) (bool, error) {
	out, err := s.do(ctx, true, "SELECT count() = 0 FROM klines FORMAT TabSeparated", nil)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(out)) == "1", nil
}

// columns: a table's K-line columns and read, in the order of its rows on the wire
const columns = "instrument, timeframe, `index`, date, open, high, low, close, volume, vw, n, `read`"

// tsv: a row's columns as TabSeparated has them, \N where vw, n or read is none
func tsv(b *bytes.Buffer, r row) {
	tsvKLine(b, r)
	read := `\N`
	if r.v.read != 0 {
		read = strconv.FormatUint(r.v.read, 10)
	}
	b.WriteString("\t" + read)
}

// tsvKLine: a row's K-line columns, the series' and the K-line's, as TabSeparated has them, \N where vw or n is none
func tsvKLine(b *bytes.Buffer, r row) {
	k := r.v.k
	vw, n := `\N`, `\N`
	if k.HasVw {
		vw = k.Vw.String()
	}
	if k.HasN {
		n = strconv.FormatUint(k.N, 10)
	}
	fmt.Fprintf(b, "%s\t%s\t%d\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s", r.m.instrument, r.m.timeframe, k.Index, k.Date,
		k.Open, k.High, k.Low, k.Close, k.Volume, vw, n)
}

// Write: the rows of each table in one INSERT, so all of them or none, the raw's, then the corrections', then the
// corrected raw's. Of the rows of one key, FINAL and the merges keep the one inserted last: a row of the corrected
// raw inserted replaces its minute's (README.md, The store). The three are not one transaction: a write cut short
// is made again by the fetch read again, its offsets not committed, each minute's corrected raw taken again from its
// versions.
func (s *ch) Write(ctx context.Context, w rows) (put, error) {
	var p put
	insert := func(table, cols string, b *bytes.Buffer) error {
		if b.Len() == 0 {
			return nil
		}
		if _, err := s.do(ctx, true, "INSERT INTO "+table+" ("+cols+") FORMAT TabSeparated", b); err != nil {
			return fmt.Errorf("store: write %s: %w", table, err)
		}
		return nil
	}
	var b bytes.Buffer
	for _, r := range w.raw {
		tsv(&b, r)
		b.WriteByte('\n')
	}
	if err := insert("klines", columns, &b); err != nil {
		return put{}, err
	}
	p.raw = len(w.raw)
	b.Reset()
	for _, c := range w.corrections {
		tsv(&b, row{m: c.s.minute, v: c.c.version})
		b.WriteString("\t[")
		for i, f := range c.c.corrected {
			if i > 0 {
				b.WriteByte(',')
			}
			b.WriteString(quote(f))
		}
		b.WriteString("]\n")
	}
	if err := insert("corrections", columns+", corrected", &b); err != nil {
		return p, err
	}
	p.corrections = len(w.corrections)
	b.Reset()
	for _, r := range w.corrected {
		tsv(&b, r)
		b.WriteByte('\n')
	}
	if err := insert("klines_corrected", columns, &b); err != nil {
		return p, err
	}
	p.corrected = len(w.corrected)
	return p, nil
}

// cleanColumns: the clean series' columns, its K-line's, no read (0005)
const cleanColumns = "instrument, timeframe, `index`, date, open, high, low, close, volume, vw, n"

// Clean: the series' clean rows deleted, by a mutation it waits for (mutations_sync), then the rows in one INSERT,
// then its rows counted with FINAL. Not one transaction: a write cut short leaves the series empty, or part of it,
// and is made again whole by -clean run again (README.md, The store).
func (s *ch) Clean(ctx context.Context, instrument, timeframe string, rs []row) (int, error) {
	where := "instrument = " + quote(instrument) + " AND timeframe = " + quote(timeframe)
	if _, err := s.do(ctx, true, "ALTER TABLE klines_clean DELETE WHERE "+where+" SETTINGS mutations_sync = 2",
		nil); err != nil {
		return 0, err
	}
	if len(rs) > 0 {
		var b bytes.Buffer
		for _, r := range rs {
			tsvKLine(&b, r)
			b.WriteByte('\n')
		}
		if _, err := s.do(ctx, true, "INSERT INTO klines_clean ("+cleanColumns+") FORMAT TabSeparated", &b); err != nil {
			return 0, fmt.Errorf("store: write klines_clean: %w", err)
		}
	}
	out, err := s.do(ctx, true, "SELECT count() FROM klines_clean FINAL WHERE "+where+" FORMAT TabSeparated", nil)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("store: klines_clean: a count %q", out)
	}
	return n, nil
}

// Held: the raw, the corrections and the corrected raw the store holds of the minutes' series over the span of their
// dates, read with FINAL, each by its key; the corrections, of the stamps'.
func (s *ch) Held(ctx context.Context, minutes []minute, stamps []stamp) (held, error) {
	h := held{raw: map[minute]version{}, corrections: map[stamp]correction{}, corrected: map[minute]version{}}
	all := append([]minute(nil), minutes...)
	for _, st := range stamps {
		all = append(all, st.minute)
	}
	if len(all) == 0 {
		return h, nil
	}
	type series struct{ instrument, timeframe string }
	span := map[series][2]uint64{}
	var order []series
	for _, m := range all {
		sr := series{m.instrument, m.timeframe}
		sp, ok := span[sr]
		if !ok {
			sp = [2]uint64{m.date, m.date}
			order = append(order, sr)
		}
		span[sr] = [2]uint64{min(sp[0], m.date), max(sp[1], m.date)}
	}
	var where []string
	for _, sr := range order {
		sp := span[sr]
		where = append(where, fmt.Sprintf("(instrument = %s AND timeframe = %s AND date BETWEEN %d AND %d)",
			quote(sr.instrument), quote(sr.timeframe), sp[0], sp[1]))
	}
	read := func(table, cols string, each func(f []string) error) error {
		out, err := s.do(ctx, true, "SELECT "+cols+" FROM "+table+" FINAL WHERE "+strings.Join(where, " OR ")+
			" FORMAT TabSeparated", nil)
		if err != nil {
			return fmt.Errorf("store: %s: %w", table, err)
		}
		sc := bufio.NewScanner(bytes.NewReader(out))
		sc.Buffer(make([]byte, 0, 4096), 1<<20)
		for sc.Scan() {
			if err := each(strings.Split(sc.Text(), "\t")); err != nil {
				return fmt.Errorf("store: %s: a stored row %q: %w", table, sc.Text(), err)
			}
		}
		return sc.Err()
	}
	for _, t := range []struct {
		table string
		into  map[minute]version
	}{{"klines", h.raw}, {"klines_corrected", h.corrected}} {
		if err := read(t.table, columns, func(f []string) error {
			m, v, err := parse(f)
			if err == nil {
				t.into[m] = v
			}
			return err
		}); err != nil {
			return held{}, err
		}
	}
	if len(stamps) > 0 {
		want := map[stamp]bool{}
		for _, st := range stamps {
			want[st] = true
		}
		if err := read("corrections", columns+", corrected", func(f []string) error {
			if len(f) != 13 {
				return fmt.Errorf("%d columns, not 13", len(f))
			}
			m, v, err := parse(f[:12])
			if err != nil {
				return err
			}
			fields, err := array(f[12])
			if err != nil {
				return err
			}
			if st := (stamp{minute: m, read: v.read}); want[st] {
				h.corrections[st] = correction{version: v, corrected: fields}
			}
			return nil
		}); err != nil {
			return held{}, err
		}
	}
	return h, nil
}

// parse: a stored row's minute and version, from its columns as ClickHouse writes them: a decimal in market's digits,
// no 0 at its end, which market reads as it is; vw, n and read not set where they are NULL (\N).
func parse(f []string) (minute, version, error) {
	var m minute
	var v version
	if len(f) != 12 {
		return m, v, fmt.Errorf("%d columns, not 12", len(f))
	}
	k := &v.k
	m.instrument, m.timeframe = f[0], f[1]
	k.HasVw, k.HasN = f[9] != `\N`, f[10] != `\N`
	ints := []*uint64{&k.Index, &k.Date, &k.N, &v.read}
	for i, c := range []string{f[2], f[3], f[10], f[11]} {
		if (i == 2 && !k.HasN) || (i == 3 && c == `\N`) {
			continue
		}
		x, err := strconv.ParseUint(c, 10, 64)
		if err != nil {
			return m, v, err
		}
		*ints[i] = x
	}
	m.date = k.Date
	decs := []*market.Decimal{&k.Open, &k.High, &k.Low, &k.Close, &k.Volume, &k.Vw}
	for i, c := range f[4:10] {
		if i == 5 && !k.HasVw {
			continue
		}
		d, err := market.ParseDecimal(c)
		if err != nil {
			return m, v, fmt.Errorf("%q: %w", c, err)
		}
		*decs[i] = d
	}
	return m, v, nil
}

// array: an Array(String) of names as TabSeparated writes it, ['a','b']; each name of letters and digits.
func array(s string) ([]string, error) {
	if !strings.HasPrefix(s, "[") || !strings.HasSuffix(s, "]") {
		return nil, fmt.Errorf("%q: not an array", s)
	}
	var out []string
	for _, e := range strings.Split(s[1:len(s)-1], ",") {
		if len(e) < 3 || e[0] != '\'' || e[len(e)-1] != '\'' || strings.ContainsAny(e[1:len(e)-1], `'\`) {
			return nil, fmt.Errorf("%q: not an array of names", s)
		}
		out = append(out, e[1:len(e)-1])
	}
	return out, nil
}

// quote: a string literal of ClickHouse's SQL
func quote(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}
