const AUTH_REDIRECT_STORAGE_KEY = 'auth_redirect_path';

export function normalizeRedirectPath(value) {
  const redirect = String(value || '').trim();
  if (!redirect.startsWith('/')) {
    return '';
  }
  if (redirect.startsWith('//')) {
    return '';
  }
  return redirect;
}

export function rememberAuthRedirectPath(value) {
  if (typeof window === 'undefined') {
    return;
  }
  const redirect = normalizeRedirectPath(value);
  if (!redirect) {
    window.sessionStorage.removeItem(AUTH_REDIRECT_STORAGE_KEY);
    return;
  }
  window.sessionStorage.setItem(AUTH_REDIRECT_STORAGE_KEY, redirect);
}

export function consumeAuthRedirectPath() {
  if (typeof window === 'undefined') {
    return '';
  }
  const redirect = normalizeRedirectPath(
    window.sessionStorage.getItem(AUTH_REDIRECT_STORAGE_KEY),
  );
  window.sessionStorage.removeItem(AUTH_REDIRECT_STORAGE_KEY);
  return redirect;
}

export function buildLoginPath(locationLike) {
  const pathname = String(locationLike?.pathname || '').trim();
  const search = String(locationLike?.search || '');
  const hash = String(locationLike?.hash || '');
  const redirect = normalizeRedirectPath(`${pathname}${search}${hash}`);
  if (!redirect || redirect === '/login') {
    return '/login';
  }
  return `/login?redirect=${encodeURIComponent(redirect)}`;
}

export function resolvePostLoginPath(searchParams, fallbackPath) {
  const redirectFromQuery = normalizeRedirectPath(
    searchParams?.get('redirect'),
  );
  if (redirectFromQuery) {
    rememberAuthRedirectPath(redirectFromQuery);
    return consumeAuthRedirectPath();
  }
  return consumeAuthRedirectPath() || fallbackPath;
}
