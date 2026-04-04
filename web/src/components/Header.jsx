import React, { useContext, useEffect, useState } from 'react';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { UserContext } from '../context/User';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Container,
  Dropdown,
  Icon,
  Menu,
  Segment,
} from 'semantic-ui-react';
import { API, getLogo, isAdmin, isMobile, showSuccess } from '../helpers';
import { WEB3_TOKEN_STORAGE_KEY } from '../helpers/web3';
import { logoutWallet } from '../services/web3Auth';
import {
  ADMIN_MENU_GROUPS,
  isAdminGroupActive,
  isAdminRouteActive,
} from '../constants/adminMenu';
import '../index.css';

const USER_HEADER_BUTTONS = [
  {
    name: 'header.dashboard',
    to: '/workspace/dashboard',
    icon: 'chart bar',
  },
  {
    name: 'header.token',
    to: '/workspace/token',
    icon: 'key',
  },
  {
    name: 'header.topup',
    to: '/workspace/topup?tab=balance',
    icon: 'cart',
  },
  {
    name: 'header.log',
    to: '/workspace/log',
    icon: 'book',
  },
  {
    name: 'header.task',
    to: '/workspace/task',
    icon: 'tasks',
  },
  {
    name: 'header.setting',
    to: '/workspace/setting',
    icon: 'setting',
  },
];

