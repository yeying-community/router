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
import {
  AppAlert,
  AppButton,
  AppDivider,
  AppInput,
  AppSelect,
} from '../router-ui';
import {
  rememberAuthRedirectPath,
  resolvePostLoginPath,
} from '../helpers/authRedirect';
import './LoginForm.css';

const WALLET_LOGIN_HISTORY_STORAGE_KEY = 'wallet_login_history';
const LAST_WALLET_LOGIN_ADDRESS_STORAGE_KEY = 'last_wallet_login_address';

const maskWalletAddress = (value) => {
  const normalized = String(value || '').trim();
  if (normalized.length <= 15) {
    return normalized;
  }
  return `${normalized.slice(0, 6)}...${normalized.slice(-6)}`;
};

const normalizeWalletAddressList = (items) => {
  if (!Array.isArray(items)) {
    return [];
  }
  const result = [];
  const seen = new Set();
  items.forEach((item) => {
    const normalized = String(item || '').trim();
    if (normalized === '' || seen.has(normalized.toLowerCase())) {
      return;
    }
    seen.add(normalized.toLowerCase());
    result.push(normalized);
  });
  return result;
};

const getStoredWalletLoginHistory = () => {
  if (typeof window === 'undefined') {
    return [];
  }
  try {
    const raw = window.localStorage.getItem(WALLET_LOGIN_HISTORY_STORAGE_KEY);
    if (!raw) {
      return [];
    }
    return normalizeWalletAddressList(JSON.parse(raw));
  } catch (error) {
    return [];
  }
};

const getStoredLastWalletAddress = () => {
  if (typeof window === 'undefined') {
    return '';
  }
  return String(
    window.localStorage.getItem(LAST_WALLET_LOGIN_ADDRESS_STORAGE_KEY) || '',
  ).trim();
};

const persistWalletLoginHistory = (address) => {
  const normalizedAddress = String(address || '').trim();
  if (normalizedAddress === '' || typeof window === 'undefined') {
    return;
  }
  const nextHistory = normalizeWalletAddressList([
    normalizedAddress,
    ...getStoredWalletLoginHistory(),
  ]).slice(0, 8);
  window.localStorage.setItem(
    WALLET_LOGIN_HISTORY_STORAGE_KEY,
    JSON.stringify(nextHistory),
  );
  window.localStorage.setItem(
    LAST_WALLET_LOGIN_ADDRESS_STORAGE_KEY,
    normalizedAddress,
  );
};

const LoginForm = () => {
  const { t } = useTranslation();
  const [inputs, setInputs] = useState({
    username: '',
    password: '',
  });
  const [walletAddressOptions, setWalletAddressOptions] = useState([]);
  const [selectedWalletAddress, setSelectedWalletAddress] = useState(
    getStoredLastWalletAddress(),
  );
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
  const [showPasswordLogin, setShowPasswordLogin] = useState(
    walletLoginDisabled && passwordLoginEnabled,
  );
  const [walletLoginSubmitting, setWalletLoginSubmitting] = useState(false);
  const [walletLoginAwaitingApproval, setWalletLoginAwaitingApproval] =
    useState(false);
  const walletLoginPromiseRef = useRef(null);
  const walletProviderStatus = useWalletProviderStatus();
  const resolveLandingPath = (role) =>
    Number(role) >= 10 ? '/admin/dashboard' : '/workspace/entry';

  useEffect(() => {
    rememberAuthRedirectPath(searchParams.get('redirect'));
  }, [searchParams]);

  useEffect(() => {
    const mergedAddresses = normalizeWalletAddressList([
      ...walletProviderStatus.accounts,
      ...getStoredWalletLoginHistory(),
    ]);
    setWalletAddressOptions(
      mergedAddresses.map((address) => ({
        key: address,
        value: address,
        label: maskWalletAddress(address),
      })),
    );
    setSelectedWalletAddress((current) => {
      const normalizedCurrent = String(current || '').trim();
      if (
        normalizedCurrent !== '' &&
        mergedAddresses.some(
          (address) =>
            address.toLowerCase() === normalizedCurrent.toLowerCase(),
        )
      ) {
        return normalizedCurrent;
      }
      const storedAddress = getStoredLastWalletAddress();
      if (
        storedAddress !== '' &&
        mergedAddresses.some(
          (address) => address.toLowerCase() === storedAddress.toLowerCase(),
        )
      ) {
        return storedAddress;
      }
      return mergedAddresses[0] || '';
    });
  }, [walletProviderStatus.accounts]);

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
      const loginTask = loginWithWallet(selectedWalletAddress);
      walletLoginPromiseRef.current = loginTask;
      setWalletLoginSubmitting(false);
      const loginResult = await loginTask;
      setWalletLoginAwaitingApproval(false);
      persistWalletLoginHistory(loginResult?.address || selectedWalletAddress);
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
      navigate(
        resolvePostLoginPath(searchParams, resolveLandingPath(userData.role)),
        { replace: true },
      );
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
        navigate(
          resolvePostLoginPath(searchParams, resolveLandingPath(data.role)),
          {
            replace: true,
          },
        );
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
              <div className='router-wallet-login-row'>
                <AppSelect
                  className='router-wallet-address-select'
                  fluid
                  search
                  clearable={false}
                  options={walletAddressOptions}
                  value={selectedWalletAddress || undefined}
                  placeholder={t(
                    'auth.login.wallet_address_placeholder',
                    '选择钱包地址',
                  )}
                  disabled={walletLoginSubmitting}
                  onChange={(_, { value }) =>
                    setSelectedWalletAddress(String(value || '').trim())
                  }
                />
                <AppButton
                  className='router-login-main-btn router-auth-button router-wallet-button'
                  onClick={onWalletLoginClicked}
                  disabled={
                    walletLoginDisabled ||
                    walletLoginSubmitting ||
                    (!walletProviderStatus.detecting &&
                      !walletProviderStatus.available)
                  }
                  loading={
                    walletLoginSubmitting || walletProviderStatus.detecting
                  }
                >
                  {t('auth.login.wallet_action', '钱包登陆')}
                </AppButton>
              </div>
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

            <div className='router-login-divider-wrap'>
              <AppDivider className='router-login-divider' horizontal>
                或
              </AppDivider>
            </div>

            <div className='router-login-section'>
              {walletLoginEnabled && passwordLoginEnabled && (
                <AppButton
                  fluid
                  className='router-login-main-btn router-auth-button router-password-toggle'
                  onClick={() =>
                    setShowPasswordLogin((previousState) => !previousState)
                  }
                >
                  {t('auth.login.password_action', '密码登陆')}
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
