import { resolveAdminSettingLocation } from '../helpers/adminSetting';
import { buildUserWorkspaceMenuItems } from './userMenu';

export const ADMIN_MENU_GROUPS = [
  {
    // 总览:唯一真正跨实体的大盘。单项组 → 侧边栏渲染为扁平叶子。
    // 其余原仪表盘 section(渠道健康/用户分析/模型运营)已归位到各实体页的
    // SectionTabs,不再作为独立侧边栏项。
    key: 'dashboard',
    name: 'header.dashboard',
    icon: 'chart line',
    items: [
      {
        name: 'dashboard.admin.nav.duty',
        to: '/admin/dashboard',
        icon: 'chart line',
        // The duty station is the only incident acknowledgement surface. Keep
        // the existing lightweight critical-channel signal as its global dot
        // until a cross-domain duty summary endpoint is available.
        badge: 'duty-alerts',
      },
    ],
  },
  {
    key: 'supply',
    name: 'header.supply',
    icon: 'cube',
    items: [
      {
        name: 'header.providers',
        to: '/admin/provider',
        icon: 'cubes',
      },
      {
        // 渠道的「面」:列表 / 健康 现已全部落在 /admin/channel?tab=…,
        // 与 /admin/channel/* 详情路由同前缀,基准 `to` 即可覆盖全部面,无需 matchPaths。
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
        // The model catalog is a supply-side artifact (published models backed
        // by providers/channels), so it belongs with供给 rather than客户.
        // 「运营」面现为模型页的 ?tab=operations,与 catalog 同路径,基准 `to` 即覆盖。
        name: 'header.model',
        to: '/workspace/service/models',
        icon: 'cube',
      },
    ],
  },
  {
    // 用户从原「产品与客户」组拎出:用户是消费主体,与「产品」(兑换码/权益)分属
    // 两类实体,独立成单项组 → 侧边栏渲染为扁平叶子。
    // 用户的「分析」/「任务」面现为 /admin/user?tab=…(基准 `to` 覆盖)。
    // /admin/task 列表已重定向到 tab 条,但任务详情仍在 /admin/task/:id,
    // 故保留该 matchPath 前缀以在详情页高亮本项。
    key: 'users',
    name: 'header.user',
    icon: 'users',
    items: [
      {
        name: 'header.user',
        to: '/admin/user',
        icon: 'user',
        matchPaths: ['/admin/task'],
      },
    ],
  },
  {
    // 产品:面向售卖的实体(兑换码、权益产品)。用户已拎出为独立组。
    key: 'products',
    name: 'header.products',
    icon: 'ticket',
    items: [
      {
        name: 'header.redemption',
        to: '/admin/redemption',
        icon: 'dollar sign',
      },
      {
        name: 'header.entitlement',
        to: '/admin/entitlement',
        icon: 'ticket',
      },
    ],
  },
  {
    // 财务:总览/利润/采购收敛为单壳 `/admin/finance?tab=`,侧边栏用无 query 的
    // 裸 `to` 前缀匹配全部 tab(同渠道),无需 matchPaths。
    key: 'finance',
    name: 'header.finance',
    icon: 'money bill alternate outline',
    items: [
      {
        name: 'header.finance',
        to: '/admin/finance',
        icon: 'money bill alternate outline',
      },
    ],
  },
  {
    // 系统组:日志(各实体下钻落点)与全站令牌管理。多叶子组按分组渲染。
    key: 'system',
    name: 'header.system',
    icon: 'book',
    items: [
      {
        name: 'header.log',
        to: '/admin/log',
        icon: 'book',
      },
      {
        name: 'header.token_admin',
        to: '/admin/token',
        icon: 'key',
      },
    ],
  },
  {
    key: 'setting',
    name: 'header.setting',
    icon: 'setting',
    items: [
      {
        name: 'header.setting',
        to: '/admin/setting?tab=basic&section=general',
        icon: 'sliders horizontal',
      },
    ],
  },
];

// Admin operators are end users too: give their own quota/tokens a dedicated
// "personal" group pinned to the bottom of the admin sidebar, so seeing your
// own usage no longer means digging into the header avatar dropdown. The items
// point at the shared /workspace/* surfaces (isUserRouteActive handles them).
export const PERSONAL_WORKSPACE_MENU_GROUP = {
  key: 'personal',
  name: 'header.mine',
  icon: 'user',
  items: [
    {
      name: 'topup.mine.quota',
      to: '/workspace/topup?tab=quota',
      icon: 'credit card',
    },
    {
      name: 'header.token',
      to: '/workspace/token',
      icon: 'key',
    },
  ],
};
export const buildUnifiedWorkspaceMenuGroups = (hasAdminAccess) => {
  if (!hasAdminAccess) {
    return buildUserWorkspaceMenuItems();
  }
  // Admin console: the operator surface, plus a personal group so operators can
  // reach their own quota/tokens straight from the sidebar. Remaining personal
  // entries (account / logs / guide) stay in the header avatar dropdown, which
  // is now shown to every role for consistency.
  return [...ADMIN_MENU_GROUPS, PERSONAL_WORKSPACE_MENU_GROUP].map((group) => ({
    ...group,
    type: 'group',
    items: group.items.map((item) => ({ ...item })),
  }));
};

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
      currentParams.get('section') || 'general'
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
  const entries = Array.from(targetParams.entries());
  if (entries.length === 0) {
    return true;
  }
  return entries.every(
    ([key, value]) => (currentParams.get(key) || '') === value
  );
};

// A single sidebar item can stand for one operational entity whose several
// "faces" live at different routes (e.g. 渠道 = list + health + tasks).
// `matchPaths` lists those extra face routes so the item highlights on any of
// them; the primary `to` stays the entity's default (list) face. Paths may be
// admin or /workspace/* — the caller picks the right matcher per path.
export const isAdminItemActive = (location, item, routeMatcher) => {
  if (!item) {
    return false;
  }
  const match =
    typeof routeMatcher === 'function' ? routeMatcher : isAdminRouteActive;
  if (match(location, item.to)) {
    return true;
  }
  return (item.matchPaths || []).some((path) => match(location, path));
};
