import React, { useContext, useEffect, useMemo, useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { UserContext } from '../context/User';
import { StatusContext } from '../context/Status';
import { API, getLogo, isAdmin, isMobile } from '../helpers';
import { WEB3_TOKEN_STORAGE_KEY } from '../helpers/web3';
import { logoutWallet } from '../services/web3Auth';
import {
  ADMIN_MENU_GROUPS,
  isAdminRouteActive,
} from '../constants/adminMenu';
import {
  buildUserWorkspaceMenuItems,
  isUserRouteActive as isUserWorkspaceRouteActive,
} from '../constants/userMenu';
import {
  AppButton,
  AppDrawer,
  AppIcon,
  AppMenuDropdown,
  AppNavMenu,
  AppSelect,
} from '../router-ui';
import '../index.css';

const Header = ({ workspace = 'user', hideNavButtons = false }) => {
  const { t, i18n } = useTranslation();
  const [userState, userDispatch] = useContext(UserContext);
  const [statusState] = useContext(StatusContext);
  const navigate = useNavigate();
  const location = useLocation();

  const [showSidebar, setShowSidebar] = useState(false);
  const logo = getLogo();
  const shouldFixHeader = Boolean(userState?.user);
  const currentWorkspace = workspace === 'admin' ? 'admin' : 'user';
  const hasAdminAccess = isAdmin();
  const adminFlatButtons = useMemo(
    () => ADMIN_MENU_GROUPS.flatMap((group) => group.items),
    [],
  );
  const userButtons = useMemo(() => buildUserWorkspaceMenuItems(), []);
  const headerContainerClass = [
    'router-header-container',
    hideNavButtons ? 'router-header-container-full' : '',
  ]
    .filter(Boolean)
    .join(' ');

  useEffect(() => {
    const body = document.body;
    if (!body) return;
    body.classList.toggle('header-fixed-active', shouldFixHeader);
    return () => {
      body.classList.remove('header-fixed-active');
    };
  }, [shouldFixHeader]);

  const isRouteActive = (to) => {
    if (currentWorkspace === 'admin') {
      return isAdminRouteActive(location, to);
    }
    return isUserWorkspaceRouteActive(location, to);
  };

  const goToWorkspace = (targetWorkspace) => {
    if (targetWorkspace === 'admin') {
      navigate('/admin/dashboard');
    } else {
      navigate('/workspace/entry');
    }
    setShowSidebar(false);
  };

  async function logout() {
    setShowSidebar(false);
    await API.get('/api/v1/public/user/logout');
    try {
      await logoutWallet();
    } catch (e) {
      // ignore web3 logout errors
    }
    userDispatch({ type: 'logout' });
    localStorage.removeItem('user');
    localStorage.removeItem(WEB3_TOKEN_STORAGE_KEY);
    localStorage.removeItem('wallet_token_expires_at');
    navigate('/login');
  }

  const languageOptions = [
    { key: 'zh', text: '中文', value: 'zh' },
    { key: 'en', text: 'English', value: 'en' },
  ];

  const changeLanguage = (language) => {
    i18n.changeLanguage(language);
  };

  const storedStatus = (() => {
    const raw = localStorage.getItem('status');
    if (!raw) {
      return undefined;
    }
    try {
      return JSON.parse(raw);
    } catch (error) {
      return undefined;
    }
  })();

  const status = statusState?.status || storedStatus || {};
  const passwordRegisterEnabled =
    status?.register_enabled !== false &&
    status?.password_register_enabled !== false;

  const desktopNavItems = useMemo(() => {
    if (currentWorkspace === 'admin') {
      return ADMIN_MENU_GROUPS.map((group) => ({
        key: group.key,
        label: t(group.name),
        children: group.items.map((item) => ({
          key: item.to,
          icon: <AppIcon name={item.icon} />,
          label: t(item.name),
        })),
      }));
    }
    return userButtons.map((button) => {
      if (button.type === 'group' && Array.isArray(button.items)) {
        return {
          key: button.key || button.name,
          label: t(button.name),
          children: button.items.map((item) => ({
            key: item.to,
            icon: <AppIcon name={item.icon} />,
            label: t(item.name),
          })),
        };
      }
      return {
        key: button.to,
        label: t(button.name),
      };
    });
  }, [currentWorkspace, t, userButtons]);

  const desktopSelectedKeys = useMemo(() => {
    if (currentWorkspace === 'admin') {
      return adminFlatButtons
        .filter((item) => isAdminRouteActive(location, item.to))
        .map((item) => item.to);
    }
    return userButtons.flatMap((button) => {
      if (button.type === 'group' && Array.isArray(button.items)) {
        return button.items
          .filter((item) => isUserWorkspaceRouteActive(location, item.to))
          .map((item) => item.to);
      }
      return button.to && isUserWorkspaceRouteActive(location, button.to)
        ? [button.to]
        : [];
    });
  }, [adminFlatButtons, currentWorkspace, location, userButtons]);

  const renderMobileButtons = () => {
    const buttons = currentWorkspace === 'admin' ? adminFlatButtons : userButtons;
    return buttons.map((button) => {
      if (button.type === 'group' && Array.isArray(button.items)) {
        return (
          <React.Fragment key={button.key || button.name}>
            <div className='router-header-item-mobile-group'>
              <AppIcon name={button.icon} />
              {t(button.name)}
            </div>
            {button.items.map((item) => (
              <button
                type='button'
                key={item.to}
                onClick={() => {
                  navigate(item.to);
                  setShowSidebar(false);
                }}
                className={`router-header-item-mobile router-header-item-mobile-child ${isRouteActive(item.to) ? 'router-header-group-active' : ''}`}
              >
                <AppIcon name={item.icon} />
                {t(item.name)}
              </button>
            ))}
          </React.Fragment>
        );
      }

      return (
        <button
          type='button'
          key={button.to || button.name}
          onClick={() => {
            navigate(button.to);
            setShowSidebar(false);
          }}
          className={`router-header-item-mobile ${isRouteActive(button.to) ? 'router-header-group-active' : ''}`}
        >
          {button.icon ? <AppIcon name={button.icon} /> : null}
          {t(button.name)}
        </button>
      );
    });
  };

  if (isMobile()) {
    return (
      <>
        <div
          className={[
            'router-header-menu',
            'router-header-menu-mobile',
            shouldFixHeader ? 'router-fixed-header' : '',
          ]
            .filter(Boolean)
            .join(' ')}
        >
          <div className={headerContainerClass}>
            <a
              href='https://www.yeying.pub'
              target='_blank'
              rel='noopener noreferrer'
              className='router-header-brand'
            >
              <img src={logo} alt='logo' />
            </a>
            <div className='router-header-actions'>
              <button
                type='button'
                className='router-header-mobile-toggle'
                onClick={() => setShowSidebar((previous) => !previous)}
              >
                <AppIcon name={showSidebar ? 'close' : 'sidebar'} />
              </button>
            </div>
          </div>
        </div>
        <AppDrawer
          open={showSidebar}
          onClose={() => setShowSidebar(false)}
          placement='right'
          width={320}
          title={
            currentWorkspace === 'admin'
              ? t('header.admin_workspace')
              : t('header.user_workspace')
          }
          className='router-header-mobile-drawer'
        >
          <div className='router-header-mobile-list'>
            {renderMobileButtons()}
            {currentWorkspace === 'user' && userState.user && (
              <AppButton
                className='router-page-button router-header-mobile-actions'
                onClick={() => {
                  setShowSidebar(false);
                  navigate('/workspace/start');
                }}
              >
                {t('workspace_start.title')}
              </AppButton>
            )}
            {hasAdminAccess && (
              <div className='router-header-mobile-workspace-switch'>
                <AppButton
                  className='router-page-button'
                  color={currentWorkspace === 'admin' ? 'blue' : undefined}
                  basic={currentWorkspace !== 'admin'}
                  fluid
                  onClick={() => goToWorkspace('admin')}
                >
                  {t('header.admin_workspace')}
                </AppButton>
                <AppButton
                  className='router-page-button'
                  color={currentWorkspace === 'user' ? 'blue' : undefined}
                  basic={currentWorkspace !== 'user'}
                  fluid
                  onClick={() => goToWorkspace('user')}
                >
                  {t('header.user_workspace')}
                </AppButton>
              </div>
            )}
            <AppSelect
              className='router-header-mobile-language router-section-dropdown'
              options={languageOptions}
              value={i18n.language}
              onChange={(_, { value }) => changeLanguage(value)}
            />
            <div className='router-header-mobile-auth'>
              {userState.user ? (
                <AppButton
                  className='router-page-button router-header-mobile-actions'
                  onClick={logout}
                >
                  {t('header.logout')}
                </AppButton>
              ) : (
                <>
                  <AppButton
                    className='router-page-button'
                    onClick={() => {
                      setShowSidebar(false);
                      navigate('/login');
                    }}
                  >
                    {t('header.login')}
                  </AppButton>
                  {passwordRegisterEnabled && (
                    <AppButton
                      className='router-page-button'
                      onClick={() => {
                        setShowSidebar(false);
                        navigate('/register');
                      }}
                    >
                      {t('header.register')}
                    </AppButton>
                  )}
                </>
              )}
            </div>
          </div>
        </AppDrawer>
      </>
    );
  }

  return (
    <div
      className={[
        'router-header-menu',
        shouldFixHeader ? 'router-fixed-header' : '',
      ]
        .filter(Boolean)
        .join(' ')}
    >
      <div className={headerContainerClass}>
        <a
          href='https://www.yeying.pub'
          target='_blank'
          rel='noopener noreferrer'
          className='router-header-brand hide-on-mobile'
        >
          <img src={logo} alt='logo' />
        </a>
        {!hideNavButtons ? (
          <div className='router-header-nav'>
            <AppNavMenu
              mode='horizontal'
              className='router-header-nav-menu'
              items={desktopNavItems}
              selectedKeys={desktopSelectedKeys}
              onClick={({ key }) => {
                if (typeof key === 'string' && key.startsWith('/')) {
                  navigate(key);
                }
              }}
            />
          </div>
        ) : null}
        <div className='router-header-actions'>
          {currentWorkspace === 'user' && userState.user ? (
            <AppButton
              type='button'
              className='router-header-quick-action'
              onClick={() => navigate('/workspace/start')}
            >
              {t('workspace_start.title')}
            </AppButton>
          ) : null}
          {hasAdminAccess && (
            <div className='router-header-dropdown router-header-trigger'>
              <AppMenuDropdown
                items={[
                  {
                    key: 'admin',
                    active: currentWorkspace === 'admin',
                    label: t('header.admin_workspace'),
                    onClick: () => goToWorkspace('admin'),
                  },
                  {
                    key: 'user',
                    active: currentWorkspace === 'user',
                    label: t('header.user_workspace'),
                    onClick: () => goToWorkspace('user'),
                  },
                ]}
              >
                <span className='router-header-toolbar-chip'>
                  {currentWorkspace === 'admin'
                    ? t('header.admin_workspace')
                    : t('header.user_workspace')}
                </span>
              </AppMenuDropdown>
            </div>
          )}
          <div className='router-header-dropdown router-header-trigger'>
            <AppMenuDropdown
              items={languageOptions.map((option) => ({
                key: option.value,
                active: i18n.language === option.value,
                label: option.text,
                onClick: () => changeLanguage(option.value),
              }))}
            >
              <span className='router-header-toolbar-icon'>
                <AppIcon name='language' className='router-header-trigger-icon' />
              </span>
            </AppMenuDropdown>
          </div>
          {userState.user ? (
            <div className='router-header-dropdown router-header-trigger'>
              <AppMenuDropdown
                items={[
                  {
                    key: 'logout',
                    label: t('header.logout'),
                    onClick: logout,
                  },
                ]}
              >
                <span className='router-header-toolbar-chip'>
                  {userState.user.username}
                </span>
              </AppMenuDropdown>
            </div>
          ) : (
            <Link to='/login' className='router-header-user-link'>
              {t('header.login')}
            </Link>
          )}
        </div>
      </div>
    </div>
  );
};

export default Header;
