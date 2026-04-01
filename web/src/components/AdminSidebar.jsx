import React, { useEffect, useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { Icon, Menu, Popup } from 'semantic-ui-react';
import { useTranslation } from 'react-i18next';
import {
  ADMIN_MENU_GROUPS,
  isAdminGroupActive,
  isAdminRouteActive,
} from '../constants/adminMenu';

const SIDEBAR_GROUP_COLLAPSED_STORAGE_KEY =
  'router_admin_sidebar_group_collapsed_v1';

const buildDefaultCollapsedState = () => {
  const defaults = {};
  ADMIN_MENU_GROUPS.forEach((group) => {
    defaults[group.key] = false;
  });
  return defaults;
};

const buildInitialCollapsedState = () => {
  const defaults = buildDefaultCollapsedState();
  if (typeof window === 'undefined') {
    return defaults;
  }
  const raw = (localStorage.getItem(SIDEBAR_GROUP_COLLAPSED_STORAGE_KEY) || '')
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

const AdminSidebar = ({ compact = false }) => {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const [collapsedGroups, setCollapsedGroups] = useState(
    buildInitialCollapsedState,
  );
  const [compactPopupGroup, setCompactPopupGroup] = useState('');

  useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }
    localStorage.setItem(
      SIDEBAR_GROUP_COLLAPSED_STORAGE_KEY,
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
    setCollapsedGroups((prev) => ({
      ...prev,
      [group.key]: !prev[group.key],
    }));
  };

  const isGroupCollapsed = (group) => Boolean(collapsedGroups[group.key]);

  return (
    <Menu vertical fluid className='router-admin-sidebar-menu'>
      {ADMIN_MENU_GROUPS.map((group) => {
        const groupActive = isAdminGroupActive(location, group);
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
                  const active = isAdminRouteActive(location, item.to);
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
                  const active = isAdminRouteActive(location, item.to);
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

export default AdminSidebar;
