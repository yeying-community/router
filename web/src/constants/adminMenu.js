import { resolveAdminSettingLocation } from '../helpers/adminSetting';

export const ADMIN_MENU_GROUPS = [
  {
    key: 'dashboard',
    name: 'header.system_overview',
    icon: 'chart bar',
    items: [
      {
        name: 'dashboard.admin.nav.spending',
        to: '/admin/dashboard?section=spending',
        icon: 'chart pie',
      },
      {
        name: 'dashboard.admin.nav.channels',
        to: '/admin/dashboard?section=channels',
        icon: 'heartbeat',
      },
      {
        name: 'dashboard.admin.nav.users',
        to: '/admin/dashboard?section=users',
        icon: 'users',
      },
      {
        name: 'dashboard.admin.nav.alerts',
        to: '/admin/alerts',
        icon: 'heartbeat',
      },
    ],
  },
  {
    key: 'model',
    name: 'header.model',
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
      {
        name: 'header.entitlement',
        to: '/admin/entitlement',
        icon: 'ticket',
      },
    ],
  },
  {
    key: 'business',
    name: 'header.operation',
    icon: 'users',
    items: [
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
    key: 'billing',
    name: 'header.billing',
    icon: 'calculator',
    items: [
      {
        name: 'billing.overview.nav',
        to: '/admin/billing/overview',
        icon: 'dashboard',
      },
      {
        name: 'billing.pricing_analysis.nav',
        to: '/admin/billing/pricing-analysis',
        icon: 'line chart',
      },
      {
        name: 'billing.procurement_report.nav',
        to: '/admin/billing/procurement-report',
        icon: 'exchange alternate',
      },
    ],
  },
  {
    key: 'setting',
    name: 'header.setting',
    icon: 'setting',
    items: [
      {
        name: 'setting.groups.basic',
        to: '/admin/setting?tab=basic&section=general',
        icon: 'sliders horizontal',
      },
      {
        name: 'setting.groups.payment',
        to: '/admin/setting?tab=payment&section=currency',
        icon: 'credit card outline',
      },
      {
        name: 'setting.groups.billing',
        to: '/admin/setting?tab=billing&section=balance',
        icon: 'money bill alternate outline',
      },
      {
        name: 'setting.groups.content',
        to: '/admin/setting?tab=content&section=notice',
        icon: 'file alternate outline',
      },
      {
        name: 'setting.groups.runtime',
        to: '/admin/setting?tab=runtime&section=monitor',
        icon: 'heartbeat',
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
    const { tab: currentTab } = resolveAdminSettingLocation(
      currentParams.get('tab') || 'basic',
      currentParams.get('section') || 'general',
    );
    if (currentTab !== targetTab) {
      return false;
    }
    return true;
  }
  if (path === '/admin/entitlement' && targetTab !== '') {
    const currentTab = (currentParams.get('tab') || 'topup')
      .trim()
      .toLowerCase();
    if (currentTab !== targetTab) {
      return false;
    }
    return true;
  }
  const targetSection = (targetParams.get('section') || '').trim().toLowerCase();
  if (path === '/admin/dashboard' && targetSection !== '') {
    const currentSection = (currentParams.get('section') || 'spending')
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
