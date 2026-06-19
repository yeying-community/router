import React, { useContext, useEffect, useRef, useState } from 'react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { UserContext } from '../context/User';
import { StatusContext } from '../context/Status';
import { API, getLogo, showError } from '../helpers';
import { toastConstants } from '../constants';
import {
  focusWalletPendingApproval,
  isWalletUserRejectedError,
  loginWithWallet,
} from '../services/web3Auth';
import { useWalletProviderStatus } from '../hooks/useWalletProviderStatus';
import { AppAlert, AppButton, AppDivider, AppInput } from '../router-ui';
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
  const passwordLoginDisabled = status?.password_login_enabled === false;
  const walletLoginEnabled = !walletLoginDisabled;
  const passwordLoginEnabled = !passwordLoginDisabled;
  const passwordRegisterEnabled =
    status?.register_enabled !== false &&
    status?.password_register_enabled !== false;
  const [showPasswordLogin, setShowPasswordLogin] =
    useState(walletLoginDisabled && passwordLoginEnabled);
  const [walletLoginSubmitting, setWalletLoginSubmitting] = useState(false);
  const [walletLoginAwaitingApproval, setWalletLoginAwaitingApproval] =
    useState(false);
  const walletLoginPromiseRef = useRef(null);
  const walletProviderStatus = useWalletProviderStatus();
  const resolveLandingPath = (role) =>
    Number(role) >= 10 ? '/admin/dashboard' : '/workspace/entry';

  useEffect(() => {
    const expiredMarker = searchParams.get('expired');
    if (expiredMarker) {
      const lastMarker = sessionStorage.getItem('last_login_expired_marker');
      if (lastMarker !== expiredMarker) {
        sessionStorage.setItem('last_login_expired_marker', expiredMarker);
        showError(t('messages.error.login_expired'), {
          autoClose: Math.floor(toastConstants.ERROR_TIMEOUT / 2),
        });
      }
      const nextParams = new URLSearchParams(searchParams);
      nextParams.delete('expired');
      const nextSearch = nextParams.toString();
      navigate(`/login${nextSearch ? `?${nextSearch}` : ''}`, {
        replace: true,
      });
      return;
    }
  }, [searchParams, t, navigate]);

  const onWalletLoginClicked = async () => {
    if (walletLoginSubmitting) {
      return;
    }
    if (walletLoginPromiseRef.current) {
      const provider =
        walletProviderStatus.provider || (await walletProviderStatus.refresh());
      const pending = await focusWalletPendingApproval(provider || undefined);
      if (!pending?.focused) {
        showError('请在钱包中完成签名，或重新发起登录');
      }
      return;
    }

    setWalletLoginSubmitting(true);
    try {
      if (status?.wallet_login === false) {
        showError(t('auth.login.wallet_disabled') || '钱包登录未开启');
        return;
      }
      await walletProviderStatus.refresh();
      setWalletLoginAwaitingApproval(true);
      const loginTask = loginWithWallet();
      walletLoginPromiseRef.current = loginTask;
      setWalletLoginSubmitting(false);
      const loginResult = await loginTask;
      setWalletLoginAwaitingApproval(false);
      const payload = loginResult?.response?.data || loginResult?.response;
      if (payload?.expiresAt) {
        localStorage.setItem(
          'wallet_token_expires_at',
          new Date(payload.expiresAt).toISOString(),
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
      navigate(resolveLandingPath(userData.role));
    } catch (error) {
      setWalletLoginAwaitingApproval(false);
      if (isWalletUserRejectedError(error)) {
        showError('用户拒绝了请求');
      } else {
        showError(error.message || '钱包登录失败');
      }
    } finally {
      walletLoginPromiseRef.current = null;
      setWalletLoginSubmitting(false);
    }
  };

  function handleChange(e) {
    const { name, value } = e.target;
    setInputs((previousInputs) => ({ ...previousInputs, [name]: value }));
  }

  async function handleSubmit() {
    if (passwordLoginDisabled) {
      showError(
        t('auth.login.password_disabled', '用户名密码登录未开启，请联系管理员'),
      );
      return;
    }
    if (username && password) {
      const res = await API.post(`/api/v1/public/user/login`, {
        username,
        password,
      });
      const { success, message, data } = res.data;
      if (success) {
        userDispatch({ type: 'login', payload: data });
        localStorage.setItem('user', JSON.stringify(data));
        navigate(resolveLandingPath(data.role));
      } else {
        showError(message);
      }
    }
  }

  useEffect(() => {
    if (walletLoginDisabled && passwordLoginEnabled) {
      setShowPasswordLogin(true);
    }
  }, [walletLoginDisabled, passwordLoginEnabled]);

  return (
    <div className='router-login-page'>
      <div className='router-login-floating-container'>
        <div className='router-login-top-banner'>
          <div className='router-login-top-banner-inner'>
            <img src={logo} className='router-login-top-banner-logo' alt='' />
            <span>
              {loginBannerText}
              <a
                href='https://www.yeying.pub'
                target='_blank'
                rel='noopener noreferrer'
              >
                了解夜莺社区
              </a>
            </span>
          </div>
        </div>

        <div className='router-login-hero'>
          <div className='router-login-card'>
            <div className='router-login-section'>
              <AppButton
                fluid
                className='router-login-main-btn router-auth-button router-wallet-button'
                onClick={onWalletLoginClicked}
                disabled={
                  walletLoginDisabled ||
                  walletLoginSubmitting ||
                  (!walletProviderStatus.detecting && !walletProviderStatus.available)
                }
                loading={walletLoginSubmitting || walletProviderStatus.detecting}
              >
                {walletLoginAwaitingApproval
                  ? '请在钱包中确认签名'
                  : t('auth.login.wallet_button', '使用钱包登录')}
              </AppButton>
              {walletLoginDisabled && (
                <AppAlert
                  type='warning'
                  showIcon
                  className='router-auth-message'
                  title={t(
                    'auth.login.wallet_disabled',
                    '钱包登录未开启，请联系管理员',
                  )}
                />
              )}
              {!walletLoginDisabled &&
                !walletProviderStatus.detecting &&
                !walletProviderStatus.available && (
                  <AppAlert
                    type='warning'
                    showIcon
                    className='router-auth-message'
                    title={t(
                      'auth.login.wallet_not_detected',
                      '未检测到钱包插件，请安装或启用钱包插件后重试',
                    )}
                  />
                )}
            </div>

            <AppDivider horizontal>或</AppDivider>

            <div className='router-login-section'>
              {walletLoginEnabled && passwordLoginEnabled && (
                <AppButton
                  basic
                  fluid
                  className='router-login-main-btn router-auth-button router-password-toggle'
                  onClick={() =>
                    setShowPasswordLogin((previousState) => !previousState)
                  }
                >
                  {t('auth.login.password_button', '使用账密登录')}
                </AppButton>
              )}

              {passwordLoginDisabled && (
                <AppAlert
                  type='warning'
                  showIcon
                  className='router-auth-message'
                  title={t(
                    'auth.login.password_disabled',
                    '用户名密码登录未开启，请联系管理员',
                  )}
                />
              )}

              {showPasswordLogin && passwordLoginEnabled && (
                <>
                  <div className='router-login-form router-auth-form'>
                    <AppInput
                      className='router-auth-input'
                      fluid
                      icon='user'
                      iconPosition='left'
                      placeholder={t('auth.login.username')}
                      name='username'
                      value={username}
                      onChange={handleChange}
                    />
                    <AppInput
                      className='router-auth-input'
                      fluid
                      icon='lock'
                      iconPosition='left'
                      placeholder={t('auth.login.password')}
                      name='password'
                      type='password'
                      value={password}
                      onChange={handleChange}
                    />
                    <AppButton
                      fluid
                      className='router-auth-button router-password-submit'
                      onClick={handleSubmit}
                    >
                      {t('auth.login.button')}
                    </AppButton>
                  </div>

                  <div className='router-login-links'>
                    <div>
                      {t('auth.login.forgot_password')}
                      <Link to='/reset'>{t('auth.login.reset_password')}</Link>
                    </div>
                    {passwordRegisterEnabled && (
                      <div>
                        {t('auth.login.no_account')}
                        <Link to='/register'>{t('auth.login.register')}</Link>
                      </div>
                    )}
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
