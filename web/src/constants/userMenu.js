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
  const items = [
    {
      key: 'service',
      type: 'group',
      name: 'header.service',
      icon: 'grid layout',
      items: [
        {
          name: 'topup.pricing.title',
          to: '/workspace/service/pricing',
          icon: 'tags',
        },
        {
          name: 'workspace_models.title',
          to: '/workspace/service/models',
          icon: 'cube',
        },
        {
          name: 'header.log',
          to: '/workspace/log',
          icon: 'book',
        },
      ],
    },
  ];

  items.push(
    {
      key: 'mine',
      type: 'group',
      name: 'header.mine',
      icon: 'user circle',
      items: [
        {
          name: 'topup.mine.spend',
          to: '/workspace/dashboard',
          icon: 'chart bar',
        },
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
        {
          name: 'header.account',
          to: '/workspace/setting',
          icon: 'setting',
        },
      ],
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
    }
  );

  return items;
};
