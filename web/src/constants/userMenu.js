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

export const buildUserWorkspaceMenuItems = ({ includeChat = false } = {}) => {
  const items = [
    {
      key: 'service',
      type: 'group',
      name: 'header.service',
      icon: 'grid layout',
      items: [
        {
          name: 'header.dashboard',
          to: '/workspace/dashboard',
          icon: 'chart bar',
        },
        {
          name: 'topup.pricing.title',
          to: '/workspace/service/pricing',
          icon: 'tags',
        },
      ],
    },
  ];

  if (includeChat) {
    items.push({
      name: 'header.chat',
      to: '/workspace/chat',
      icon: 'comments',
    });
  }

  items.push(
    {
      key: 'mine',
      type: 'group',
      name: 'header.mine',
      icon: 'user circle',
      items: [
        {
          name: 'topup.mine.balance',
          to: '/workspace/topup?tab=balance',
          icon: 'credit card',
        },
        {
          name: 'topup.mine.package',
          to: '/workspace/topup?tab=package',
          icon: 'gift',
        },
        {
          name: 'header.token',
          to: '/workspace/token',
          icon: 'key',
        },
        {
          name: 'header.account',
          to: '/workspace/setting',
          icon: 'setting',
        },
      ],
    },
    {
      key: 'records',
      type: 'group',
      name: 'header.records',
      icon: 'history',
      items: [
        {
          name: 'topup.record_nav.topup',
          to: '/workspace/topup?tab=records&record=topup',
          icon: 'credit card',
        },
        {
          name: 'topup.record_nav.package',
          to: '/workspace/topup?tab=records&record=package',
          icon: 'gift',
        },
        {
          name: 'topup.record_nav.redeem',
          to: '/workspace/topup?tab=records&record=redeem',
          icon: 'ticket alternate',
        },
        {
          name: 'header.log',
          to: '/workspace/log',
          icon: 'book',
        },
      ],
    },
    {
      name: 'header.task',
      to: '/workspace/task',
      icon: 'tasks',
    },
    {
      key: 'help',
      type: 'group',
      name: 'header.help',
      icon: 'book',
      items: [
        {
          name: 'header.usage_guide',
          to: '/workspace/service/help',
          icon: 'file alternate outline',
        },
      ],
    },
  );

  return items;
};
