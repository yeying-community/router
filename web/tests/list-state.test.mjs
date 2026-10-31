import assert from 'node:assert/strict';
import {
  buildQueryFilterSignature,
  resolveListEffectPage,
} from '../src/helpers/listState.mjs';

const logFilterKeys = [
  'log_type',
  'type',
  'start_timestamp',
  'end_timestamp',
  'token_name',
  'model_name',
  'channel',
  'group_id',
  'username',
];

const baseSignature = buildQueryFilterSignature(
  '?order_by=created_at&order=desc&channel=channel-1',
  logFilterKeys
);
const pagedSignature = buildQueryFilterSignature(
  '?order_by=created_at&order=desc&channel=channel-1&page=2&page_size=50&tab=logs&source=quota',
  logFilterKeys
);

assert.equal(
  pagedSignature,
  baseSignature,
  'pagination, sorting and host-navigation params must not change the filter signature'
);
assert.notEqual(
  buildQueryFilterSignature('?channel=channel-2&page=2', logFilterKeys),
  baseSignature,
  'a real filter change must change the signature'
);
assert.equal(
  buildQueryFilterSignature('?channel=%20channel-1%20', logFilterKeys),
  baseSignature,
  'equivalent trimmed filter values should have the same signature'
);

assert.equal(
  resolveListEffectPage('same-request', 'same-request', 3),
  3,
  'mount and StrictMode mount replay should restore the URL page'
);
assert.equal(
  resolveListEffectPage('old-filter', 'new-filter', 3),
  1,
  'a real request-filter change should reset pagination'
);
assert.equal(
  resolveListEffectPage('same-request', 'same-request', 0),
  1,
  'invalid initial pages should normalize to page 1'
);

console.log('list state tests passed');
