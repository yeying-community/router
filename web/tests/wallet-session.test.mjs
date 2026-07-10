import assert from 'node:assert/strict';
import {
  ACCESS_TOKEN_RENEW_SKEW_MS,
  getAccessTokenExpiresAt,
  isAccessTokenFresh,
} from '../src/helpers/walletSession.mjs';

function createToken(expiresAtMs) {
  const encode = (value) =>
    Buffer.from(JSON.stringify(value))
      .toString('base64url')
      .replace(/=+$/, '');
  return `${encode({ alg: 'HS256', typ: 'JWT' })}.${encode({
    exp: Math.floor(expiresAtMs / 1000),
  })}.signature`;
}

const now = 1_800_000_000_000;
const freshToken = createToken(now + ACCESS_TOKEN_RENEW_SKEW_MS + 60_000);
const expiringToken = createToken(now + ACCESS_TOKEN_RENEW_SKEW_MS - 1_000);

assert.equal(
  getAccessTokenExpiresAt(freshToken),
  Math.floor((now + ACCESS_TOKEN_RENEW_SKEW_MS + 60_000) / 1000) * 1000,
);
assert.equal(
  isAccessTokenFresh(freshToken, now),
  true,
  'wallet disconnect should preserve a still-valid Router session',
);
assert.equal(
  isAccessTokenFresh(expiringToken, now),
  false,
  'a token near expiry should be refreshed before preserving the session',
);
assert.equal(
  isAccessTokenFresh('not-a-jwt', now),
  false,
  'malformed tokens must not be treated as active sessions',
);

console.log('wallet session tests passed');
