import React, { useContext, useEffect, useState } from 'react';
import { Button, Divider, Form, Grid, Header, Image, Message, Card } from 'semantic-ui-react';
import { Link, useNavigate, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { UserContext } from '../context/User';
import { API, getLogo, showError, showSuccess, showWarning } from '../helpers';

const LoginForm = () => {
  const { t } = useTranslation();
  const [inputs, setInputs] = useState({
    username: '',
    password: '',
  });
  const [searchParams] = useSearchParams();
  const [submitted, setSubmitted] = useState(false);
  const { username, password } = inputs;
  const [userState, userDispatch] = useContext(UserContext);
  let navigate = useNavigate();
  const [status, setStatus] = useState({});
  const logo = getLogo();

  useEffect(() => {
    if (searchParams.get('expired')) {
      showError(t('messages.error.login_expired'));
    }
    let status = localStorage.getItem('status');
    if (status) {
      status = JSON.parse(status);
      setStatus(status);
    }
  }, []);

  // 微信登录已下线

  const onWalletLoginClicked = async () => {
    try {
      if (!status.wallet_login) {
        showError(t('auth.login.wallet_disabled') || '钱包登录未开启');
        return;
      }
      if (!window.ethereum || !window.ethereum.request) {
        showError('未检测到钱包，请安装 MetaMask 或开启浏览器钱包');
        return;
      }
      const accounts = await window.ethereum.request({
        method: 'eth_requestAccounts',
      });
      if (!accounts || accounts.length === 0) {
        showError('未获取到钱包账户');
        return;
      }
      const address = accounts[0];
      const chainHex = await window.ethereum.request({
        method: 'eth_chainId',
      });
      const chain_id = parseInt(chainHex, 16).toString();
      const nonceResp = await API.post(
        '/api/v1/public/common/auth/challenge',
        {
          address,
          chain_id,
        }
      );
      const noncePayload =
        nonceResp?.data?.data || nonceResp?.data?.body || nonceResp?.data;
      if (nonceResp?.data?.success === false) {
        showError(nonceResp.data?.message || '获取挑战失败');
        return;
      }
      const nonceData = {
        nonce: noncePayload?.nonce,
        message: noncePayload?.message || noncePayload?.result,
      };
      if (!nonceData.nonce || !nonceData.message) {
        showError('服务器返回的挑战数据异常');
        return;
      }
      const signature = await window.ethereum.request({
        method: 'personal_sign',
        params: [nonceData.message, address],
      });
      const res = await API.post('/api/v1/public/common/auth/verify', {
        address,
        signature,
        nonce: nonceData.nonce,
        chain_id,
      });
      if (res?.data?.success === false) {
        showError(res.data?.message || '钱包登录失败');
        return;
      }
      const body = res?.data?.body || res?.data?.data || res?.data;
      const token = body?.token || res?.data?.token;
      const userData = body?.user || body?.data || res?.data?.data;
      if (!userData) {
        showError('未获取到用户信息');
        return;
      }
      if (token) {
        userData.token = token;
        localStorage.setItem('wallet_token', token);
      }
      if (body?.expires_at) {
        localStorage.setItem('wallet_token_expires_at', body.expires_at);
      }
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
    setSubmitted(true);
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
                  background: '#2F73FF', // 使用更现代的蓝色
                  color: 'white',
                  marginBottom: '1.5em',
                }}
                onClick={handleSubmit}
              >
                {t('auth.login.button')}
              </Button>
            </Form>

            <Divider />
            <Message style={{ background: 'transparent', boxShadow: 'none' }}>
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

            {status.wallet_login && (
              <>
                <Divider
                  horizontal
                  style={{ color: '#666', fontSize: '0.9em' }}
                >
                  {t('auth.login.wallet_title', '钱包登录')}
                </Divider>
                <Button
                  fluid
                  size='large'
                  color='orange'
                  onClick={onWalletLoginClicked}
                  style={{ marginTop: '0.5em' }}
                >
                  {t('auth.login.wallet_button', '使用钱包登录')}
                </Button>
              </>
            )}
          </Card.Content>
        </Card>
      </Grid.Column>
    </Grid>
  );
};

export default LoginForm;
