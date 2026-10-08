// node --test infra/cloudflare/coord-round.test.js
//
// Loads the Worker with addEventListener and fetch stubbed, so the rewrite can
// be asserted without a Workers runtime.
import { test } from "node:test"
import assert from "node:assert/strict"
import { readFileSync } from "node:fs"

let handler
const fetched = []

globalThis.addEventListener = (_type, fn) => { handler = fn }
globalThis.fetch = (req) => { fetched.push(req.url); return "response" }

new Function(readFileSync(new URL("coord-round.js", import.meta.url), "utf8"))()

const rewrite = (url) => {
  fetched.length = 0
  handler({ request: new Request(url), respondWith: () => {} })

  return fetched[0]
}

test("rounds both coordinates to two decimals", () => {
  assert.equal(
    rewrite("https://api.tenkinow.com/v1/weather/latest?lat=35.68123&lon=139.76543"),
    "https://api.tenkinow.com/v1/weather/latest?lat=35.68&lon=139.77")
})

test("collapses nearby coordinates onto one cache key", () => {
  const a = rewrite("https://api.tenkinow.com/v1/weather/latest?lat=35.6812&lon=139.7601")
  const b = rewrite("https://api.tenkinow.com/v1/weather/latest?lat=35.6819&lon=139.7604")
  assert.equal(a, b)
})

test("pads short coordinates so 35.6 and 35.60 share a key", () => {
  assert.equal(
    rewrite("https://api.tenkinow.com/v1/weather/latest?lat=35.6&lon=139.7"),
    rewrite("https://api.tenkinow.com/v1/weather/latest?lat=35.60&lon=139.70"))
})

test("leaves unparseable coordinates for the handler to reject", () => {
  assert.equal(
    rewrite("https://api.tenkinow.com/v1/weather/latest?lat=abc&lon=139.76"),
    "https://api.tenkinow.com/v1/weather/latest?lat=abc&lon=139.76")
})

test("passes through a request with no coordinates untouched", () => {
  assert.equal(
    rewrite("https://api.tenkinow.com/v1/weather/now"),
    "https://api.tenkinow.com/v1/weather/now")
})
