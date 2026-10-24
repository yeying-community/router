export const isUserRouteActive = (location, to) => {
  const [path, queryString = ''] = String(to || '').split('?');
  if (!path) {
    return false;
  }
  if (location.pathname !== path && !location.pathname.startsWith(`${path}/`)) {
    return false;
  }
  if (!queryString) {
    return true;
  }
  const targetParams = new URLSearchParams(queryString);
  const currentParams = new URLSearchParams(location.search || '');
  for (const [key, value] of targetParams.entries()) {
    if ((currentParams.get(key) || '') !== value) {
      return false;
    }
  }
  return true;
};

export const buildUserWorkspaceMenuItems = () => {
  // Normal-user workspace: a flat list of the day-to-day functional entries.
  // Secondary personal entries (account / logs / guides) live in the header
  // avatar dropdown, mirroring the admin console.
  const items = [
    {
      name: 'workspace_models.title',
      to: '/workspace/service/models',
      icon: 'cube',
    },
    {
      name: 'header.token',
      to: '/workspace/token',
      icon: 'key',
    },
    {
      name: 'personal_routing.title',
      to: '/workspace/channels',
      icon: 'share alternate',
    },
    {
      name: 'community_services.title',
      to: '/workspace/community-services',
      icon: 'cloud',
    },
    {
      name: 'topup.mine.quota',
      to: '/workspace/topup?tab=quota',
      icon: 'credit card',
    },
  ];
  // Chat is an embedded iframe that only works once an operator configures a
  // workspace/chat URL (persisted to localStorage from site status). Surface it
  // only when that link exists so we never route users to a blank iframe.
  const chatLink =
    typeof localStorage !== 'undefined'
      ? String(localStorage.getItem('chat_link') || '').trim()
      : '';
  if (chatLink) {
    items.push({
      name: 'header.chat',
      to: '/workspace/chat',
      icon: 'comments',
    });
  }
  return items;
};
