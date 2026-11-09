package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-contracts/market/go/market"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/clean"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/raw"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream/mem"
)

const topic = "market.clean.validation.v0-1-0.massive.SPY.M1"

var ny = func() *time.Location {
	l, err := time.LoadLocation("America/New_York")
	if err != nil {
		panic(err)
	}
	return l
}()

// at is a minute in New York, in market's date.
func at(y int, m time.Month, d, h, min int) uint64 {
	return uint64(time.Date(y, m, d, h, min, 0, 0, ny).UnixMilli())
}

// dates are the tests' K-lines' dates: a minute before the range, three of 2026-10 in New York (the last one
// 2026-11-01 in UTC), one of 2026-11, and one after the range, which is 2026-10-30 to 2026-11-02.
var dates = []uint64{at(2026, 10, 29, 15, 59), at(2026, 10, 30, 9, 30), at(2026, 10, 30, 9, 31),
	at(2026, 10, 31, 23, 30), at(2026, 11, 2, 9, 30), at(2026, 11, 3, 9, 30)}

// source is a clean value's K-line as Massive's source gives it, its own fields n and vw beside the base's.
func source(t *testing.T, index int, date uint64, volume string) raw.KLine {
	t.Helper()
	k, err := raw.DecodeKLine([]byte(fmt.Sprintf(`{"close":700.75,"date":%d,"high":701.25,"index":%d,"low":699.5,`+
		`"n":12,"open":700.1,"volume":%s,"vw":700.6}`, date, index, volume)))
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func message(k raw.KLine) clean.Message {
	return clean.Message{Cleaning: clean.Cleaning{Name: "validation", Version: "0.1.0"}, Origin: raw.Live,
		Source: "massive", Instrument: "SPY", Timeframe: "M1", Publisher: "acquisition", KLine: k}
}

// bus is an in-memory bus whose clean topic holds the K-lines, in that order.
func bus(t *testing.T, ks ...raw.KLine) stream.Bus {
	t.Helper()
	ctx := context.Background()
	b := mem.New()
	adm, err := b.Admin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := adm.Ensure(ctx, clean.Spec(topic, 1)); err != nil {
		t.Fatal(err)
	}
	sink, err := b.Sink(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer sink.Close(ctx)
	for _, k := range ks {
		r, err := message(k).Record()
		if err != nil {
			t.Fatal(err)
		}
		if err := sink.Publish(ctx, r); err != nil {
			t.Fatal(err)
		}
	}
	return b
}

// klines is a topic of K-lines of the dates, each its index its place.
func klines(t *testing.T, ds ...uint64) []raw.KLine {
	var ks []raw.KLine
	for i, d := range ds {
		ks = append(ks, source(t, i, d, fmt.Sprint(1000+slices.Index(dates, d))))
	}
	return ks
}

func testSpec(t *testing.T) spec {
	t.Helper()
	s, err := parse(topic, "America/New_York", "2026-10-30", "2026-11-02", "qi-v2-platform-klines-test", "1.2.3",
		"ghcr.io/owner/qi-v2-platform-klines-test")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// build runs the program into a folder of the test's own: the folder and the run's line.
func build(t *testing.T, b stream.Bus, s spec) (string, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "out")
	var log bytes.Buffer
	if err := run(context.Background(), b, s, dir, &log); err != nil {
		t.Fatal(err)
	}
	return dir, log.String()
}

// want is the data set's line of the test's K-line i at index n: market's K-line alone, Massive's n and vw left out.
func want(t *testing.T, i, n int) string {
	t.Helper()
	b := source(t, i, dates[i], fmt.Sprint(1000+i)).Base()
	b.Index = uint64(n)
	return fmt.Sprintf(`{"close":700.75,"date":%d,"high":701.25,"index":%d,"low":699.5,"open":700.1,"volume":%d}`,
		b.Date, n, 1000+i)
}

func TestMake(t *testing.T) {
	s := testSpec(t)
	dir, line := build(t, bus(t, klines(t, dates...)...), s)
	files := map[string]string{
		"2026-10.jsonl": want(t, 1, 0) + "\n" + want(t, 2, 1) + "\n" + want(t, 3, 2) + "\n",
		"2026-11.jsonl": want(t, 4, 3) + "\n",
	}
	names, err := os.ReadDir(filepath.Join(dir, "files"))
	if err != nil || len(names) != len(files) {
		t.Fatalf("files: %v, %v", names, err)
	}
	for name, text := range files {
		b, err := os.ReadFile(filepath.Join(dir, "files", name))
		if err != nil || string(b) != text {
			t.Fatalf("%s: %q, %v; want %q", name, b, err, text)
		}
		for l := range strings.SplitSeq(strings.TrimSuffix(text, "\n"), "\n") {
			if k, err := raw.DecodeKLine([]byte(l)); err != nil || k.Text() != l {
				t.Errorf("%s: %s not market's K-line in its canonical text: %v", name, l, err)
			}
		}
	}
	for _, w := range []string{"6 K-lines read, 0 copies dropped (0 of another value)",
		"2026-10-30T09:30-04:00 to 2026-11-02T09:30-05:00, 4 K-lines in 2 files", "the image sha256:"} {
		if !strings.Contains(line, w) {
			t.Errorf("the line %q: no %q", line, w)
		}
	}

	// The lock: what it is made of, and the image's digest and each file's sha256, size and K-lines.
	var l lockFile
	lb, err := os.ReadFile(filepath.Join(dir, s.name+".lock"))
	if err != nil || json.Unmarshal(lb, &l) != nil {
		t.Fatalf("lock: %s, %v", lb, err)
	}
	if l.Dataset != s.name || l.Version != "1.2.3" || l.Tag != "1.2.3" || l.Image != s.image ||
		l.Made != (made{Topic: topic, Zone: "America/New_York", From: "2026-10-30", To: "2026-11-02"}) ||
		len(l.Files) != 2 || !bytes.HasSuffix(lb, []byte("}\n")) || !bytes.HasPrefix(lb, []byte("{\n  \"dataset\": ")) {
		t.Fatalf("lock: %s", lb)
	}
	for i, name := range []string{"2026-10.jsonl", "2026-11.jsonl"} {
		sum := sha256.Sum256([]byte(files[name]))
		if e := l.Files[i]; e != (lockEntry{Path: name, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(files[name])),
			KLines: strings.Count(files[name], "\n")}) {
			t.Errorf("lock: %+v", e)
		}
	}

	// The layout: its index names the manifest by its tag; the manifest the config and a layer a file; each layer the
	// tar of its file alone, gzip-compressed; each blob named by its digest.
	oci := filepath.Join(dir, "oci")
	blob := func(d descriptor, media string) []byte {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(oci, "blobs", "sha256", strings.TrimPrefix(d.Digest, "sha256:")))
		if err != nil || digest(b) != d.Digest || int64(len(b)) != d.Size || d.MediaType != media {
			t.Fatalf("blob %+v: %d bytes, %v", d, len(b), err)
		}
		return b
	}
	if b, err := os.ReadFile(filepath.Join(oci, "oci-layout")); err != nil || string(b) != `{"imageLayoutVersion":"1.0.0"}` {
		t.Fatalf("oci-layout: %s, %v", b, err)
	}
	var ix index
	if b, err := os.ReadFile(filepath.Join(oci, "index.json")); err != nil || json.Unmarshal(b, &ix) != nil ||
		ix.SchemaVersion != 2 || ix.MediaType != mediaIndex || len(ix.Manifests) != 1 ||
		ix.Manifests[0].Annotations["org.opencontainers.image.ref.name"] != "1.2.3" || ix.Manifests[0].Digest != l.Digest {
		t.Fatalf("index.json: %+v, %v", ix, err)
	}
	var m manifest
	if err := json.Unmarshal(blob(ix.Manifests[0], mediaManifest), &m); err != nil || m.SchemaVersion != 2 ||
		m.MediaType != mediaManifest || len(m.Layers) != 2 || m.Annotations["org.opencontainers.image.source"] != repository ||
		m.Annotations["org.opencontainers.image.version"] != "1.2.3" ||
		m.Annotations["org.opencontainers.image.title"] != s.name {
		t.Fatalf("manifest: %+v, %v", m, err)
	}
	cb := blob(m.Config, mediaConfig)
	var cfg imageConfig
	if err := json.Unmarshal(cb, &cfg); err != nil || cfg.OS != "linux" || cfg.Architecture != "amd64" ||
		cfg.RootFS.Type != "layers" || len(cfg.RootFS.DiffIDs) != 2 ||
		!maps(cfg.Config.Labels, m.Annotations) {
		t.Fatalf("config: %s, %v", cb, err)
	}
	for _, no := range []string{"created", "Cmd", "Entrypoint", "history"} {
		if bytes.Contains(cb, []byte(`"`+no+`"`)) {
			t.Errorf("config: %s", no)
		}
	}
	for i, d := range m.Layers {
		name := l.Files[i].Path
		zr, err := gzip.NewReader(bytes.NewReader(blob(d, mediaLayer)))
		if err != nil || !zr.ModTime.IsZero() || zr.Name != "" {
			t.Fatalf("layer %d: %v, %+v", i, err, zr.Header)
		}
		tb, err := io.ReadAll(zr)
		if err != nil || digest(tb) != cfg.RootFS.DiffIDs[i] {
			t.Fatalf("layer %d: %v, diff_id %s", i, err, cfg.RootFS.DiffIDs[i])
		}
		tr := tar.NewReader(bytes.NewReader(tb))
		h, err := tr.Next()
		if err != nil || h.Name != name || h.Typeflag != tar.TypeReg || h.Mode != 0o444 || h.Uid != 0 || h.Gid != 0 ||
			h.ModTime.Unix() != 0 {
			t.Fatalf("layer %d: %+v, %v", i, h, err)
		}
		if b, err := io.ReadAll(tr); err != nil || string(b) != files[name] {
			t.Fatalf("layer %d: %q, %v", i, b, err)
		}
		if _, err := tr.Next(); err != io.EOF {
			t.Fatalf("layer %d: more than its file: %v", i, err)
		}
	}

	// Made again from the same topic, the same bytes, every file.
	again, _ := build(t, bus(t, klines(t, dates...)...), s)
	if a, b := tree(t, dir), tree(t, again); !maps(a, b) {
		t.Errorf("made twice, two trees:\n%v\n%v", a, b)
	}
}

