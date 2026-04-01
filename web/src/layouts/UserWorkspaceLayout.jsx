import React, { useEffect, useMemo, useState } from 'react';
import { Outlet } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Container, Icon } from 'semantic-ui-react';
import Footer from '../components/Footer';
import Header from '../components/Header';
import UserSidebar from '../components/UserSidebar';

const USER_SIDEBAR_COMPACT_STORAGE_KEY = 'router_user_sidebar_compact_v1';

const UserWorkspaceLayout = () => {
  const { t } = useTranslation();
  const initialCompact = useMemo(() => {
    if (typeof window === 'undefined') {
      return false;
    }
    const raw = (localStorage.getItem(USER_SIDEBAR_COMPACT_STORAGE_KEY) || '')
      .trim()
      .toLowerCase();
    return raw === '1' || raw === 'true';
  }, []);
  const [sidebarCompact, setSidebarCompact] = useState(initialCompact);

  useEffect(() => {
    if (typeof window === 'undefined') {
      return;
    }
    localStorage.setItem(
      USER_SIDEBAR_COMPACT_STORAGE_KEY,
      sidebarCompact ? '1' : '0',
    );
  }, [sidebarCompact]);

  return (
    <>
      <Header workspace='user' hideNavButtons />
      <div className={`router-admin-shell ${sidebarCompact ? 'compact' : ''}`}>
        <aside
          className={`router-admin-sidebar ${sidebarCompact ? 'compact' : ''}`}
        >
          <UserSidebar compact={sidebarCompact} />
        </aside>
        <span
          className='router-admin-divider-toggle'
          role='button'
          tabIndex={0}
          title={
            sidebarCompact
              ? t('header.sidebar_expand')
              : t('header.sidebar_compact')
          }
          onClick={() => setSidebarCompact((previous) => !previous)}
          onKeyDown={(event) => {
            if (event.key === 'Enter' || event.key === ' ') {
              event.preventDefault();
              setSidebarCompact((previous) => !previous);
            }
          }}
        >
          <Icon
            name={sidebarCompact ? 'angle double right' : 'angle double left'}
          />
        </span>
        <Container className='main-content router-admin-main'>
          <Outlet />
        </Container>
      </div>
      <Footer />
    </>
  );
};

export default UserWorkspaceLayout;
