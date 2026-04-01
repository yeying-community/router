import React, { useEffect, useMemo, useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { Icon, Menu, Popup } from 'semantic-ui-react';
import { useTranslation } from 'react-i18next';

const USER_SIDEBAR_GROUP_COLLAPSED_STORAGE_KEY =
  'router_user_sidebar_group_collapsed_v1';

const isUserRouteActive = (location, to) => {
  const [path] = String(to || '').split('?');
  if (!path) {
    return false;
  }
  return location.pathname === path || location.pathname.startsWith(`${path}/`);
};

const buildUserMenuGroups = (includeChat = false) => {
  const groups = [
    {
      key: 'dashboard',
      name: 'header.dashboard',
      icon: 'chart bar',
      items: [
        {
          name: 'header.dashboard',
          to: '/workspace/dashboard',
          icon: 'chart bar',
        },
      ],
    },
    {
      key: 'token',
      name: 'header.token',
      icon: 'key',
      items: [
        {
          name: 'header.token',
          to: '/workspace/token',
          icon: 'key',
        },
      ],
    },
  ];

  if (includeChat) {
    groups.push({
      key: 'chat',
      name: 'header.chat',
      icon: 'comments',
      items: [
        {
          name: 'header.chat',
          to: '/workspace/chat',
          icon: 'comments',
        },
      ],
    });
  }

  groups.push(
    {
      key: 'topup',
      name: 'header.topup',
      icon: 'cart',
      items: [
        {
          name: 'header.topup',
          to: '/workspace/topup',
          icon: 'cart',
        },
      ],
    },
    {
      key: 'log',
      name: 'header.log',
      icon: 'book',
      items: [
        {
          name: 'header.log',
          to: '/workspace/log',
          icon: 'book',
        },
      ],
    },
    {
      key: 'task',
      name: 'header.task',
      icon: 'tasks',
      items: [
        {
          name: 'header.task',
          to: '/workspace/task',
          icon: 'tasks',
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
          to: '/workspace/setting',
          icon: 'setting',
        },
      ],
    },
  );

  return groups;
};

const buildDefaultCollapsedState = (groups) => {
  const defaults = {};
  groups.forEach((group) => {
    defaults[group.key] = false;
  });
  return defaults;
};

const buildInitialCollapsedState = (groups) => {
  const defaults = buildDefaultCollapsedState(groups);
  if (typeof window === 'undefined') {
    return defaults;
  }
  const raw = (localStorage.getItem(USER_SIDEBAR_GROUP_COLLAPSED_STORAGE_KEY) || '')
    .trim();
  if (raw === '') {
    return defaults;
  }
  try {
    const parsed = JSON.parse(raw);
    if (!parsed || typeof parsed !== 'object') {
      return defaults;
    }
    return Object.keys(defaults).reduce((result, key) => {
      result[key] = Boolean(parsed[key]);
      return result;
    }, {});
  } catch (error) {
    return defaults;
  }
};

const UserSidebar = ({ compact = false }) => {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const includeChat = Boolean(localStorage.getItem('chat_link'));
  const menuGroups = useMemo(
    () => buildUserMenuGroups(includeChat),
    [includeChat],
  );

  const [collapsedGroups, setCollapsedGroups] = useState(() =>
    buildInitialCollapsedState(menuGroups),
  );
  const [compactPopupGroup, setCompactPopupGroup] = useState('');

  useEffect(() => {
    setCollapsedGroups((previous) => {
      const defaults = buildDefaultCollapsedState(menuGroups);
      return Object.keys(defaults).reduce((result, key) => {
        result[key] = Boolean(previous?.[key]);
        return result;
      }, {});
    });
  }, [menuGroups]);

  useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }
    localStorage.setItem(
      USER_SIDEBAR_GROUP_COLLAPSED_STORAGE_KEY,
      JSON.stringify(collapsedGroups),
    );
  }, [collapsedGroups]);

  useEffect(() => {
    if (!compact) {
      setCompactPopupGroup('');
    }
  }, [compact]);

  useEffect(() => {
    setCompactPopupGroup('');
  }, [location.pathname, location.search, location.hash]);

  const toggleGroup = (group) => {
    if (!group?.key) {
      return;
    }
    setCollapsedGroups((previous) => ({
      ...previous,
      [group.key]: !previous[group.key],
    }));
  };

  const isGroupActive = (group) =>
    Array.isArray(group?.items) &&
    group.items.some((item) => isUserRouteActive(location, item.to));

  const isGroupCollapsed = (group) => Boolean(collapsedGroups[group.key]);

  return (
    <Menu vertical fluid className='router-admin-sidebar-menu'>
      {menuGroups.map((group) => {
        const groupActive = isGroupActive(group);
        if (compact) {
          const popupOpen = compactPopupGroup === group.key;
          return (
            <Popup
              key={group.key}
              className='router-admin-compact-popup'
              on='click'
              position='right center'
              open={popupOpen}
              onClose={() =>
                setCompactPopupGroup((previous) =>
                  previous === group.key ? '' : previous,
                )
              }
              trigger={
                <Menu.Item
                  className={`router-admin-sidebar-group ${groupActive ? 'active' : ''}`}
                  onClick={() =>
                    setCompactPopupGroup((previous) =>
                      previous === group.key ? '' : group.key,
                    )
                  }
                  title={t(group.name)}
                >
                  <span className='router-admin-sidebar-group-title'>
                    <Icon name={group.icon} />
                    <span className='router-admin-sidebar-group-label'>
                      {t(group.name)}
                    </span>
                  </span>
                </Menu.Item>
              }
            >
              <Menu vertical secondary className='router-admin-compact-popup-menu'>
                {group.items.map((item) => {
                  const active = isUserRouteActive(location, item.to);
                  return (
                    <Menu.Item
                      key={item.to}
                      active={active}
                      className='router-admin-compact-popup-item'
                      onClick={() => {
                        setCompactPopupGroup('');
                        navigate(item.to);
                      }}
                    >
                      <Icon name={item.icon} />
                      <span className='router-admin-compact-popup-item-label'>
                        {t(item.name)}
                      </span>
                    </Menu.Item>
                  );
                })}
              </Menu>
            </Popup>
          );
        }

        const collapsed = isGroupCollapsed(group);
        return (
          <Menu.Item
            key={group.key}
            className={`router-admin-sidebar-group ${groupActive ? 'active' : ''}`}
          >
            <div
              className='router-admin-sidebar-group-header'
              role='button'
              tabIndex={0}
              onClick={() => toggleGroup(group)}
              onKeyDown={(event) => {
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault();
                  toggleGroup(group);
                }
              }}
            >
              <span
                className='router-admin-sidebar-group-title'
                title={t(group.name)}
              >
                <Icon name={group.icon} />
                <span className='router-admin-sidebar-group-label'>
                  {t(group.name)}
                </span>
              </span>
              <Icon name={collapsed ? 'angle right' : 'angle down'} />
            </div>
            {!collapsed ? (
              <Menu.Menu>
                {group.items.map((item) => {
                  const active = isUserRouteActive(location, item.to);
                  return (
                    <Menu.Item
                      key={item.to}
                      active={active}
                      onClick={() => navigate(item.to)}
                      className='router-admin-sidebar-item'
                      title={t(item.name)}
                    >
                      <Icon name={item.icon} />
                      <span className='router-admin-sidebar-item-label'>
                        {t(item.name)}
                      </span>
                    </Menu.Item>
                  );
                })}
              </Menu.Menu>
            ) : null}
          </Menu.Item>
        );
      })}
    </Menu>
  );
};

export default UserSidebar;
