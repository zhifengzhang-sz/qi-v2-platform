// Package massive reads Massive's aggregates (formerly Polygon.io's; Custom Bars, GET /v2/aggs/ticker/…/range/…), on
// market's Go form and the standard library alone (../README.md), for any app, the program around it thin.
//
//   - The schema (aggregates.schema.json, Schema, CheckPage): Massive's aggregates response as a JSON Schema, checked
//     against Massive's published spec (its OpenAPI document, the operation GetStocksAggregates), which it cites; a
//     page is checked against it, a name at most once in an object.
//   - The fetcher (Fetcher): the pages of a request (Request: a series and its spans) asked of Massive, the key read
//     from its file (KeyFile) and sent in the Authorization header alone, Massive's paging followed on its own host,
//     the calls spaced by the plan's limit (Interval) and a 429 or a 5xx waited out; each answer of 200 saved as
//     Massive sent it, but its request_id, before it is checked or read, a page a file named by its content in its
//     series' folder (PagesDir by default), so that a page is kept once however many fetches see it, each fetch a
//     line of the folder's log of fetches.
//   - Massive's K-line (KLine, Source, Text, ReadKLine): the base K-line of market 1.1.0 and Massive's own vw and n,
//     each optional, as Massive's spec has its row; its canonical text the base's rules, a field not set left out,
//     written and read with market's means for a record of a taker's own; its base held to market's vectors.
//   - The reader (ReadPage): a page in, its bars out, each Massive's K-line, every field of its row kept; a page that
//     breaks the schema, or a row with a field the K-line has no place for (otc), refuses the page, whole, never
//     dropping a row or a field.
//   - The bars of a request (Bars): its pages' bars in its spans and closed, ascending, as lines indexed from 0.
package massive
