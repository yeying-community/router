export const ADMIN_MENU_GROUPS = [
  {
    key: 'dashboard',
    name: 'header.dashboard',
    icon: 'chart bar',
    items: [
      {
        name: 'header.dashboard',
        to: '/admin/dashboard',
        icon: 'chart bar',
      },
    ],
  },
  {
    key: 'resource',
    name: 'header.resource',
    icon: 'sitemap',
    items: [
      {
        name: 'header.providers',
        to: '/admin/provider',
        icon: 'cubes',
      },
      {
        name: 'header.channel',
        to: '/admin/channel',
        icon: 'sitemap',
      },
      {
        name: 'header.group',
        to: '/admin/group',
        icon: 'group',
      },
    ],
  },
  {
    key: 'operation',
    name: 'header.operation',
    icon: 'tasks',
    items: [
      {
        name: 'header.config',
        to: '/admin/setting?tab=operation&section=quota',
        icon: 'sliders horizontal',
      },
      {
        name: 'header.user',
        to: '/admin/user',
        icon: 'user',
      },
      {
        name: 'header.redemption',
        to: '/admin/redemption',
        icon: 'dollar sign',
      },
      {
        name: 'header.log',
        to: '/admin/log',
        icon: 'book',
      },
      {
        name: 'header.task',
        to: '/admin/task',
        icon: 'tasks',
      },
    ],
  },
  {
    key: 'setting',
    name: 'header.setting_center',
    icon: 'setting',
    items: [
      {
        name: 'setting.tabs.system',
        to: '/admin/setting?tab=system&section=general',
        icon: 'server',
      },
      {
        name: 'setting.tabs.other',
        to: '/admin/setting?tab=other&section=notice',
        icon: 'options',
      },
    ],
  },
];

export const isAdminRouteActive = (location, to) => {
  if (!location) {
    return false;
  }
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
  const targetTab = (targetParams.get('tab') || '').trim().toLowerCase();
  if (path === '/admin/setting' && targetTab !== '') {
    const currentTab = (currentParams.get('tab') || 'system')
      .trim()
      .toLowerCase();
    if (currentTab !== targetTab) {
      return false;
    }
    return true;
  }
  const entries = Array.from(targetParams.entries());
  if (entries.length === 0) {
    return true;
  }
  return entries.every(
    ([key, value]) => (currentParams.get(key) || '') === value,
  );
};

export const isAdminGroupActive = (location, group) =>
  Array.isArray(group?.items) &&
  group.items.some((item) => isAdminRouteActive(location, item.to));
