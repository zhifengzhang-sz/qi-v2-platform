# The platform's library

`lib/`, a Go module of its own (`github.com/zhifengzhang-sz/qi-v2-platform/lib`): what reads a source, or does any other
work an app can share, as packages any app can use, the platform's or a user's, with the program around it thin. It
takes market's Go form (market 1.1.0, the flake's package `market-go`, required at `v0.0.0` and replaced in `go.work`)
and the standard library, and nothing else; `TestImports` checks it on every `make check`.

| Package | What it is |
|---|---|
| `massive` | Massive's reader: Massive's K-line, the base K-line of market 1.1.0 and Massive's `vw` and `n`, each optional, its text written and read; Massive's aggregates response as a JSON Schema (`aggregates.schema.json`, checked against Massive's published spec), a page checked against it; the fetcher, a request's pages asked of Massive and each saved as Massive sent it but its `request_id`, named by its content and kept once; the reader, a page in and its bars out, each Massive's K-line in its canonical text, every field kept; the bars of a request's spans, closed, as lines indexed from 0 |

## `massive`

- **Massive's K-line.** `KLine`, the library's own record (market 1.1.0 leaves a source's K-line to its taker,
  market's SPEC.md, K-line): the base K-line's fields and rule, `Base` giving market's K-line, and Massive's own two,
  `vw` (the volume-weighted average price, a price) and `n` (the number of trades, a whole number from 0 to market's
  bound), each optional (`HasVw`, `HasN`; a field not set is 0), as Massive's spec has its row. `Source`, `massive`, is
  the source whose messages on the stream carry it (`../core/README.md`). Its canonical text is the base's rules: its
  fields in the order of their names, a field not set left out, no spaces, a number in its digits (`Text`, then a line
  break). Its JSON is read by `ReadKLine` under the rules of market's JSON: the base's seven fields once each, `vw` and
  `n` once if there, nothing else, a whole number in digits within the bound, a decimal by market's `ParseDecimal`, no
  `null`, no other kind, text UTF-8, then the rules (`Check`). Both are written with market's means for a record of a
  taker's own (`Base`, `Check`, `Decimal`, `ParseDecimal`, `AppendTo`), and its base is held to market's vectors
  (`TestVectors`, the vectors at `QI_MARKET_VECTORS`, the shell's): `kline.json`'s K-line read with neither `vw` nor
  `n` and written back as its text; every reject vector refused, but those that break only `asOf`, which a K-line has
  not, each read as `kline.json`'s K-line once its `asOf` is taken out. The stores' writers read a message's K-line of
  the source `massive` with it (`../consumers/`).
- **The schema.** `aggregates.schema.json`, JSON Schema 2020-12, checked against Massive's published spec, its OpenAPI
  document (https://api.massive.com/openapi, read without a key), the operation `GetStocksAggregates`, its answer 200's
  schema, an `allOf` of four objects held here as one: each of their properties a property, of the spec's type, its
  description the spec's (and the documentation's of Custom Bars, from which it was first written,
  https://massive.com/docs/rest/stocks/aggregates/custom-bars.md); required the spec's but `request_id`, which no saved
  page holds; `results` not required, Massive leaving it off a page of no row; a row requiring `o`, `h`, `l`, `c`, `v`
  and `t`, the spec's, so that `vw` and `n` may be missing; the spec's `format` `double` of a row's prices and volume
  not kept, a number read exactly from its digits. The schema's `$comment` says so. Other attributes are allowed
  (Massive's pages carry `count`, which the spec does not name). The check knows
  the keywords the schema uses (`type`, `properties`, `required`, `items` and the annotations) and refuses a schema with
  any other, so that none goes unchecked; it reads a number's type from its digits, and refuses a name twice in one
  object.
- **The fetcher.** `Fetcher.Fetch` asks Massive once for the whole of a request's spans, unadjusted, ascending, 50,000
  rows a page, following `next_url` on Massive's host alone; its calls spaced by its `Interval` (the free plan's five a
  minute, 12 s, is `massive.Interval`; none when 0), a 429 or a 5xx waited out (`Retry-After`, else a minute) and asked
  again. The free plan gives no bar of the current trading day at any hour: a day's bars, its minutes included, are
  given once midnight Eastern Time has passed, and a request for the current day before then is refused, HTTP 403
  `NOT_AUTHORIZED` (Massive's knowledge base, "On the free Basic plan, when can I pull the previous trading day's
  data?", https://massive.com/knowledge-base/article/on-the-free-basic-plan-when-can-i-pull-the-previous-trading-days-data).
  The key is read from its file (`massive.KeyFile()`, `~/.config/qi/massive_key`, one line) when a fetch starts
  and sent in the `Authorization` header alone: no URL, page, log line or error carries it, and an answer carrying it is
  not saved and stops the fetch. Each answer of 200 is saved before it is checked or read, as Massive sent it but its
  `request_id` (the member and its comma taken out, every other byte as sent), in its series' folder,
  `~/.local/share/qi/massive/pages/INSTRUMENT/TIMEFRAME/` (`massive.PagesDir()`), outside any repository, named by its
  content, `HASH.json`, `HASH` the SHA-256 of its bytes in hex: a page kept already is not saved again, so that a page
  is kept once however many fetches see it, and a file of its name holding other bytes stops the fetch, never
  replaced. A page is written to a file of its own and synced, then linked to its name. Each fetch that took a page is
  a line of the folder's log, `fetches`, once it ends: its start (UTC, `20061102T150405.000Z`), its `FROM` and `TO`,
  and its pages' names in order; `Fetch` gives them too, and how many it saved. An answer whose `request_id` cannot be
  taken out (twice, in bytes not one JSON object, or anywhere but the top) is not saved and stops the fetch, so that no
  saved page holds one. An answer but 200 is a refusal, not saved; a refusal is named by Massive's error and message,
  else its body, its `request_id` taken out.
- **The reader.** `ReadPage`: the page checked against the schema, its status `OK` or `DELAYED`, each row read as
  Massive's K-line, each name as it is written, its case too: `t` as `date`; `o`, `h`, `l`, `c`, `v`; `vw` and `n`, each
  set when the row has it; the numbers read exactly from Massive's digits into market's decimals. The page's other
  attributes (`ticker`, `count`, …) are not read; the saved page keeps them. A page that breaks the schema, and a row
  that is not such a K-line (`otc` or any field it has no place for, more than six digits after the point, a number
  past market's bounds, a broken rule), refuse the page, whole: nothing is dropped or changed to fit.
- **The bars of a request.** `Bars` takes a request's pages in order and gives the bars in its spans and closed,
  ascending (rows not ascending stop it), each as a line, its `index` its place from 0; the rows outside the spans and
  those not closed are counted.

Its tests run on pages the fetcher saved (`massive/testdata`: SPY's first six minutes of 2026-10-23, three rows a page,
and a page of no row), those pages without `vw` and `n`, and market's vectors.
