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
import '../index.css';

const ADMIN_HEADER_BUTTONS = [
  {
    name: 'header.dashboard',
    to: '/admin/dashboard',
    icon: 'chart bar',
  },
  {
    name: 'header.model_providers',
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
    name: 'header.setting',
    to: '/admin/setting',
    icon: 'setting',
  },
];

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
    to: '/workspace/topup',
    icon: 'cart',
  },
  {
    name: 'header.log',
    to: '/workspace/log',
    icon: 'book',
  },
  {
    name: 'header.setting',
    to: '/workspace/setting',
    icon: 'setting',
  },
  {
    name: 'header.about',
    to: '/workspace/about',
    icon: 'info circle',
  },
];

const Header = ({ workspace = 'user' }) => {
  const { t, i18n } = useTranslation();
  const [userState, userDispatch] = useContext(UserContext);
  const navigate = useNavigate();
  const location = useLocation();

  const [showSidebar, setShowSidebar] = useState(false);
  const logo = getLogo();
  const shouldFixHeader = Boolean(userState?.user);

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
  const buttons = (() => {
    const baseButtons =
      currentWorkspace === 'admin' ? ADMIN_HEADER_BUTTONS : USER_HEADER_BUTTONS;
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

  const isActive = (path) => {
    if (location.pathname === path) {
      return true;
    }
    return location.pathname.startsWith(`${path}/`);
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
            style={{ fontSize: '15px' }}
            active={isActive(button.to)}
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
          style={{
            fontSize: '15px',
            fontWeight: '400',
            color: '#666',
          }}
          active={isActive(button.to)}
        >
          <Icon
            name={button.icon}
            style={{ marginRight: '4px' }}
          />
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
          size='large'
          className={shouldFixHeader ? 'router-fixed-header' : ''}
          style={
            showSidebar
              ? {
                  borderBottom: 'none',
                  marginBottom: '0',
                  borderTop: 'none',
                  height: '51px',
                }
              : { borderTop: 'none', height: '52px' }
          }
        >
          <Container
            style={{
              width: '100%',
              maxWidth: isMobile() ? '100%' : '1200px',
              padding: isMobile() ? '0 10px' : '0 20px',
            }}
          >
            <Menu.Item
              as='a'
              href='https://www.yeying.pub'
              target='_blank'
              rel='noopener noreferrer'
            >
              <img
                src={logo}
                alt='logo'
              />
            </Menu.Item>
            <Menu.Menu position='right'>
              <Menu.Item onClick={toggleSidebar}>
                <Icon name={showSidebar ? 'close' : 'sidebar'} />
              </Menu.Item>
            </Menu.Menu>
          </Container>
        </Menu>
        {showSidebar ? (
          <Segment style={{ marginTop: 0, borderTop: '0' }}>
            <Menu
              secondary
              vertical
              style={{ width: '100%', margin: 0 }}
            >
              {renderButtons(true)}
              {hasAdminAccess && (
                <Menu.Item>
                  <Button.Group
                    fluid
                    size='small'
                  >
                    <Button
                      basic={currentWorkspace !== 'admin'}
                      primary={currentWorkspace === 'admin'}
                      onClick={() => goToWorkspace('admin')}
                    >
                      {t('header.admin_workspace')}
                    </Button>
                    <Button
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
                  selection
                  trigger={
                    <Icon
                      name='language'
                      style={{ margin: 0, fontSize: '18px' }}
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
                    onClick={logout}
                    style={{ color: '#666666' }}
                  >
                    {t('header.logout')}
                  </Button>
                ) : (
                  <>
                    <Button
                      onClick={() => {
                        setShowSidebar(false);
                        navigate('/login');
                      }}
                    >
                      {t('header.login')}
                    </Button>
                    <Button
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
        className={shouldFixHeader ? 'router-fixed-header' : ''}
        style={{
          borderTop: 'none',
          boxShadow: 'rgba(0, 0, 0, 0.04) 0px 2px 12px 0px',
          border: 'none',
        }}
      >
        <Container
          style={{
            width: '100%',
            maxWidth: isMobile() ? '100%' : '1200px',
            padding: isMobile() ? '0 10px' : '0 20px',
          }}
        >
          <Menu.Item
            as='a'
            href='https://www.yeying.pub'
            target='_blank'
            rel='noopener noreferrer'
            className={'hide-on-mobile'}
          >
            <img
              src={logo}
              alt='logo'
            />
          </Menu.Item>
          {renderButtons(false)}
          <Menu.Menu position='right'>
            {hasAdminAccess && (
              <Dropdown
                item
                text={
                  currentWorkspace === 'admin'
                    ? t('header.admin_workspace')
                    : t('header.user_workspace')
                }
                pointing
                className='link item'
                style={{
                  fontSize: '15px',
                  fontWeight: '400',
                  color: '#666',
                }}
              >
                <Dropdown.Menu>
                  <Dropdown.Item
                    active={currentWorkspace === 'admin'}
                    onClick={() => goToWorkspace('admin')}
                    style={{
                      fontSize: '15px',
                      fontWeight: '400',
                      color: '#666',
                    }}
                  >
                    {t('header.admin_workspace')}
                  </Dropdown.Item>
                  <Dropdown.Item
                    active={currentWorkspace === 'user'}
                    onClick={() => goToWorkspace('user')}
                    style={{
                      fontSize: '15px',
                      fontWeight: '400',
                      color: '#666',
                    }}
                  >
                    {t('header.user_workspace')}
                  </Dropdown.Item>
                </Dropdown.Menu>
              </Dropdown>
            )}
            <Dropdown
              item
              trigger={
                <Icon
                  name='language'
                  style={{ margin: 0, fontSize: '18px' }}
                />
              }
              options={languageOptions}
              value={i18n.language}
              onChange={(_, { value }) => changeLanguage(value)}
              style={{
                fontSize: '16px',
                fontWeight: '400',
                color: '#666',
                padding: '0 10px',
              }}
            />
            {userState.user ? (
              <Dropdown
                text={userState.user.username}
                pointing
                className='link item'
                style={{
                  fontSize: '15px',
                  fontWeight: '400',
                  color: '#666',
                }}
              >
                <Dropdown.Menu>
                  <Dropdown.Item
                    onClick={logout}
                    style={{
                      fontSize: '15px',
                      fontWeight: '400',
                      color: '#666',
                    }}
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
                className='btn btn-link'
                style={{
                  fontSize: '15px',
                  fontWeight: '400',
                  color: '#666',
                }}
              />
            )}
          </Menu.Menu>
        </Container>
      </Menu>
    </>
  );
};

export default Header;
