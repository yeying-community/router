import React, { useEffect, useState } from 'react';
import {
  Button,
  Form,
  Grid,
  Header,
  Card,
  Statistic,
  Label,
  Table,
} from 'semantic-ui-react';
import {
  API,
  showError,
  showInfo,
  showSuccess,
  timestamp2string,
} from '../../helpers';
import { renderYYC } from '../../helpers/render';
import { useTranslation } from 'react-i18next';

const normalizeTopUpResult = (raw) => {
  if (!raw || typeof raw !== 'object') {
    return null;
  }
  const redeemedYYC = Number(raw?.redeemed_yyc ?? 0) || 0;
  const beforeYYCBalance = Number(raw?.before_yyc_balance ?? 0) || 0;
  const afterYYCBalance = Number(raw?.after_yyc_balance ?? 0) || 0;
  return {
    redeemed_yyc: redeemedYYC,
    before_yyc_balance: beforeYYCBalance,
    after_yyc_balance: afterYYCBalance,
    redemption_id: raw?.redemption_id || '',
    redemption_name: raw?.redemption_name || '',
    redeemed_at: Number(raw?.redeemed_at ?? 0) || 0,
  };
};

const TopUp = () => {
  const { t } = useTranslation();
  const [redemptionCode, setRedemptionCode] = useState('');
  const [topUpLink, setTopUpLink] = useState('');
  const [userQuota, setUserQuota] = useState(0);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isCreatingTopUpOrder, setIsCreatingTopUpOrder] = useState(false);
  const [topupOrders, setTopupOrders] = useState([]);
  const [loadingOrders, setLoadingOrders] = useState(false);
  const [topupLogs, setTopupLogs] = useState([]);
  const [loadingLogs, setLoadingLogs] = useState(false);
  const [recentTopUpResult, setRecentTopUpResult] = useState(null);

  const topUp = async () => {
    if (redemptionCode === '') {
      showInfo(t('topup.redeem_code.empty_code'));
      return;
    }
    setIsSubmitting(true);
    try {
      const res = await API.post('/api/v1/public/user/topup', {
        code: redemptionCode,
      });
      const { success, message, data } = res.data;
      if (success) {
        const normalizedResult =
          normalizeTopUpResult(data) || {
            redeemed_yyc: Number(data ?? 0) || 0,
            before_yyc_balance: userQuota,
            after_yyc_balance: userQuota + (Number(data ?? 0) || 0),
            redemption_id: '',
            redemption_name: '',
            redeemed_at: 0,
          };
        showSuccess(t('topup.redeem_code.success'));
        setUserQuota(normalizedResult.after_yyc_balance);
        setRecentTopUpResult(normalizedResult);
        setRedemptionCode('');
        getTopupLogs().then();
      } else {
        showError(message);
      }
    } catch (err) {
      showError(t('topup.redeem_code.request_failed'));
    } finally {
      setIsSubmitting(false);
    }
  };

  const openTopUpLink = async () => {
    if (!topUpLink) {
      showError(t('topup.redeem_code.no_link'));
      return;
    }
    const popup = window.open('', '_blank', 'noopener,noreferrer');
    if (!popup) {
      showError(t('topup.get_code.popup_blocked'));
      return;
    }
    setIsCreatingTopUpOrder(true);
    try {
      const res = await API.post('/api/v1/public/user/topup/orders');
      const { success, message, data } = res.data;
      if (!success) {
        popup.close();
        showError(message);
        return;
      }
      const redirectURL = data?.redirect_url;
      if (!redirectURL) {
        popup.close();
        showError(t('topup.get_code.request_failed'));
        return;
      }
      getTopupOrders().then();
      popup.location.href = redirectURL;
      popup.focus();
    } catch (err) {
      popup.close();
      showError(t('topup.get_code.request_failed'));
    } finally {
      setIsCreatingTopUpOrder(false);
    }
  };

  const getUserQuota = async () => {
    let res = await API.get(`/api/v1/public/user/self`);
    const { success, message, data } = res.data;
    if (success) {
      setUserQuota(Number(data?.yyc_balance ?? data?.quota ?? 0) || 0);
    } else {
      showError(message);
    }
  };

  const getTopupOrders = async () => {
    setLoadingOrders(true);
    try {
      const res = await API.get('/api/v1/public/user/topup/orders?page=1&page_size=10');
      const { success, message, data } = res.data;
      if (success) {
        setTopupOrders(Array.isArray(data?.items) ? data.items : []);
      } else {
        showError(message);
      }
    } finally {
      setLoadingOrders(false);
    }
  };

  const getTopupLogs = async () => {
    setLoadingLogs(true);
    try {
      const res = await API.get('/api/v1/public/log?page=1&type=1');
      const { success, message, data } = res.data;
      if (success) {
        setTopupLogs(Array.isArray(data) ? data : []);
      } else {
        showError(message);
      }
    } finally {
      setLoadingLogs(false);
    }
  };

  useEffect(() => {
    let status = localStorage.getItem('status');
    if (status) {
      status = JSON.parse(status);
      if (status.top_up_link) {
        setTopUpLink(status.top_up_link);
      }
    }
    getUserQuota().then();
    getTopupOrders().then();
    getTopupLogs().then();
  }, []);

  const renderTopupOrderStatus = (status) => {
    switch (status) {
      case 'created':
        return (
          <Label basic color='blue' className='router-tag'>
            {t('topup.orders.status.created')}
          </Label>
        );
      case 'pending':
        return (
          <Label basic color='orange' className='router-tag'>
            {t('topup.orders.status.pending')}
          </Label>
        );
      case 'paid':
        return (
          <Label basic color='teal' className='router-tag'>
            {t('topup.orders.status.paid')}
          </Label>
        );
      case 'fulfilled':
        return (
          <Label basic color='green' className='router-tag'>
            {t('topup.orders.status.fulfilled')}
          </Label>
        );
      case 'failed':
        return (
          <Label basic color='red' className='router-tag'>
            {t('topup.orders.status.failed')}
          </Label>
        );
      case 'canceled':
        return (
          <Label basic className='router-tag'>
            {t('topup.orders.status.canceled')}
          </Label>
        );
      default:
        return (
          <Label basic className='router-tag'>
            {status || '-'}
          </Label>
        );
    }
  };

  return (
    <div className='dashboard-container'>
      <Card fluid className='chart-card'>
        <Card.Content>
          <Card.Header>
            <Header as='h2' className='router-page-title'>{t('topup.title')}</Header>
          </Card.Header>

          <Grid columns={2} stackable>
            <Grid.Column>
              <Card
                fluid
                className='router-soft-card router-soft-card-fill'
              >
                <Card.Content className='router-card-fill'>
                  <Card.Header className='router-card-header'>
                    <Header as='h3' className='router-section-title router-title-accent-primary'>
                      <i className='credit card icon'></i>
                      {t('topup.get_code.title')}
                    </Header>
                  </Card.Header>
                  <Card.Description className='router-card-fill'>
                    <div className='router-card-body-spread'>
                      <div className='router-center-panel'>
                        <Statistic className='router-accent-statistic'>
                          <Statistic.Value>
                            {renderYYC(userQuota, t)}
                          </Statistic.Value>
                          <Statistic.Label>
                            {t('topup.get_code.current_quota')}
                          </Statistic.Label>
                        </Statistic>
                        <div className='router-text-muted' style={{ marginTop: '0.75rem' }}>
                          {t('topup.get_code.description')}
                        </div>
                      </div>

                      <div className='router-action-footer'>
                        <Button
                          className='router-section-button router-action-button-wide'
                          primary
                          onClick={openTopUpLink}
                          loading={isCreatingTopUpOrder}
                          disabled={isCreatingTopUpOrder || !topUpLink}
                        >
                          {isCreatingTopUpOrder
                            ? t('topup.get_code.creating')
                            : t('topup.get_code.button')}
                        </Button>
                      </div>
                    </div>
                  </Card.Description>
                </Card.Content>
              </Card>
            </Grid.Column>

            <Grid.Column>
              <Card
                fluid
                className='router-soft-card router-soft-card-fill'
              >
                <Card.Content className='router-card-fill'>
                  <Card.Header className='router-card-header'>
                    <Header as='h3' className='router-section-title router-title-accent-positive'>
                      <i className='ticket alternate icon'></i>
                      {t('topup.redeem_code.title')}
                    </Header>
                  </Card.Header>
                  <Card.Description className='router-card-fill'>
                    <div className='router-card-body-spread'>
                      <div className='router-text-muted'>
                        {t('topup.redeem_code.description')}
                      </div>

                      <Form.Input
                        className='router-section-input'
                        fluid
                        icon='key'
                        iconPosition='left'
                        placeholder={t('topup.redeem_code.placeholder')}
                        value={redemptionCode}
                        onChange={(e) => {
                          setRedemptionCode(e.target.value);
                        }}
                        onPaste={(e) => {
                          e.preventDefault();
                          const pastedText = e.clipboardData.getData('text');
                          setRedemptionCode(pastedText.trim());
                        }}
                        action={
                          <Button
                            className='router-section-button'
                            onClick={async () => {
                              try {
                                const text =
                                  await navigator.clipboard.readText();
                                setRedemptionCode(text.trim());
                              } catch (err) {
                                showError(t('topup.redeem_code.paste_error'));
                              }
                            }}
                          >
                            {t('topup.redeem_code.paste')}
                          </Button>
                        }
                      />

                      <div className='router-action-footer'>
                        <Button
                          className='router-section-button'
                          color='green'
                          fluid
                          onClick={topUp}
                          loading={isSubmitting}
                          disabled={isSubmitting}
                        >
                          {isSubmitting
                            ? t('topup.redeem_code.submitting')
                            : t('topup.redeem_code.submit')}
                        </Button>
                      </div>
                    </div>
                  </Card.Description>
                </Card.Content>
              </Card>
            </Grid.Column>
          </Grid>

          {recentTopUpResult ? (
            <Card fluid className='router-soft-card' style={{ marginTop: '1rem' }}>
              <Card.Content>
                <Card.Header className='router-card-header'>
                  <div className='router-toolbar'>
                    <Header as='h3' className='router-section-title router-title-accent-warning'>
                      <i className='check circle icon'></i>
                      {t('topup.result.title')}
                    </Header>
                    <Button
                      className='router-section-button'
                      basic
                      size='small'
                      onClick={() => setRecentTopUpResult(null)}
                    >
                      {t('topup.result.close')}
                    </Button>
                  </div>
                </Card.Header>
                <Table basic='very' compact='very' className='router-list-table'>
                  <Table.Body>
                    <Table.Row>
                      <Table.Cell width={4}>{t('topup.result.fields.redeemed_yyc')}</Table.Cell>
                      <Table.Cell>{renderYYC(recentTopUpResult.redeemed_yyc, t)}</Table.Cell>
                      <Table.Cell width={4}>{t('topup.result.fields.redeemed_at')}</Table.Cell>
                      <Table.Cell>
                        {recentTopUpResult.redeemed_at
                          ? timestamp2string(recentTopUpResult.redeemed_at)
                          : '-'}
                      </Table.Cell>
                    </Table.Row>
                    <Table.Row>
                      <Table.Cell>{t('topup.result.fields.before_balance')}</Table.Cell>
                      <Table.Cell>{renderYYC(recentTopUpResult.before_yyc_balance, t)}</Table.Cell>
                      <Table.Cell>{t('topup.result.fields.after_balance')}</Table.Cell>
                      <Table.Cell>{renderYYC(recentTopUpResult.after_yyc_balance, t)}</Table.Cell>
                    </Table.Row>
                    <Table.Row>
                      <Table.Cell>{t('topup.result.fields.redemption_name')}</Table.Cell>
                      <Table.Cell>{recentTopUpResult.redemption_name || '-'}</Table.Cell>
                      <Table.Cell>{t('topup.result.fields.redemption_id')}</Table.Cell>
                      <Table.Cell>{recentTopUpResult.redemption_id || '-'}</Table.Cell>
                    </Table.Row>
                  </Table.Body>
                </Table>
              </Card.Content>
            </Card>
          ) : null}

          <Card fluid className='router-soft-card' style={{ marginTop: '1rem' }}>
            <Card.Content>
              <Card.Header className='router-card-header'>
                <div className='router-toolbar'>
                  <Header as='h3' className='router-section-title router-title-accent-primary'>
                    <i className='credit card outline icon'></i>
                    {t('topup.orders.title')}
                  </Header>
                  <Button
                    className='router-section-button'
                    onClick={getTopupOrders}
                    loading={loadingOrders}
                  >
                    {t('topup.orders.refresh')}
                  </Button>
                </div>
              </Card.Header>
              <Table basic='very' compact className='router-list-table'>
                <Table.Header>
                  <Table.Row>
                    <Table.HeaderCell width={3}>
                      {t('topup.orders.columns.time')}
                    </Table.HeaderCell>
                    <Table.HeaderCell width={2}>
                      {t('topup.orders.columns.status')}
                    </Table.HeaderCell>
                    <Table.HeaderCell width={4}>
                      {t('topup.orders.columns.order_id')}
                    </Table.HeaderCell>
                    <Table.HeaderCell>
                      {t('topup.orders.columns.transaction_id')}
                    </Table.HeaderCell>
                  </Table.Row>
                </Table.Header>
                <Table.Body>
                  {topupOrders.length === 0 ? (
                    <Table.Row>
                      <Table.Cell colSpan='4' className='router-text-muted'>
                        {loadingOrders
                          ? t('common.loading')
                          : t('topup.orders.empty')}
                      </Table.Cell>
                    </Table.Row>
                  ) : (
                    topupOrders.map((order) => (
                      <Table.Row key={order.id}>
                        <Table.Cell>
                          {order.created_at ? timestamp2string(order.created_at) : '-'}
                        </Table.Cell>
                        <Table.Cell>{renderTopupOrderStatus(order.status)}</Table.Cell>
                        <Table.Cell style={{ wordBreak: 'break-all' }}>
                          {order.id || '-'}
                        </Table.Cell>
                        <Table.Cell style={{ wordBreak: 'break-all' }}>
                          {order.transaction_id || '-'}
                        </Table.Cell>
                      </Table.Row>
                    ))
                  )}
                </Table.Body>
              </Table>
            </Card.Content>
          </Card>

          <Card fluid className='router-soft-card' style={{ marginTop: '1rem' }}>
            <Card.Content>
              <Card.Header className='router-card-header'>
                <div className='router-toolbar'>
                  <Header as='h3' className='router-section-title router-title-accent-secondary'>
                    <i className='history icon'></i>
                    {t('topup.redemptions.title')}
                  </Header>
                  <Button
                    className='router-section-button'
                    onClick={getTopupLogs}
                    loading={loadingLogs}
                  >
                    {t('topup.history.refresh')}
                  </Button>
                </div>
              </Card.Header>
              <Table basic='very' compact className='router-list-table'>
                <Table.Header>
                  <Table.Row>
                    <Table.HeaderCell width={3}>
                      {t('topup.redemptions.columns.time')}
                    </Table.HeaderCell>
                    <Table.HeaderCell width={2}>
                      {t('topup.redemptions.columns.quota')}
                    </Table.HeaderCell>
                    <Table.HeaderCell>
                      {t('topup.redemptions.columns.detail')}
                    </Table.HeaderCell>
                  </Table.Row>
                </Table.Header>
                <Table.Body>
                  {topupLogs.length === 0 ? (
                    <Table.Row>
                      <Table.Cell colSpan='3' className='router-text-muted'>
                        {loadingLogs
                          ? t('common.loading')
                          : t('topup.redemptions.empty')}
                      </Table.Cell>
                    </Table.Row>
                  ) : (
                    topupLogs.map((log) => (
                      <Table.Row key={log.trace_id || `${log.created_at}-${log.content}`}>
                        <Table.Cell>
                          {log.created_at ? timestamp2string(log.created_at) : '-'}
                        </Table.Cell>
                        <Table.Cell>
                          {log.quota ? (
                            <Label basic color='green' className='router-tag'>
                              {renderYYC(log.quota, t)}
                            </Label>
                          ) : (
                            '-'
                          )}
                        </Table.Cell>
                        <Table.Cell>{log.content || '-'}</Table.Cell>
                      </Table.Row>
                    ))
                  )}
                </Table.Body>
              </Table>
            </Card.Content>
          </Card>
        </Card.Content>
      </Card>
    </div>
  );
};

export default TopUp;
