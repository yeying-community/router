import React, { useContext, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Divider, Form, Header, Message, Segment } from 'semantic-ui-react';
import { Link, useNavigate } from 'react-router-dom';
import { API, copy, showError, showSuccess } from '../helpers';
import { UserContext } from '../context/User';
import { WEB3_TOKEN_STORAGE_KEY } from '../helpers/web3';
import {
  getWalletContext,
  signWalletMessage,
  logoutWallet,
} from '../services/web3Auth';

const PersonalSetting = () => {
  const { t } = useTranslation();
  const [userState, userDispatch] = useContext(UserContext);
  const navigate = useNavigate();

  const [status, setStatus] = useState({});
  const [systemToken, setSystemToken] = useState('');
  const [affLink, setAffLink] = useState('');
  const [walletBinding, setWalletBinding] = useState(userState?.user?.wallet_address || '');
  const [inputs, setInputs] = useState({
    self_account_deletion_confirmation: '',
  });
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    const cached = localStorage.getItem('status');
    if (cached) {
      setStatus(JSON.parse(cached));
    }
    if (userState?.user?.wallet_address) {
      setWalletBinding(userState.user.wallet_address);
    }
  }, [userState?.user?.wallet_address]);

  const handleInputChange = (e, { name, value }) => {
    setInputs((prev) => ({ ...prev, [name]: value }));
  };

  const bindWallet = async () => {
    try {
      if (!status.wallet_login) {
        showError('管理员未开启钱包登录');
        return;
      }
      const { provider, address, chainId } = await getWalletContext();
      const nonceResp = await API.post('/api/v1/public/common/auth/challenge', {
        address,
        chain_id: chainId,
      });
      const payload = nonceResp?.data?.data || nonceResp?.data?.body || nonceResp?.data;
      if (nonceResp?.data?.success === false) {
        showError(nonceResp.data?.message || '获取挑战失败');
        return;
      }
      const nonceData = { nonce: payload?.nonce, message: payload?.message || payload?.result };
      if (!nonceData.nonce || !nonceData.message) {
        showError('服务器返回的挑战数据异常');
        return;
      }
      const { signature } = await signWalletMessage(
        nonceData.message,
        address,
        provider
      );
      const res = await API.post('/api/oauth/wallet/bind', {
        address,
        signature,
        nonce: nonceData.nonce,
        chain_id: chainId,
      });
      const { success, message } = res.data;
      if (success) {
        showSuccess('钱包绑定成功');
        setWalletBinding(address);
      } else {
        showError(message);
      }
    } catch (err) {
      if (err?.code === 4001) {
        showError('用户取消了签名');
      } else {
        showError(err.message || '绑定失败');
      }
    }
  };

  const generateAccessToken = async () => {
    const res = await API.get('/api/user/token');
    const { success, message, data } = res.data;
    if (success) {
      setSystemToken(data);
      setAffLink('');
      await copy(data);
      showSuccess('令牌已重置并已复制到剪贴板');
    } else {
      showError(message);
    }
  };

  const getAffLink = async () => {
    const res = await API.get('/api/user/aff');
    const { success, message, data } = res.data;
    if (success) {
      const link = `${window.location.origin}/register?aff=${data}`;
      setAffLink(link);
      setSystemToken('');
      await copy(link);
      showSuccess('邀请链接已复制到剪切板');
    } else {
      showError(message);
    }
  };

  const deleteAccount = async () => {
    if (inputs.self_account_deletion_confirmation !== userState.user.username) {
      showError('请输入你的账户名以确认删除！');
      return;
    }
    setLoading(true);
    const res = await API.delete('/api/user/self');
    const { success, message } = res.data;
    setLoading(false);
    if (success) {
      showSuccess('账户已删除！');
      await API.get('/api/user/logout');
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
    } else {
      showError(message);
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
      <Segment>
        <Header as='h3'>{t('setting.personal.binding.title', '钱包绑定')}</Header>
        <p style={{ color: '#666' }}>
          {walletBinding ? `当前绑定地址：${walletBinding}` : '尚未绑定钱包'}
        </p>
        <Button primary onClick={bindWallet} disabled={!status.wallet_login}>
          {status.wallet_login ? '绑定 / 更换钱包' : '管理员未开启钱包登录'}
        </Button>
        {!status.wallet_login && (
          <Message warning style={{ marginTop: '12px' }}>
            管理员未开启钱包登录，仍可使用账号密码登录。
          </Message>
        )}
      </Segment>

      <Segment>
        <Header as='h3'>{t('setting.personal.tokens.title', '令牌与邀请')}</Header>
        <Button color='blue' onClick={generateAccessToken} style={{ marginBottom: '8px' }}>
          生成并复制系统令牌
        </Button>
        {systemToken && (
          <Message success size='small' style={{ wordBreak: 'break-all' }}>
            {systemToken}
          </Message>
        )}
        <Divider />
        <Button onClick={getAffLink}>生成并复制邀请链接</Button>
        {affLink && (
          <Message success size='small' style={{ wordBreak: 'break-all' }}>
            {affLink}
          </Message>
        )}
      </Segment>

      <Segment>
        <Header as='h3' color='red'>
          {t('setting.personal.delete.title', '删除账户')}
        </Header>
        <p style={{ color: '#666' }}>删除账户将同时清除钱包绑定与令牌。</p>
        <Form>
          <Form.Input
            label='请输入账户名以确认删除'
            placeholder={userState.user?.username}
            name='self_account_deletion_confirmation'
            value={inputs.self_account_deletion_confirmation}
            onChange={handleInputChange}
          />
          <Button negative loading={loading} onClick={deleteAccount}>
            {t('setting.personal.delete.confirm', '确认删除')}
          </Button>
        </Form>
      </Segment>

      <Message info>
        <Link to='/reset'>{t('auth.login.reset_password', '找回密码')}</Link>
        <span style={{ marginLeft: 8 }}>仍可通过账号密码登录，无需第三方账号。</span>
      </Message>
    </div>
  );
};

export default PersonalSetting;
