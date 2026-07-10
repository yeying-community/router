export const ACCESS_TOKEN_RENEW_SKEW_MS = 5 * 60 * 1000;

function decodeBase64Url(value) {
  if (typeof value !== 'string' || typeof globalThis.atob !== 'function') {
    return '';
  }
  const normalized = value.replace(/-/g, '+').replace(/_/g, '/');
  const padded = normalized.padEnd(Math.ceil(normalized.length / 4) * 4, '=');
  return globalThis.atob(padded);
}

export function getAccessTokenExpiresAt(token) {
  if (typeof token !== 'string' || token.trim() === '') {
    return 0;
  }
  const parts = token.split('.');
  if (parts.length < 2) {
    return 0;
  }
  try {
    const payload = JSON.parse(decodeBase64Url(parts[1]));
    const expiresAtSeconds = Number(payload?.exp || 0);
    return Number.isFinite(expiresAtSeconds) && expiresAtSeconds > 0
      ? expiresAtSeconds * 1000
      : 0;
  } catch (error) {
    return 0;
  }
}

export function isAccessTokenFresh(
  token,
  now = Date.now(),
  renewSkewMs = ACCESS_TOKEN_RENEW_SKEW_MS,
) {
  const expiresAt = getAccessTokenExpiresAt(token);
  return expiresAt > now + renewSkewMs;
}
