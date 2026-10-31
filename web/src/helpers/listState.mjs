const normalizePage = (value) => {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 ? parsed : 1;
};

/**
 * Return a stable signature containing only the query parameters that affect a
 * list request's filters. Pagination/sort/navigation parameters must not be
 * included: writing `page=2` to the URL must never masquerade as a filter
 * change and trigger a reset back to page 1.
 */
export function buildQueryFilterSignature(search, filterKeys) {
  const source = new URLSearchParams(search || '');
  const signature = new URLSearchParams();
  const seen = new Set();

  (Array.isArray(filterKeys) ? filterKeys : []).forEach((rawKey) => {
    const key = String(rawKey || '').trim();
    if (key === '' || seen.has(key)) {
      return;
    }
    seen.add(key);
    const value = (source.get(key) || '').trim();
    if (value !== '') {
      signature.set(key, value);
    }
  });

  return signature.toString();
}

/**
 * Effects are replayed on mount in React StrictMode. Comparing a semantic
 * request key, instead of a mutable "did mount" boolean, keeps both mount
 * passes on the URL-restored page while still resetting real filter changes.
 * Callers should run the effect only when that semantic key changes (apart
 * from StrictMode's deliberate mount replay).
 */
export function resolveListEffectPage(previousKey, nextKey, initialPage) {
  return previousKey === nextKey ? normalizePage(initialPage) : 1;
}