func maps(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || w != v {
			return false
		}
	}
	return true
}

// tree is every file under dir and its digest, by its path in dir.
func tree(t *testing.T, dir string) map[string]string {
	t.Helper()
	out := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, e os.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		rel, _ := filepath.Rel(dir, p)
		out[rel] = digest(b)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestCopies(t *testing.T) {
	ks := klines(t, dates...)
	// a copy of K-line 2, and one of K-line 4 of another value: each dropped, the first taken
	other := source(t, 4, dates[4], "1")
	b := bus(t, ks[0], ks[1], ks[2], ks[2], ks[3], ks[4], other, ks[5])
	dir, line := build(t, b, testSpec(t))
	if !strings.Contains(line, "6 K-lines read, 2 copies dropped (1 of another value)") {
		t.Errorf("the line %q", line)
	}
	if b, err := os.ReadFile(filepath.Join(dir, "files", "2026-11.jsonl")); err != nil || string(b) != want(t, 4, 3)+"\n" {
		t.Errorf("2026-11.jsonl: %q, %v", b, err)
	}
}

func TestFaults(t *testing.T) {
	ks := klines(t, dates...)
	s := testSpec(t)
	with := func(f func(*spec)) spec { c := s; f(&c); return c }
	for name, c := range map[string]struct {
		ks   []raw.KLine
		s    spec
		want string
	}{
		"an index not the next": {[]raw.KLine{ks[0], source(t, 2, dates[1], "1")}, s,
			topic + "/0@1: index 2, the next 1"},
		"a date not after the last": {[]raw.KLine{source(t, 0, dates[1], "1"), source(t, 1, dates[0], "1")}, s,
			topic + "/0@1: date " + fmt.Sprint(dates[0]) + ", not after the last"},
		"a range from before the topic": {klines(t, dates[1:]...), with(func(c *spec) { c.from = "2026-10-29" }),
			"its sessions 2026-10-30 to 2026-11-03 in America/New_York, which do not cover the range 2026-10-29 to 2026-11-02"},
		"a range to after the topic": {ks[:5], with(func(c *spec) { c.to = "2026-11-03" }),
			"its sessions 2026-10-29 to 2026-11-02 in America/New_York, which do not cover the range 2026-10-30 to 2026-11-03"},
		"a range with no K-line": {ks, with(func(c *spec) { c.from, c.to = "2026-11-01", "2026-11-01" }),
			"no K-line in the sessions 2026-11-01 to 2026-11-01"},
		"no record": {nil, s, topic + ": no record"},
	} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "out")
			err := run(context.Background(), bus(t, c.ks...), c.s, dir, io.Discard)
			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("%v; want %q", err, c.want)
			}
			if _, err := os.Lstat(dir); err == nil {
				t.Errorf("a folder written")
			}
		})
	}
	// a folder there already: refused before the topic is read
	dir := t.TempDir()
	if err := run(context.Background(), bus(t, ks...), s, dir, io.Discard); err == nil ||
		!strings.Contains(err.Error(), "a folder that does not exist") {
		t.Errorf("a folder there: %v", err)
	}
}

