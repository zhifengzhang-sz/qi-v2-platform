// The charting consumer's page (docs/designs/design-chart-consumer-2026-11-09.md, The feed and the page): the program's
// feed drawn by @qi/chart as its README's two components are used together (@qi/chart's README, Using it). At a start,
// the batch: each K-line read by @qi/market, the chart's components (the series' title, the scale, the K-lines as the
// x-axis, a run of no record, no add-ons, no options), its tv engine's page data given as CHART and LAYERS with the
// page's PALETTE; the first start loads the engine's script, which draws them and gives UPDATE, REDRAW and CHART_UI,
// and a later one draws the chart whole again (REDRAW, then the view on the newest K-lines, since REDRAW keeps the view
// by its places). At a K-line, the online component: complete, then UPDATE with the step's tail. A fault is shown on
// the page and on the console, the chart left as it was, and nothing more is stepped until the next start.

import { type Components, complete, type Online, online, tvChart, tvTail } from "@qi/chart";
import { decodeAs, type KLine, klineSchema } from "@qi/market";

// The palette (@qi/chart's README, The page and The palette's names): a colour, light then dark, of each name the
// library uses, every name of its table; a chart of K-lines and their volume draws with the first six. The package
// holds no colours: they are the page's.
const palette: Readonly<Record<string, readonly [string, string]>> = {
  "cl-bg": ["#ffffff", "#131722"],
  "cl-ink": ["#1f2328", "#d1d4dc"],
  "cl-muted": ["#8c8f99", "#7a7e8a"],
  "cl-state-fill": ["#eef0f4", "#232733"],
  "cl-up": ["#15803d", "#22c55e"],
  "cl-down": ["#c2410c", "#f97316"],
  "cl-stroke": ["#2563eb", "#60a5fa"],
  "cl-level": ["#7c3aed", "#a78bfa"],
  "cl-center": ["#b45309", "#fbbf24"],
  "cl-event": ["#0f766e", "#2dd4bf"],
  "cl-gap": ["#be185d", "#f472b6"],
};

// the prices' scale: market's precision, 6 digits after the point (the design, A record of the clean topic)
const scale = 1_000_000;

// a run of 缠论's contract with no record: a chart of the K-lines alone
const none = { merged: [], fractals: [], strokes: [], segments: [], moves: [], consolidations: [] };

// What the page keeps for the checks to read: the starts drawn, the K-lines' texts it has taken, each step's cost in
// milliseconds, the faults.
type State = { starts: number; klines: string[]; costs: number[]; faults: string[] };

type Ui = { readonly latest: () => void; readonly here: () => string };
type Page = typeof globalThis & {
  CHART?: unknown;
  LAYERS?: unknown;
  PALETTE?: unknown;
  UPDATE?: (tail: unknown) => void;
  REDRAW?: () => void;
  CHART_UI?: Ui;
  CHART_PAGE?: State;
};

const page = globalThis as Page;
const state: State = { starts: 0, klines: [], costs: [], faults: [] };
page.CHART_PAGE = state;
page.PALETTE = palette;

// the chart so far, once a start has drawn it and while no fault has stopped it
const at: { online: Online | undefined } = { online: undefined };

const line = (): HTMLElement | null => document.getElementById("fault");

const fault = (what: string): void => {
  at.online = undefined;
  state.faults.push(what);
  console.error(`chart: ${what}`);
  const l = line();
  if (l !== null) {
    l.textContent = `chart: ${what}`;
    l.style.display = "block";
  }
};

// a K-line's text read as market's K-line, or the fault
const read = (text: string): KLine | string => {
  const k = decodeAs(klineSchema, text);
  return k.ok ? k.value : `${text}: ${k.error}`;
};

// the engine's script, loaded once
const engine = (): Promise<void> =>
  new Promise((done, failed) => {
    const s = document.createElement("script");
    s.src = "/tv.js";
    s.onload = () => done();
    s.onerror = () => failed(new Error("/tv.js not loaded"));
    document.body.appendChild(s);
  });

// A start: the chart drawn whole from the last window of K-lines.
const begin = async (data: string): Promise<void> => {
  const s = JSON.parse(data) as { title: string; klines: string[] };
  const ks = s.klines.map(read);
  const bad = ks.find((k) => typeof k === "string");
  if (bad !== undefined) return fault(bad as string);
  const c: Components = { title: s.title, scale, xAxis: ks as KLine[], run: none, addons: [] };
  const d = tvChart(c, "all");
  if (d.tag === "failure") return fault(d.error.message);
  const o = online(c);
  if (o.tag === "failure") return fault(o.error.message);
  page.CHART = d.value.chart;
  page.LAYERS = d.value.layers;
  document.title = s.title;
  const faults = state.faults.length;
  if (page.REDRAW === undefined) {
    await engine();
  } else {
    page.REDRAW();
    page.CHART_UI?.latest();
  }
  if (state.faults.length > faults) return; // the engine's script failed as it drew: its fault stands
  at.online = o.value;
  state.starts++;
  state.klines = [...s.klines];
  const l = line();
  if (l !== null) l.style.display = "none";
};

// A K-line: the next, stepped onto the chart by the online component.
const step = (text: string): void => {
  const o = at.online;
  if (o === undefined) return; // no chart, or a fault: nothing stepped until the next start
  const t0 = performance.now();
  const k = read(text);
  if (typeof k === "string") return fault(k);
  const s = complete(o, k);
  if (s.tag === "failure") return fault(s.error.message);
  const tail = tvTail(s.value);
  if (tail.tag === "failure") return fault(tail.error.message);
  if (page.UPDATE === undefined) return fault("no UPDATE: the engine's script has not drawn the chart");
  page.UPDATE(tail.value);
  at.online = s.value.online;
  state.klines.push(text);
  state.costs.push(performance.now() - t0);
};

// The events taken one after another, the engine's script loaded before any K-line is stepped.
const queue: { last: Promise<void> } = { last: Promise.resolve() };
const then = (f: () => void | Promise<void>): void => {
  queue.last = queue.last.then(f).catch((e: unknown) => fault(e instanceof Error ? e.message : String(e)));
};

window.addEventListener("error", (e) => fault(e.message));
window.addEventListener("unhandledrejection", (e) => fault(String(e.reason)));

const feed = new EventSource("/feed");
feed.addEventListener("start", (e) => then(() => begin((e as MessageEvent<string>).data)));
feed.addEventListener("kline", (e) => then(() => step((e as MessageEvent<string>).data)));
