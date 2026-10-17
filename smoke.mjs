import assert from 'node:assert/strict';
import {randomUUID} from 'node:crypto';

const base = 'http://retail-order-service.localhost';
const token = process.env.ORDER_TEST_TOKEN;
assert.ok(token?.length >= 32, 'checkout test credential required');
const repository = process.env.ORDER_TEST_REPOSITORY;
assert.ok(repository, 'expected build repository required; run through setup.sh smoke');
const orders = [];

async function request(path, body, authorized = true) {
  const response = await fetch(base + path, {
    method: body === undefined ? 'GET' : 'POST',
    headers: {'Content-Type': 'application/json', ...(authorized ? {Authorization: 'Bearer ' + token} : {})},
    body: body === undefined ? undefined : JSON.stringify(body),
    signal: AbortSignal.timeout(15000), redirect: 'error',
  });
  const text = await response.text();
  let data; try { data = JSON.parse(text); } catch { data = null; }
  return {status: response.status, headers: response.headers, data};
}

const build = await request('/build-info');
assert.equal(build.status, 200);
assert.equal(build.data.repository, repository);
assert.match(build.data.revision, /^build-[a-f0-9]{64}$/);
assert.ok(build.data.pod_uid);
const start = new Date(Date.parse(build.headers.get('date')) - 1000).toISOString();
assert.equal((await request('/orders', {}, false)).status, 401);
const catalogue = await request('/catalog');
assert.equal(catalogue.status, 200);
assert.equal(catalogue.data.length, 3);

for (const input of [
  {sku: 'COFFEE-250', quantity: 1},
  {sku: 'FILTER-100', quantity: 3},
  {sku: 'COFFEE-250', quantity: 1, promotion_code: 'WELCOME100'},
]) {
  const body = {order_id: randomUUID(), ...input};
  const created = await request('/orders', body);
  assert.equal(created.status, 201);
  assert.equal(created.data.order_id, body.order_id);
  assert.equal(created.data.operation, 'create_order');
  assert.equal(created.data.revision, build.data.revision);
  assert.match(created.data.target_trace_id, /^[a-f0-9]{32}$/);
  assert.equal(created.headers.get('x-request-id'), created.data.observation_id);
  const product = catalogue.data.find(p => p.sku === body.sku);
  const total = product.unit_price_cents * body.quantity - (body.promotion_code ? 100 : 0);
  assert.equal(created.data.ledger_total_cents, total);
  const stored = await request('/orders/' + body.order_id);
  assert.equal(stored.status, 200);
  assert.equal(stored.data.ledger.total, total);
  assert.equal(stored.data.ledger.writes, 1);
  const replay = await request('/orders', body);
  assert.equal(replay.status, 200);
  assert.equal(replay.headers.get('idempotency-replayed'), 'true');
  assert.equal(replay.data.ledger_total_cents, total);
  assert.notEqual(replay.data.observation_id, created.data.observation_id);
  assert.deepEqual((await request('/orders/' + body.order_id)).data, stored.data);
  const conflict = await request('/orders', {...body, quantity: body.quantity + 1});
  assert.equal(conflict.status, 409);
  assert.equal(conflict.data.error_code, 'order_id_conflict');
  assert.equal((await request('/evidence/' + conflict.data.observation_id)).data.ledger_total_cents, undefined);
  const evidence = await request('/evidence/' + created.data.observation_id);
  assert.equal(evidence.status, 200);
  assert.equal(evidence.data.ledger_total_cents, total);
  assert.equal(evidence.data.ledger_writes, 1);
  orders.push({order_id: body.order_id, observation_id: created.data.observation_id, trace_id: created.data.target_trace_id});
}
const concurrent = {order_id: randomUUID(), sku: 'MUG-350', quantity: 2};
const responses = await Promise.all(Array.from({length: 8}, () => request('/orders', concurrent)));
assert.equal(responses.filter(r => r.status === 201).length, 1);
assert.equal(responses.filter(r => r.status === 200).length, 7);
assert.equal((await request('/orders/' + concurrent.order_id)).data.ledger.writes, 1);
for (const body of [
  {order_id: randomUUID(), sku: 'COFFEE-250', quantity: 0},
  {order_id: randomUUID(), sku: 'UNKNOWN', quantity: 1},
  {order_id: randomUUID(), sku: 'COFFEE-250', quantity: 1, promotion_code: 'UNKNOWN'},
]) assert.equal((await request('/orders', body)).status, 422);
assert.equal((await request('/orders', {order_id: randomUUID(), sku: 'COFFEE-250', quantity: 1, unit_price_cents: 1})).status, 400);
// Use the service's clock; a workstation can be slightly ahead of the Kind VM.
let feed;
for (let attempt = 0; attempt < 10; attempt++) {
  await new Promise(resolve => setTimeout(resolve, 250));
  const clock = await request('/build-info');
  assert.equal(clock.status, 200);
  const end = new Date(clock.headers.get('date')).toISOString();
  feed = await request('/observations?' + new URLSearchParams({start, end, limit: '50'}));
  if (feed.status === 200 && orders.every(order => feed.data.candidates.some(c => c.observation_id === order.observation_id))) break;
}
assert.equal(feed.status, 200);
assert.equal(feed.data.truncated, false);
for (const order of orders) assert.ok(feed.data.candidates.some(c => c.observation_id === order.observation_id && c.http_status === 201));
console.log('PASS: catalogue, order persistence, replay, conflict, concurrent duplicate prevention, validation, evidence and observation feed.');
console.log(JSON.stringify({build: build.data.revision, source_matches_commit: build.data.source_matches_commit, orders}, null, 2));
