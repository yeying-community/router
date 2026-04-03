export const ADMIN_MENU_GROUPS = [
  {
    key: 'dashboard',
    name: 'header.system_overview',
    icon: 'chart bar',
    items: [
      {
        name: 'dashboard.admin.nav.overview',
        to: '/admin/dashboard?section=overview',
        icon: 'chart pie',
      },
      {
        name: 'dashboard.admin.sections.trend',
        to: '/admin/dashboard?section=trend',
        icon: 'chart line',
      },
      {
        name: 'dashboard.admin.sections.channels',
        to: '/admin/dashboard?section=health',
        icon: 'heartbeat',
      },
    ],
  },
  {
    key: 'resource',
    name: 'header.resource',
    icon: 'cube',
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
    key: 'business',
    name: 'header.business_operation',
    icon: 'users',
    items: [
      {
        name: 'header.package',
        to: '/admin/package',
        icon: 'gift',
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
    ],
  },
  {
    key: 'flow',
    name: 'header.business_flow',
    icon: 'clipboard list',
    items: [
      {
        name: 'flow.topup.title',
        to: '/admin/flow/topup',
        icon: 'credit card',
      },
      {
        name: 'flow.package.title',
        to: '/admin/flow/package',
        icon: 'gift',
      },
      {
        name: 'flow.redemption.title',
        to: '/admin/flow/redemption',
        icon: 'exchange',
      },
    ],
  },
  {
    key: 'operation',
    name: 'header.platform_operation',
    icon: 'tasks',
    items: [
      {
        name: 'setting.tabs.currency',
        to: '/admin/setting?tab=currency&section=catalog',
        icon: 'money bill alternate outline',
      },
      {
        name: 'setting.tabs.exchange',
        to: '/admin/setting?tab=exchange&section=rates',
        icon: 'exchange',
      },
      {
        name: 'header.config',
        to: '/admin/setting?tab=operation&section=config',
        icon: 'sliders horizontal',
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
        name: 'setting.system.general.title',
        to: '/admin/setting?tab=general&section=general',
        icon: 'sliders horizontal',
      },
      {
        name: 'setting.system.smtp.title',
        to: '/admin/setting?tab=smtp&section=smtp',
        icon: 'mail',
      },
      {
        name: 'setting.system.login.title',
        to: '/admin/setting?tab=login&section=login',
        icon: 'server',
      },
      {
        name: 'setting.operation.monitor.title',
        to: '/admin/setting?tab=monitor&section=monitor',
        icon: 'heartbeat',
      },
      {
        name: 'setting.operation.log.title',
        to: '/admin/setting?tab=log_setting&section=log',
        icon: 'book',
      },
      {
        name: 'setting.system.notice',
        to: '/admin/setting?tab=notice&section=notice',
        icon: 'options',
      },
      {
        name: 'setting.other.content.title',
        to: '/admin/setting?tab=content&section=content',
        icon: 'file alternate outline',
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
    const rawCurrentTab = (currentParams.get('tab') || 'general')
      .trim()
      .toLowerCase();
    const currentSection = (currentParams.get('section') || '')
      .trim()
      .toLowerCase();
    const currentTab =
      rawCurrentTab === 'system'
        ? currentSection === 'smtp'
          ? 'smtp'
          : currentSection === 'login'
            ? 'login'
            : currentSection === 'monitor'
              ? 'monitor'
              : currentSection === 'log'
                ? 'log_setting'
                : 'general'
        : rawCurrentTab === 'operation'
          ? currentSection === 'monitor'
            ? 'monitor'
            : currentSection === 'log'
              ? 'log_setting'
              : 'operation'
          : rawCurrentTab === 'other'
            ? currentSection === 'content'
              ? 'content'
              : 'notice'
            : rawCurrentTab;
    if (currentTab !== targetTab) {
      return false;
    }
    return true;
  }
  const targetSection = (targetParams.get('section') || '').trim().toLowerCase();
  if (path === '/admin/dashboard' && targetSection !== '') {
    const currentSection = (currentParams.get('section') || 'overview')
      .trim()
      .toLowerCase();
    if (currentSection !== targetSection) {
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
