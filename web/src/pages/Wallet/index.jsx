import React, { useEffect, useState } from 'react';
import { Button, Card, Form, Header, Icon, Message, Segment } from 'semantic-ui-react';
import { showError, showSuccess } from '../../helpers';

const WalletPage = () => {
  const [address, setAddress] = useState('');
  const [chainId, setChainId] = useState('');
  const [balance, setBalance] = useState('');
  const [messageToSign, setMessageToSign] = useState('Login to Router');
  const [signResult, setSignResult] = useState('');
  const [tx, setTx] = useState({ to: '', value: '' });
  const [loading, setLoading] = useState(false);

  const hasWallet = typeof window !== 'undefined' && window.ethereum;

  useEffect(() => {
    if (hasWallet) {
      window.ethereum.on('accountsChanged', (accounts) => {
        if (accounts && accounts.length > 0) {
          setAddress(accounts[0]);
          refreshBalance(accounts[0]);
        } else {
          setAddress('');
          setBalance('');
        }
      });
      window.ethereum.on('chainChanged', (id) => {
        setChainId(parseInt(id, 16).toString());
        if (address) {
          refreshBalance(address);
        }
      });
    }
    /* eslint-disable-next-line react-hooks/exhaustive-deps */
  }, [hasWallet, address]);

  const connect = async () => {
    try {
      if (!hasWallet) {
        showError('未检测到钱包，请安装 MetaMask 或开启浏览器钱包');
        return;
      }
      const accounts = await window.ethereum.request({
        method: 'eth_requestAccounts',
      });
      const chainHex = await window.ethereum.request({ method: 'eth_chainId' });
      setChainId(parseInt(chainHex, 16).toString());
      if (accounts && accounts[0]) {
        setAddress(accounts[0]);
        refreshBalance(accounts[0]);
      }
    } catch (e) {
      showError(e.message || '连接失败');
    }
  };

  const refreshBalance = async (addr) => {
    try {
      const target = addr || address;
      if (!target) return;
      const result = await window.ethereum.request({
        method: 'eth_getBalance',
        params: [target, 'latest'],
      });
      const eth = parseInt(result, 16) / 1e18;
      setBalance(eth.toFixed(4));
    } catch (e) {
      showError('获取余额失败: ' + e.message);
    }
  };

  const signMessage = async () => {
    try {
      if (!address) {
        showError('请先连接钱包');
        return;
      }
      const signature = await window.ethereum.request({
        method: 'personal_sign',
        params: [messageToSign, address],
      });
      setSignResult(signature);
      showSuccess('签名成功');
    } catch (e) {
      if (e?.code === 4001) {
        showError('用户取消签名');
      } else {
        showError(e.message || '签名失败');
      }
    }
  };

  const sendTx = async () => {
    try {
      if (!address) {
        showError('请先连接钱包');
        return;
      }
      if (!tx.to || !tx.value) {
        showError('请输入收款地址和金额');
        return;
      }
      setLoading(true);
      const wei = '0x' + Math.floor(parseFloat(tx.value) * 1e18).toString(16);
      const hash = await window.ethereum.request({
        method: 'eth_sendTransaction',
        params: [
          {
            from: address,
            to: tx.to,
            value: wei,
          },
        ],
      });
      showSuccess('交易已提交：' + hash);
      setLoading(false);
    } catch (e) {
      setLoading(false);
      if (e?.code === 4001) {
        showError('用户取消交易');
      } else {
        showError(e.message || '发送失败');
      }
    }
  };

  return (
    <div style={{ maxWidth: 720, margin: '0 auto', padding: '32px 16px' }}>
      <Header as='h2'>钱包工具</Header>
      {!hasWallet && (
        <Message warning>
          未检测到 `window.ethereum`，请安装 MetaMask 或打开浏览器钱包后刷新。
        </Message>
      )}
      <Segment>
        <Button primary onClick={connect} disabled={!hasWallet}>
          <Icon name='plug' />
          连接钱包
        </Button>
        <div style={{ marginTop: '12px' }}>
          <div>地址：{address || '-'}</div>
          <div>链 ID：{chainId || '-'}</div>
          <div>余额：{balance ? `${balance} ETH` : '-'}</div>
        </div>
        <Button basic style={{ marginTop: '8px' }} onClick={() => refreshBalance()}>
          刷新余额
        </Button>
      </Segment>

      <Card fluid>
        <Card.Content>
          <Card.Header>签名测试</Card.Header>
          <Form>
            <Form.TextArea
              label='待签名消息'
              value={messageToSign}
              onChange={(e) => setMessageToSign(e.target.value)}
            />
            <Button color='orange' onClick={signMessage}>
              personal_sign
            </Button>
            {signResult && (
              <Message success style={{ wordBreak: 'break-all', marginTop: '8px' }}>
                {signResult}
              </Message>
            )}
          </Form>
        </Card.Content>
      </Card>

      <Card fluid>
        <Card.Content>
          <Card.Header>发送 ETH</Card.Header>
          <Form>
            <Form.Input
              label='收款地址'
              placeholder='0x...'
              value={tx.to}
              onChange={(e) => setTx({ ...tx, to: e.target.value })}
            />
            <Form.Input
              label='金额（ETH）'
              type='number'
              placeholder='0.01'
              value={tx.value}
              onChange={(e) => setTx({ ...tx, value: e.target.value })}
            />
            <Button color='green' loading={loading} onClick={sendTx}>
              发送
            </Button>
          </Form>
        </Card.Content>
      </Card>
    </div>
  );
};

export default WalletPage;