func TestParse(t *testing.T) {
	good := []string{topic, "America/New_York", "2026-10-30", "2026-11-02", "qi-v2-platform-klines-test", "1.2.3",
		"ghcr.io/owner/qi-v2-platform-klines-test"}
	for name, c := range map[string]struct {
		at int
		v  string
	}{
		"a topic not clean":           {0, "market.raw"},
		"a zone not known":            {1, "America/Nowhere"},
		"the local zone":              {1, "Local"},
		"a date not YYYY-MM-DD":       {2, "2026-1-5"},
		"a date not of the calendar":  {3, "2026-02-30"},
		"from after to":               {2, "2026-11-03"},
		"a name in capitals":          {4, "Klines"},
		"a name of two separators":    {4, "a--b"},
		"a version not of three":      {5, "1.0"},
		"a version of a leading 0":    {5, "01.0.0"},
		"an image of another name":    {6, "ghcr.io/owner/other"},
		"an image of no registry":     {6, "qi-v2-platform-klines-test"},
		"an image of a longer name":   {6, "ghcr.io/owner/xqi-v2-platform-klines-test"},
		"an image of its name before": {6, "ghcr.io/qi-v2-platform-klines-test/x"},
	} {
		v := slices.Clone(good)
		v[c.at] = c.v
		if _, err := parse(v[0], v[1], v[2], v[3], v[4], v[5], v[6]); err == nil {
			t.Errorf("%s: %v parsed", name, v)
		}
	}
	s, err := parse(good[0], good[1], good[2], good[3], good[4], good[5], good[6])
	if err != nil || s.loc.String() != "America/New_York" {
		t.Fatalf("%+v, %v", s, err)
	}
	// the zone's dates, not UTC's: 2026-10-31 23:30 in New York is 2026-11-01 in UTC
	if d := session(market.KLine{Date: dates[3]}, s.loc); d != "2026-10-31" {
		t.Errorf("session %s", d)
	}
}
