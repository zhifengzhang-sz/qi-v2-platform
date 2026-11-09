package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/zhifengzhang-sz/qi-v2-platform/core/raw"
	"github.com/zhifengzhang-sz/qi-v2-platform/core/stream/mem"
)

// The browser check (design-chart-consumer, How it is checked): the page in headless Chromium, driven by its DevTools
// protocol over --remote-debugging-pipe with the standard library alone, a process and two pipes. Chromium is
// nixpkgs' at the flake's pin and the page the flake's chart-page, each given by a variable, QI_CHROMIUM (its
// bin/chromium) and QI_CHART_PAGE (the package's folder); without them the tests skip, saying so. The screenshots go
// to QI_CHART_OUT, the run's folder (make check-chart's, .reports/chart-check), else to a folder of the test's own,
// removed after it.

// message is one of the protocol's: a command's answer (ID), or an event (Method).
type message struct {
	ID        int             `json:"id,omitempty"`
	Method    string          `json:"method,omitempty"`
	Params    json.RawMessage `json:"params,omitempty"`
	Result    json.RawMessage `json:"result,omitempty"`
	SessionID string          `json:"sessionId,omitempty"`
	Error     *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// browser is a headless Chromium of its own, its profile in a folder of its own, its page attached by one session.
type browser struct {
	cmd     *exec.Cmd
	exited  chan struct{}
	profile string
	in      *os.File // the protocol's commands, Chromium's fd 3
	mu      sync.Mutex
	next    int
	waiting map[int]chan message
	session string
	faults  []string // exceptions and errors on the page's console
	urls    []string // every request the page made
}

func launch(t *testing.T, chromium string) *browser {
	t.Helper()
	profile, err := os.MkdirTemp("", "chart-chromium-")
	if err != nil {
		t.Fatal(err)
	}
	r3, w3, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	r4, w4, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(chromium, "--headless=new", "--no-sandbox", "--disable-gpu", "--hide-scrollbars",
		"--no-first-run", "--no-default-browser-check", "--remote-debugging-pipe", "--window-size=1280,800",
		"--user-data-dir="+profile, "about:blank")
	cmd.ExtraFiles = []*os.File{r3, w4} // fd 3, what it reads; fd 4, what it writes
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	r3.Close()
	w4.Close()
	b := &browser{cmd: cmd, exited: make(chan struct{}), profile: profile, in: w3, waiting: map[int]chan message{}}
	go func() { cmd.Wait(); close(b.exited) }()
	go b.read(r4)
	t.Cleanup(func() { b.close(t) })
	var target struct{ TargetID string }
	if err := b.call("", "Target.createTarget", map[string]any{"url": "about:blank"}, &target); err != nil {
		t.Fatal(err)
	}
	var attached struct{ SessionID string }
	if err := b.call("", "Target.attachToTarget", map[string]any{"targetId": target.TargetID, "flatten": true},
		&attached); err != nil {
		t.Fatal(err)
	}
	b.session = attached.SessionID
	for _, m := range []string{"Page.enable", "Runtime.enable", "Network.enable", "Log.enable"} {
		if err := b.call(b.session, m, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
	return b
}

// read takes the protocol's messages, each ended by a NUL: an answer to its command, an event kept as the page's.
func (b *browser) read(out *os.File) {
	r := bufio.NewReader(out)
	for {
		raw, err := r.ReadBytes(0)
		if err != nil {
			return
		}
		var m message
		if json.Unmarshal(raw[:len(raw)-1], &m) != nil {
			continue
		}
		b.mu.Lock()
		if m.ID != 0 {
			if c := b.waiting[m.ID]; c != nil {
				c <- m
				delete(b.waiting, m.ID)
			}
		} else {
			b.event(m)
		}
		b.mu.Unlock()
	}
}

// event keeps what the checks read of the page's events: its exceptions, its console's errors, its requests.
func (b *browser) event(m message) {
	switch m.Method {
	case "Runtime.exceptionThrown":
		var p struct {
			ExceptionDetails struct {
				Text      string
				Exception struct{ Description string }
			}
		}
		json.Unmarshal(m.Params, &p)
		b.faults = append(b.faults, "exception: "+p.ExceptionDetails.Text+" "+p.ExceptionDetails.Exception.Description)
	case "Runtime.consoleAPICalled":
		var p struct {
			Type string
			Args []struct {
				Value       any
				Description string
			}
		}
		json.Unmarshal(m.Params, &p)
		if p.Type == "error" || p.Type == "assert" {
			b.faults = append(b.faults, fmt.Sprintf("console.%s: %v", p.Type, p.Args))
		}
	case "Log.entryAdded":
		var p struct {
			Entry struct{ Level, Text, URL string }
		}
		json.Unmarshal(m.Params, &p)
		if p.Entry.Level == "error" {
			b.faults = append(b.faults, "log: "+p.Entry.Text+" "+p.Entry.URL)
		}
	case "Network.requestWillBeSent":
		var p struct{ Request struct{ URL string } }
		json.Unmarshal(m.Params, &p)
		b.urls = append(b.urls, p.Request.URL)
	}
}

// call sends a command and waits for its answer, 30 s at most.
func (b *browser) call(session, method string, params any, result any) error {
	b.mu.Lock()
	b.next++
	id := b.next
	c := make(chan message, 1)
	b.waiting[id] = c
	b.mu.Unlock()
	cmd := map[string]any{"id": id, "method": method}
	if params != nil {
		cmd["params"] = params
	}
	if session != "" {
		cmd["sessionId"] = session
	}
	text, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	if _, err := b.in.Write(append(text, 0)); err != nil {
		return fmt.Errorf("%s: %w", method, err)
	}
	select {
	case m := <-c:
		if m.Error != nil {
			return fmt.Errorf("%s: %s", method, m.Error.Message)
		}
		if result != nil {
			return json.Unmarshal(m.Result, result)
		}
		return nil
	case <-b.exited:
		return fmt.Errorf("%s: Chromium has exited", method)
	case <-time.After(30 * time.Second):
		return fmt.Errorf("%s: no answer in 30 s", method)
	}
}

// eval is an expression's value in the page, read into v.
func (b *browser) eval(expr string, v any) error {
	var r struct {
		Result           struct{ Value json.RawMessage }
		ExceptionDetails *struct{ Text string }
	}
	if err := b.call(b.session, "Runtime.evaluate", map[string]any{"expression": expr, "returnByValue": true,
		"awaitPromise": true}, &r); err != nil {
		return err
	}
	if r.ExceptionDetails != nil {
		return fmt.Errorf("%s: %s", expr, r.ExceptionDetails.Text)
	}
	return json.Unmarshal(r.Result.Value, v)
}

func (b *browser) open(t *testing.T, url string) {
	t.Helper()
	if err := b.call(b.session, "Page.navigate", map[string]any{"url": url}, nil); err != nil {
		t.Fatal(err)
	}
}

// shot is a screenshot of the page, written to path.
func (b *browser) shot(t *testing.T, path string) {
	t.Helper()
	var r struct{ Data string }
	if err := b.call(b.session, "Page.captureScreenshot", map[string]any{"format": "png"}, &r); err != nil {
		t.Fatal(err)
	}
	png, err := base64.StdEncoding.DecodeString(r.Data)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, png, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Logf("screenshot: %s", path)
}

// seen is what the page has done so far: its faults and the requests it made.
func (b *browser) seen() (faults, urls []string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.faults), slices.Clone(b.urls)
}

// close stops Chromium (Browser.close, then its process group killed if it has not exited in 10 s) and finds it gone:
// no process of its group, and none naming its profile.
func (b *browser) close(t *testing.T) {
	t.Helper()
	if b.cmd.Process == nil {
		return
	}
	b.call("", "Browser.close", nil, nil)
	select {
	case <-b.exited:
	case <-time.After(10 * time.Second):
		t.Error("Chromium not exited 10 s after Browser.close: its group killed")
		syscall.Kill(-b.cmd.Process.Pid, syscall.SIGKILL)
		<-b.exited
	}
	b.in.Close()
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(100 * time.Millisecond) {
		left := chromiumLeft(b.cmd.Process.Pid, b.profile)
		if len(left) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Errorf("Chromium left running 10 s after it exited: %v", left)
			syscall.Kill(-b.cmd.Process.Pid, syscall.SIGKILL)
			break
		}
	}
	os.RemoveAll(b.profile)
	b.cmd.Process = nil
}

// chromiumLeft is each process left of a Chromium: of its process group, or naming its profile.
func chromiumLeft(pgid int, profile string) []string {
	var out []string
	if err := syscall.Kill(-pgid, 0); err == nil || !errors.Is(err, syscall.ESRCH) {
		out = append(out, fmt.Sprintf("process group %d", pgid))
	}
	dirs, _ := os.ReadDir("/proc")
	for _, d := range dirs {
		if _, err := strconv.Atoi(d.Name()); err != nil {
			continue
		}
		cmdline, err := os.ReadFile(filepath.Join("/proc", d.Name(), "cmdline"))
		if err == nil && strings.Contains(string(cmdline), profile) {
			out = append(out, "pid "+d.Name())
		}
	}
	return out
}

// pageState is what the page keeps for the checks (page/main.ts, State).
type pageState struct {
	Starts int       `json:"starts"`
	KLines []string  `json:"klines"`
	Costs  []float64 `json:"costs"`
	Faults []string  `json:"faults"`
}

func (b *browser) state() (pageState, error) {
	var s pageState
	err := b.eval("window.CHART_PAGE ?? {starts: 0, klines: [], costs: [], faults: []}", &s)
	return s, err
}

// view is the last K-line in view, its date in seconds, from CHART_UI.here() (time=FROM-TO&off=…).
func (b *browser) view() (uint64, error) {
	var here string
	if err := b.eval("window.CHART_UI.here()", &here); err != nil {
		return 0, err
	}
	var from, to uint64
	if _, err := fmt.Sscanf(strings.SplitN(here, "&", 2)[0], "time=%d-%d", &from, &to); err != nil {
		return 0, fmt.Errorf("here %q: %w", here, err)
	}
	return to, nil
}

// waitFor is the page's state once ok holds, within a minute; the page's faults stop it.
func (b *browser) waitFor(t *testing.T, what string, ok func(pageState) bool) pageState {
	t.Helper()
	for deadline := time.Now().Add(time.Minute); ; time.Sleep(100 * time.Millisecond) {
		s, err := b.state()
		if err != nil {
			t.Fatal(err)
		}
		if faults, _ := b.seen(); len(faults) > 0 || len(s.Faults) > 0 {
			t.Fatalf("the page's faults: %q %q", faults, s.Faults)
		}
		if ok(s) {
			return s
		}
		if time.Now().After(deadline) {
			t.Fatalf("%s: not in a minute; the page: %d starts, %d K-lines", what, s.Starts, len(s.KLines))
		}
	}
}

// chromium is QI_CHROMIUM and the page QI_CHART_PAGE's files, or the test skipped.
func chromium(t *testing.T) (string, map[string][]byte) {
	t.Helper()
	c, p := os.Getenv("QI_CHROMIUM"), os.Getenv("QI_CHART_PAGE")
	if c == "" || p == "" {
		t.Skip("the browser check needs QI_CHROMIUM and QI_CHART_PAGE (make check-chart)")
	}
	page, err := load(p)
	if err != nil {
		t.Fatal(err)
	}
	return c, page
}

// outDir is the run's folder for the screenshots: QI_CHART_OUT, else one of the test's own.
func outDir(t *testing.T) string {
	t.Helper()
	d := os.Getenv("QI_CHART_OUT")
	if d == "" {
		return t.TempDir()
	}
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	return d
}

// dec is n units of 10^-places in market's text: no trailing 0 after the point.
func dec(n int64, places int) string {
	s := strconv.FormatInt(n, 10)
	for len(s) <= places {
		s = "0" + s
	}
	w, f := s[:len(s)-places], strings.TrimRight(s[len(s)-places:], "0")
	if f == "" {
		return w
	}
	return w + "." + f
}

// spy is a K-line at Massive's precision, SPY's kind: prices to 4 digits after the point, a volume to 6, n and vw;
// high moves it up by that many ten-thousandths.
func spy(t *testing.T, i int, high int64) raw.KLine {
	t.Helper()
	o := int64(7_590_000 + (i*137)%5_000)
	c := o + int64((i*53)%400) - 200
	h := max(o, c) + int64((i*17)%300) + high
	l := min(o, c) - int64((i*29)%300)
	k, err := raw.DecodeKLine([]byte(fmt.Sprintf(`{"close":%s,"date":%d,"high":%s,"index":%d,"low":%s,"n":%d,`+
		`"open":%s,"volume":%s,"vw":%s}`, dec(c, 4), day(i), dec(h, 4), i, dec(l, 4), 1000+i, dec(o, 4),
		dec(int64(50_000_000_000+i*123_457), 6), dec((o+c)/2, 4))))
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// The page in headless Chromium, its feed from the in-memory stream, the window 30: drawn whole from the batch with no
// exception and no fault, every request the program's; each K-line stepped, its texts the feed's; a start again at
// twice the window and one after the topic is made again (as at a reset: the program stopped, the topic made again,
// started again, the page's EventSource reconnecting), each drawn whole again, the K-lines in view its newest.
func TestPage(t *testing.T) {
	bin, page := chromium(t)
	out := outDir(t)
	bus := mem.New()
	s := series{topic: "market.clean.validation.v0-1-0.massive.SPY.M1", source: "massive", instrument: "SPY"}
	provision(t, bus, s.topic)
	ks := make([]raw.KLine, 90)
	for i := range ks {
		ks[i] = spy(t, i, 0)
	}
	publish(t, bus, s, ks[:60]...)
	o := options{topic: s.topic, name: "chart", window: 30}
	r := start(t, bus, o, page)
	r.log.wait(t, "serving")
	b := launch(t, bin)
	b.open(t, "http://"+r.addr+"/")
	same := func(got []string, want []raw.KLine) bool { return slices.Equal(got, texts(t, want...)) }
	newest := func(k raw.KLine) {
		t.Helper()
		v, err := b.view()
		if err != nil {
			t.Fatal(err)
		}
		if v != k.Base().Date/1000 {
			t.Fatalf("the last K-line in view at %d, the newest at %d", v, k.Base().Date/1000)
		}
	}

	st := b.waitFor(t, "the batch", func(p pageState) bool { return p.Starts == 1 })
	if !same(st.KLines, ks[30:60]) {
		t.Fatalf("the batch's K-lines %q, the feed's %q", st.KLines, texts(t, ks[30:60]...))
	}
	b.shot(t, filepath.Join(out, "1-batch.png"))

	publish(t, bus, s, ks[60:70]...)
	st = b.waitFor(t, "ten steps", func(p pageState) bool { return len(p.KLines) == 40 })
	if !same(st.KLines, ks[30:70]) || st.Starts != 1 || len(st.Costs) != 10 {
		t.Fatalf("after ten steps: %d starts, %d costs, K-lines %q", st.Starts, len(st.Costs), st.KLines)
	}
	newest(ks[69])
	t.Logf("ten steps of a chart of 31 to 40 K-lines: %.1f ms each at most", slices.Max(st.Costs))
	b.shot(t, filepath.Join(out, "2-steps.png"))

	publish(t, bus, s, ks[70:90]...)
	st = b.waitFor(t, "twice the window", func(p pageState) bool {
		return p.Starts == 2 && len(p.KLines) == 30 &&
			p.KLines[29] == text(ks[89].Base())
	})
	if !same(st.KLines, ks[60:90]) {
		t.Fatalf("drawn again at twice the window: %q", st.KLines)
	}
	newest(ks[89])
	b.shot(t, filepath.Join(out, "3-again.png"))

	// as at a reset: stopped, the topic made again with ten highs changed, started again where it served
	r.stop()
	if err := r.end(t); err != nil {
		t.Fatal(err)
	}
	adm, err := bus.Admin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := adm.Delete(context.Background(), s.topic); err != nil {
		t.Fatal(err)
	}
	provision(t, bus, s.topic)
	remade := make([]raw.KLine, 85)
	for i := range remade {
		up := int64(0)
		if i >= 70 && i < 80 {
			up = 100 // 0.01
		}
		remade[i] = spy(t, i, up)
	}
	publish(t, bus, s, remade...)
	again := startAt(t, r.addr, bus, o, page)
	again.log.wait(t, "serving")
	st = b.waitFor(t, "the topic made again", func(p pageState) bool { return p.Starts == 3 })
	if !same(st.KLines, remade[55:85]) {
		t.Fatalf("after the topic was made again: %q, want %q", st.KLines, texts(t, remade[55:85]...))
	}
	newest(remade[84])
	b.shot(t, filepath.Join(out, "4-remade.png"))

	_, urls := b.seen()
	for _, u := range urls {
		if !strings.HasPrefix(u, "http://"+r.addr+"/") {
			t.Fatalf("a request not to the program: %s (of %q)", u, urls)
		}
	}
	slices.Sort(urls)
	t.Logf("the page's requests: %q", slices.Compact(urls))
}

// TestLive is the browser check's driver on a running chart's page, for the checks before the path is called done:
// with QI_CHART_URL, the page's address, and QI_CHART_OUT, its folder, it opens the page in headless Chromium and,
// until it is stopped (SIGINT, SIGTERM), writes a line to live.log at each change of what the page holds (its starts,
// its K-lines and their first and last, its steps, its faults) and, each time the page has held the same for 3 s after
// a change, N.png, a screenshot, N.klines, the K-lines' texts it holds, and N.costs, each step's cost in ms. Run as the
// test binary itself (go test -c), so that a signal reaches it; Chromium is stopped at its end and found gone.
func TestLive(t *testing.T) {
	url, dir := os.Getenv("QI_CHART_URL"), os.Getenv("QI_CHART_OUT")
	bin := os.Getenv("QI_CHROMIUM")
	if url == "" || dir == "" || bin == "" {
		t.Skip("the driver on a running chart's page needs QI_CHART_URL, QI_CHART_OUT and QI_CHROMIUM")
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	logf, err := os.OpenFile(filepath.Join(dir, "live.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer logf.Close()
	b := launch(t, bin)
	b.open(t, url)
	say := func(format string, args ...any) {
		line := time.Now().UTC().Format("15:04:05.000Z ") + fmt.Sprintf(format, args...)
		fmt.Fprintln(logf, line)
		t.Log(line)
	}
	say("open %s", url)
	key, changed, n := "", time.Time{}, 0
	for ctx.Err() == nil {
		st, err := b.state()
		if err != nil {
			say("state: %v", err)
			time.Sleep(time.Second)
			continue
		}
		faults, _ := b.seen()
		first, last := "-", "-"
		if len(st.KLines) > 0 {
			first, last = index(st.KLines[0]), index(st.KLines[len(st.KLines)-1])
		}
		k := fmt.Sprintf("%d starts, %d K-lines (%s to %s), %d steps, %d faults %q %q", st.Starts, len(st.KLines),
			first, last, len(st.Costs), len(faults)+len(st.Faults), st.Faults, faults)
		if k != key {
			key, changed = k, time.Now()
			say("%s", k)
		} else if !changed.IsZero() && time.Since(changed) > 3*time.Second {
			n++
			b.shot(t, filepath.Join(dir, fmt.Sprintf("%d.png", n)))
			os.WriteFile(filepath.Join(dir, fmt.Sprintf("%d.klines", n)), []byte(strings.Join(st.KLines, "\n")+"\n"), 0o644)
			costs := make([]string, len(st.Costs))
			for i, c := range st.Costs {
				costs[i] = strconv.FormatFloat(c, 'f', 1, 64)
			}
			os.WriteFile(filepath.Join(dir, fmt.Sprintf("%d.costs", n)), []byte(strings.Join(costs, "\n")+"\n"), 0o644)
			say("settled: %d.png, %d.klines, %d.costs", n, n, n)
			changed = time.Time{}
		}
		select {
		case <-ctx.Done():
		case <-time.After(500 * time.Millisecond):
		}
	}
	say("stopped")
}
