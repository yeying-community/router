import React, { useContext, useEffect, useState } from 'react';
import { Button, Divider, Form, Image, Message } from 'semantic-ui-react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { UserContext } from '../context/User';
import { StatusContext } from '../context/Status';
import { API, getLogo, showError, showSuccess, showWarning } from '../helpers';
import { loginWithWallet } from '../services/web3Auth';
import './LoginForm.css';

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
  const navigate = useNavigate();
  const logo = getLogo();
  const loginBannerText =
    '帮助您更好的管理、分发、路由和使用各大模型厂商接口服务';
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
      const selfResp = await API.get('/api/v1/public/user/self');
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

  function handleChange(e) {
    const { name, value } = e.target;
    setInputs((previousInputs) => ({ ...previousInputs, [name]: value }));
  }

  async function handleSubmit() {
    if (username && password) {
      const res = await API.post(`/api/v1/public/user/login`, {
        username,
        password,
      });
      const { success, message, data } = res.data;
      if (success) {
        userDispatch({ type: 'login', payload: data });
        localStorage.setItem('user', JSON.stringify(data));
        navigate('/token');
        showSuccess(t('messages.success.login'));
        if (username === 'root' && password === '123456') {
          showWarning(t('messages.error.root_password'));
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
    <div className='router-login-page'>
      <div className='router-login-floating-container'>
        <div className='router-login-top-banner'>
          <div className='router-login-top-banner-inner'>
            <Image src={logo} className='router-login-top-banner-logo' />
            <span>
              {loginBannerText}
              <a href='https://www.yeying.pub' target='_blank' rel='noopener noreferrer'>
                了解夜莺社区
              </a>
            </span>
          </div>
        </div>

        <div className='router-login-hero'>
          <div className='router-login-card'>
            <div className='router-login-section'>
              <Button
                fluid
                size='large'
                className='router-login-main-btn router-wallet-button'
                onClick={onWalletLoginClicked}
                disabled={walletLoginDisabled}
              >
                {t('auth.login.wallet_button', '使用钱包登录')}
              </Button>
              {walletLoginDisabled && (
                <Message warning size='small'>
                  {t('auth.login.wallet_disabled', '钱包登录未开启，请联系管理员')}
                </Message>
              )}
            </div>

            <Divider horizontal>或</Divider>

            <div className='router-login-section'>
              {walletLoginEnabled && (
                <Button
                  basic
                  fluid
                  size='large'
                  className='router-login-main-btn router-password-toggle'
                  onClick={() =>
                    setShowPasswordLogin((previousState) => !previousState)
                  }
                >
                  使用账密登陆
                </Button>
              )}

              {showPasswordLogin && (
                <>
                  <Form size='large' className='router-login-form'>
                    <Form.Input
                      fluid
                      icon='user'
                      iconPosition='left'
                      placeholder={t('auth.login.username')}
                      name='username'
                      value={username}
                      onChange={handleChange}
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
                    />
                    <Button
                      fluid
                      size='large'
                      className='router-password-submit'
                      onClick={handleSubmit}
                    >
                      {t('auth.login.button')}
                    </Button>
                  </Form>

                  <div className='router-login-links'>
                    <div>
                      {t('auth.login.forgot_password')}
                      <Link to='/reset'>{t('auth.login.reset_password')}</Link>
                    </div>
                    <div>
                      {t('auth.login.no_account')}
                      <Link to='/register'>{t('auth.login.register')}</Link>
                    </div>
                  </div>
                </>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default LoginForm;