const Header = ({ workspace = 'user', hideNavButtons = false }) => {
  const { t, i18n } = useTranslation();
  const [userState, userDispatch] = useContext(UserContext);
  const navigate = useNavigate();
  const location = useLocation();

  const [showSidebar, setShowSidebar] = useState(false);
  const logo = getLogo();
  const shouldFixHeader = Boolean(userState?.user);
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

  const currentWorkspace = workspace === 'admin' ? 'admin' : 'user';
  const hasAdminAccess = isAdmin();
  const adminFlatButtons = ADMIN_MENU_GROUPS.flatMap(
    (group) => group.items,
  );
  const buttons = (() => {
    const baseButtons =
      currentWorkspace === 'admin' ? adminFlatButtons : USER_HEADER_BUTTONS;
    const next = [...baseButtons];
    if (currentWorkspace === 'user' && localStorage.getItem('chat_link')) {
      next.splice(2, 0, {
        name: 'header.chat',
        to: '/workspace/chat',
        icon: 'comments',
      });
    }
    return next;
  })();

  const isRouteActive = (to) => {
    if (currentWorkspace === 'admin') {
      return isAdminRouteActive(location, to);
    }
    const [path] = String(to || '').split('?');
    if (!path) {
      return false;
    }
    return location.pathname === path || location.pathname.startsWith(`${path}/`);
  };

  const renderAdminDesktopButtons = () => {
    return ADMIN_MENU_GROUPS.map((group) => {
      if (!group?.items?.length) {
        return null;
      }
      if (group.items.length === 1) {
        const item = group.items[0];
        return (
          <Menu.Item
            key={group.key}
            as={Link}
            to={item.to}
            className={`router-header-item ${isAdminGroupActive(location, group) ? 'router-header-group-active' : ''}`}
            active={isRouteActive(item.to)}
          >
            <Icon name={group.icon || item.icon} />
            {t(group.name)}
          </Menu.Item>
        );
      }
      return (
        <Dropdown
          key={group.key}
          className={`link item router-header-dropdown router-header-trigger router-header-item ${isAdminGroupActive(location, group) ? 'router-header-group-active' : ''}`}
          item
          pointing
          trigger={
            <span>
              <Icon name={group.icon} />
              {t(group.name)}
            </span>
          }
        >
          <Dropdown.Menu>
            {group.items.map((item) => (
              <Dropdown.Item
                key={item.to}
                active={isRouteActive(item.to)}
                onClick={() => navigate(item.to)}
                className='router-header-item'
              >
                <Icon name={item.icon} />
                {t(item.name)}
              </Dropdown.Item>
            ))}
          </Dropdown.Menu>
        </Dropdown>
      );
    });
  };

  const toggleSidebar = () => {
    setShowSidebar(!showSidebar);
  };

  const goToWorkspace = (targetWorkspace) => {
    if (targetWorkspace === 'admin') {
      navigate('/admin/dashboard');
    } else {
      navigate('/workspace/token');
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
    showSuccess('注销成功!');
    userDispatch({ type: 'logout' });
    localStorage.removeItem('user');
    localStorage.removeItem(WEB3_TOKEN_STORAGE_KEY);
    localStorage.removeItem('wallet_token_expires_at');
    navigate('/login');
  }

  const renderButtons = (mobileView) => {
    return buttons.map((button) => {
      if (mobileView) {
        return (
          <Menu.Item
            key={button.name}
            onClick={() => {
              navigate(button.to);
              setShowSidebar(false);
            }}
            className='router-header-item-mobile'
            active={isRouteActive(button.to)}
          >
            {t(button.name)}
          </Menu.Item>
        );
      }
      return (
        <Menu.Item
          key={button.name}
          as={Link}
          to={button.to}
          className='router-header-item'
          active={isRouteActive(button.to)}
        >
          <Icon name={button.icon} />
          {t(button.name)}
        </Menu.Item>
      );
    });
  };

  const languageOptions = [
    { key: 'zh', text: '中文', value: 'zh' },
    { key: 'en', text: 'English', value: 'en' },
  ];

  const changeLanguage = (language) => {
    i18n.changeLanguage(language);
  };

  if (isMobile()) {
    return (
      <>
        <Menu
          borderless
          className={[
            'router-header-menu',
            'router-header-menu-mobile',
            shouldFixHeader ? 'router-fixed-header' : '',
            showSidebar ? 'router-header-menu-mobile-open' : '',
          ]
            .filter(Boolean)
            .join(' ')}
        >
          <Container className={headerContainerClass}>
            <Menu.Item
              as='a'
              href='https://www.yeying.pub'
              target='_blank'
              rel='noopener noreferrer'
            >
              <img src={logo} alt='logo' />
            </Menu.Item>
            <Menu.Menu position='right'>
              <Menu.Item onClick={toggleSidebar}>
                <Icon name={showSidebar ? 'close' : 'sidebar'} />
              </Menu.Item>
            </Menu.Menu>
          </Container>
        </Menu>
        {showSidebar ? (
          <Segment className='router-header-mobile-segment'>
            <Menu secondary vertical className='router-header-mobile-list'>
              {renderButtons(true)}
              {hasAdminAccess && (
                <Menu.Item>
                  <Button.Group fluid>
                    <Button
                      className='router-page-button'
                      basic={currentWorkspace !== 'admin'}
                      primary={currentWorkspace === 'admin'}
                      onClick={() => goToWorkspace('admin')}
                    >
                      {t('header.admin_workspace')}
                    </Button>
                    <Button
                      className='router-page-button'
                      basic={currentWorkspace !== 'user'}
                      primary={currentWorkspace === 'user'}
                      onClick={() => goToWorkspace('user')}
                    >
                      {t('header.user_workspace')}
                    </Button>
                  </Button.Group>
                </Menu.Item>
              )}
              <Menu.Item>
                <Dropdown
                  className='router-header-dropdown'
                  selection
                  trigger={
                    <Icon
                      name='language'
                      className='router-header-trigger-icon'
                    />
                  }
                  options={languageOptions}
                  value={i18n.language}
                  onChange={(_, { value }) => changeLanguage(value)}
                />
              </Menu.Item>
              <Menu.Item>
                {userState.user ? (
                  <Button
                    className='router-page-button router-header-mobile-actions'
                    onClick={logout}
                  >
                    {t('header.logout')}
                  </Button>
                ) : (
                  <>
                    <Button
                      className='router-page-button'
                      onClick={() => {
                        setShowSidebar(false);
                        navigate('/login');
                      }}
                    >
                      {t('header.login')}
                    </Button>
                    <Button
                      className='router-page-button'
                      onClick={() => {
                        setShowSidebar(false);
                        navigate('/register');
                      }}
                    >
                      {t('header.register')}
                    </Button>
                  </>
                )}
              </Menu.Item>
            </Menu>
          </Segment>
        ) : (
          <></>
        )}
      </>
    );
  }

  return (
    <>
      <Menu
        borderless
        className={[
          'router-header-menu',
          shouldFixHeader ? 'router-fixed-header' : '',
        ]
          .filter(Boolean)
          .join(' ')}
      >
        <Container className={headerContainerClass}>
          <Menu.Item
            as='a'
            href='https://www.yeying.pub'
            target='_blank'
            rel='noopener noreferrer'
            className={'hide-on-mobile'}
          >
            <img src={logo} alt='logo' />
          </Menu.Item>
          {!hideNavButtons
            ? currentWorkspace === 'admin'
              ? renderAdminDesktopButtons()
              : renderButtons(false)
            : null}
          <Menu.Menu position='right'>
            {hasAdminAccess && (
              <Dropdown
                className='link item router-header-dropdown router-header-trigger'
                item
                text={
                  currentWorkspace === 'admin'
                    ? t('header.admin_workspace')
                    : t('header.user_workspace')
                }
                pointing
              >
                <Dropdown.Menu>
                  <Dropdown.Item
                    active={currentWorkspace === 'admin'}
                    onClick={() => goToWorkspace('admin')}
                    className='router-header-item'
                  >
                    {t('header.admin_workspace')}
                  </Dropdown.Item>
                  <Dropdown.Item
                    active={currentWorkspace === 'user'}
                    onClick={() => goToWorkspace('user')}
                    className='router-header-item'
                  >
                    {t('header.user_workspace')}
                  </Dropdown.Item>
                </Dropdown.Menu>
              </Dropdown>
            )}
            <Dropdown
              className='router-header-dropdown router-header-trigger'
              item
              trigger={
                <Icon name='language' className='router-header-trigger-icon' />
              }
              options={languageOptions}
              value={i18n.language}
              onChange={(_, { value }) => changeLanguage(value)}
            />
            {userState.user ? (
              <Dropdown
                className='link item router-header-dropdown router-header-trigger'
                text={userState.user.username}
                pointing
              >
                <Dropdown.Menu>
                  <Dropdown.Item
                    onClick={logout}
                    className='router-header-item'
                  >
                    {t('header.logout')}
                  </Dropdown.Item>
                </Dropdown.Menu>
              </Dropdown>
            ) : (
              <Menu.Item
                name={t('header.login')}
                as={Link}
                to='/login'
                className='btn btn-link router-header-user-link'
              />
            )}
          </Menu.Menu>
        </Container>
      </Menu>
    </>
  );
};

export default Header;
