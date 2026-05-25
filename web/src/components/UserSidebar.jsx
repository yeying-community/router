import React, { useEffect, useMemo, useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  buildUserWorkspaceMenuItems,
  isUserRouteActive,
} from '../constants/userMenu';
import { AppIcon, AppNavMenu } from '../router-ui';

const USER_SIDEBAR_GROUP_OPEN_STORAGE_KEY = 'router_user_sidebar_group_open_v2';

const UserSidebar = ({ compact = false }) => {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const menuItems = useMemo(() => buildUserWorkspaceMenuItems(), []);

  const groupedKeys = useMemo(
    () => menuItems.filter((item) => item.type === 'group').map((item) => item.key),
    [menuItems],
  );

  const [openKeys, setOpenKeys] = useState(() => {
    if (typeof window === 'undefined') {
      return groupedKeys;
    }
    const raw = (
      localStorage.getItem(USER_SIDEBAR_GROUP_OPEN_STORAGE_KEY) || ''
    ).trim();
    if (raw === '') {
      return groupedKeys;
    }
    try {
      const parsed = JSON.parse(raw);
      if (!Array.isArray(parsed)) {
        return groupedKeys;
      }
      const allowed = new Set(groupedKeys);
      const filtered = parsed.filter((key) => allowed.has(key));
      return filtered.length > 0 ? filtered : groupedKeys;
    } catch {
      return groupedKeys;
    }
  });

  const selectedKeys = useMemo(() => {
    const active = [];
    menuItems.forEach((item) => {
      if (item.type === 'group' && Array.isArray(item.items)) {
        item.items.forEach((child) => {
          if (isUserRouteActive(location, child.to)) {
            active.push(child.to);
          }
        });
        return;
      }
      if (item.to && isUserRouteActive(location, item.to)) {
        active.push(item.to);
      }
    });
    return active;
  }, [location, menuItems]);

  useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }
    localStorage.setItem(
      USER_SIDEBAR_GROUP_OPEN_STORAGE_KEY,
      JSON.stringify(openKeys),
    );
  }, [openKeys]);

  useEffect(() => {
    if (compact || selectedKeys.length === 0) {
      return;
    }
    const activeGroupKeys = menuItems
      .filter(
        (item) =>
          item.type === 'group' &&
          Array.isArray(item.items) &&
          item.items.some((child) => selectedKeys.includes(child.to)),
      )
      .map((item) => item.key);
    if (activeGroupKeys.length === 0) {
      return;
    }
    setOpenKeys((previous) => {
      const next = Array.from(new Set([...previous, ...activeGroupKeys]));
      return next.length === previous.length &&
        next.every((item, index) => item === previous[index])
        ? previous
        : next;
    });
  }, [compact, menuItems, selectedKeys]);

  const items = useMemo(
    () =>
      menuItems.map((item) => {
        if (item.type === 'group' && Array.isArray(item.items)) {
          return {
            key: item.key,
            icon: <AppIcon name={item.icon} />,
            label: t(item.name),
            children: item.items.map((child) => ({
              key: child.to,
              icon: <AppIcon name={child.icon} />,
              label: t(child.name),
            })),
          };
        }
        return {
          key: item.to,
          icon: <AppIcon name={item.icon} />,
          label: t(item.name),
        };
      }),
    [menuItems, t],
  );

  return (
    <AppNavMenu
      className='router-admin-nav-menu router-user-nav-menu'
      mode='inline'
      inlineCollapsed={compact}
      triggerSubMenuAction={compact ? 'click' : 'hover'}
      items={items}
      selectedKeys={selectedKeys}
      {...(!compact
        ? {
            openKeys,
            onOpenChange: (nextKeys) => setOpenKeys(nextKeys),
          }
        : {})}
      onClick={({ key }) => {
        if (typeof key === 'string' && key.startsWith('/')) {
          navigate(key);
        }
      }}
    />
  );
};

export default UserSidebar;
