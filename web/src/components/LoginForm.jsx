import React, { useContext, useEffect, useState } from 'react';
import { Button, Divider, Form, Grid, Header, Image, Message, Card } from 'semantic-ui-react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { UserContext } from '../context/User';
import { StatusContext } from '../context/Status';
import { API, getLogo, showError, showSuccess, showWarning } from '../helpers';
import { loginWithWallet } from '../services/web3Auth';

const LoginForm = () => {
  const { t } = useTranslation();
  const [inputs, setInputs] = useState({
    username: '',
    password: '',
  });
  const [searchParams] = useSearchParams();
  const { username, password } = inputs;
  const [, userDispatch] = useContext(UserContext);
  const [statusState] = useContext(StatusContext);
  let navigate = useNavigate();
  const logo = getLogo();
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
  const walletLoginDisabled = status?.wallet_login === false;
  const walletLoginEnabled = !walletLoginDisabled;
  const [showPasswordLogin, setShowPasswordLogin] = useState(walletLoginDisabled);

  useEffect(() => {
    if (searchParams.get('expired')) {
      showError(t('messages.error.login_expired'));
    }
  }, [searchParams, t]);

  // 微信登录已下线

  const onWalletLoginClicked = async () => {
    try {
      if (status?.wallet_login === false) {
        showError(t('auth.login.wallet_disabled') || '钱包登录未开启');
        return;
      }
      const loginResult = await loginWithWallet();
      const payload = loginResult?.response?.data || loginResult?.response;
      if (payload?.expiresAt) {
        localStorage.setItem(
          'wallet_token_expires_at',
          new Date(payload.expiresAt).toISOString()
        );
      }
      const selfResp = await API.get('/api/user/self');
      const { success, data, message } = selfResp?.data || {};
      if (!success || !data) {
        showError(message || '未获取到用户信息');
        return;
      }
      const userData = { ...data, token: loginResult.token };
      userDispatch({ type: 'login', payload: userData });
      localStorage.setItem('user', JSON.stringify(userData));
      navigate('/token');
      showSuccess(t('messages.success.login'));
    } catch (error) {
      if (error?.code === 4001) {
        showError('用户拒绝了请求');
      } else {
        showError(error.message || '钱包登录失败');
      }
    }
  };

  // 微信登录已移除

  function handleChange(e) {
    const { name, value } = e.target;
    setInputs((inputs) => ({ ...inputs, [name]: value }));
  }

  async function handleSubmit(e) {
    if (username && password) {
      const res = await API.post(`/api/user/login`, {
        username,
        password,
      });
      const { success, message, data } = res.data;
      if (success) {
        userDispatch({ type: 'login', payload: data });
        localStorage.setItem('user', JSON.stringify(data));
        if (username === 'root' && password === '123456') {
          navigate('/user/edit');
          showSuccess(t('messages.success.login'));
          showWarning(t('messages.error.root_password'));
        } else {
          navigate('/token');
          showSuccess(t('messages.success.login'));
        }
      } else {
        showError(message);
      }
    }
  }

  useEffect(() => {
    if (walletLoginDisabled) {
      setShowPasswordLogin(true);
    }
  }, [walletLoginDisabled]);

  return (
    <Grid textAlign='center' style={{ marginTop: '48px' }}>
      <Grid.Column style={{ maxWidth: 450 }}>
        <Card
          fluid
          className='chart-card'
          style={{ boxShadow: '0 1px 3px rgba(0,0,0,0.12)' }}
        >
          <Card.Content>
            <Card.Header>
              <Header
                as='h2'
                textAlign='center'
                style={{ marginBottom: '1.5em' }}
              >
                <Image src={logo} style={{ marginBottom: '10px' }} />
                <Header.Content>{t('auth.login.title')}</Header.Content>
              </Header>
            </Card.Header>
            <Button
              fluid
              size='large'
              color='orange'
              onClick={onWalletLoginClicked}
              disabled={walletLoginDisabled}
              style={{ marginBottom: '0.75em' }}
            >
              {t('auth.login.wallet_button', '使用钱包登录')}
            </Button>
            {walletLoginDisabled && (
              <Message warning size='small' style={{ marginBottom: '1.25em' }}>
                {t(
                  'auth.login.wallet_disabled',
                  '钱包登录未开启，请联系管理员'
                )}
              </Message>
            )}

            <Divider horizontal style={{ color: '#666', fontSize: '0.9em' }}>
              {t('auth.login.password_title', '或使用账号密码')}
            </Divider>

            {!showPasswordLogin && walletLoginEnabled && (
              <Button
                basic
                fluid
                onClick={() => setShowPasswordLogin(true)}
                style={{ marginBottom: '1em' }}
              >
                {t('auth.login.password_toggle', '使用账号密码登录')}
              </Button>
            )}

            {showPasswordLogin && (
              <>
                <Form size='large'>
                  <Form.Input
                    fluid
                    icon='user'
                    iconPosition='left'
                    placeholder={t('auth.login.username')}
                    name='username'
                    value={username}
                    onChange={handleChange}
                    style={{ marginBottom: '1em' }}
                  />
                  <Form.Input
                    fluid
                    icon='lock'
                    iconPosition='left'
                    placeholder={t('auth.login.password')}
                    name='password'
                    type='password'
                    value={password}
                    onChange={handleChange}
                    style={{ marginBottom: '1.5em' }}
                  />
                  <Button
                    fluid
                    size='large'
                    style={{
                      background: '#2F73FF',
                      color: 'white',
                      marginBottom: '1.5em',
                    }}
                    onClick={handleSubmit}
                  >
                    {t('auth.login.button')}
                  </Button>
                </Form>

                <Message
                  style={{ background: 'transparent', boxShadow: 'none' }}
                >
                  <div
                    style={{
                      display: 'flex',
                      justifyContent: 'space-between',
                      fontSize: '0.9em',
                      color: '#666',
                    }}
                  >
                    <div>
                      {t('auth.login.forgot_password')}
                      <Link
                        to='/reset'
                        style={{ color: '#2185d0', marginLeft: '2px' }}
                      >
                        {t('auth.login.reset_password')}
                      </Link>
                    </div>
                    <div>
                      {t('auth.login.no_account')}
                      <Link
                        to='/register'
                        style={{ color: '#2185d0', marginLeft: '2px' }}
                      >
                        {t('auth.login.register')}
                      </Link>
                    </div>
                  </div>
                </Message>
              </>
            )}
          </Card.Content>
        </Card>
      </Grid.Column>
    </Grid>
  );
};

export default LoginForm;
