import React, { useCallback, useEffect, useMemo, useState } from 'react';
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
import { formatAmountWithUnit, renderYYC } from '../../helpers/render';
import {
  buildDisplayUnitOptions,
  buildPublicDisplayCurrencyIndex,
  convertYYCToDisplayAmount,
  DEFAULT_FIAT_DISPLAY_CODE,
  loadPublicDisplayCurrencyCatalog,
  normalizeDisplayCurrencyCode,
  resolvePreferredDisplayCurrency,
  YYC_DISPLAY_CODE,
} from '../../helpers/billing';
import { useTranslation } from 'react-i18next';
import UnitDropdown from '../../components/UnitDropdown';

const TOPUP_DISPLAY_CURRENCY_STORAGE_KEY = 'topup_display_currency';

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
    group_id: raw?.group_id || '',
    group_name: raw?.group_name || '',
    face_value_amount: Number(raw?.face_value_amount ?? 0) || 0,
    face_value_unit: raw?.face_value_unit || '',
    redeemed_at: Number(raw?.redeemed_at ?? 0) || 0,
  };
};

const normalizeRedemptionRecord = (raw) => {
  if (!raw || typeof raw !== 'object') {
    return null;
  }
  return {
    ...raw,
    // Keep legacy quota fallback for older redemption history records.
    yycAmount: Number(raw?.yyc_amount ?? raw?.quota ?? 0) || 0,
  };
};

const getStoredDisplayCurrency = () => {
  if (typeof window === 'undefined') {
    return '';
  }
  return normalizeDisplayCurrencyCode(
    window.localStorage.getItem(TOPUP_DISPLAY_CURRENCY_STORAGE_KEY)
  );
};

const storeDisplayCurrency = (code) => {
  if (typeof window === 'undefined') {
    return;
  }
  window.localStorage.setItem(
    TOPUP_DISPLAY_CURRENCY_STORAGE_KEY,
    normalizeDisplayCurrencyCode(code)
  );
};

const resolveDisplayCurrency = (currencyIndex, current = '') => {
  return resolvePreferredDisplayCurrency(
    currencyIndex,
    current || getStoredDisplayCurrency() || DEFAULT_FIAT_DISPLAY_CODE
  );
};

const getStoredStatusConfig = () => {
  if (typeof window === 'undefined') {
    return {};
  }
  try {
    const raw = window.localStorage.getItem('status');
    if (!raw) {
      return {};
    }
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch (error) {
    return {};
  }
};

const TopUp = () => {
  const { t } = useTranslation();
  const initialCurrencyIndex = buildPublicDisplayCurrencyIndex([]);
  const [redemptionCode, setRedemptionCode] = useState('');
  const [externalTopupLink, setExternalTopupLink] = useState('');
  const [userBalanceYYC, setUserBalanceYYC] = useState(0);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [isCreatingTopUpOrder, setIsCreatingTopUpOrder] = useState(false);
  const [topupAmount, setTopupAmount] = useState('0');
  const [packages, setPackages] = useState([]);
  const [selectedPackageId, setSelectedPackageId] = useState('');
  const [externalTopupOrders, setExternalTopupOrders] = useState([]);
  const [loadingExternalTopupOrders, setLoadingExternalTopupOrders] = useState(false);
  const [redemptionRecords, setRedemptionRecords] = useState([]);
  const [loadingRedemptionRecords, setLoadingRedemptionRecords] = useState(false);
  const [recentRedemptionResult, setRecentRedemptionResult] = useState(null);
  const [displayCurrencyIndex, setDisplayCurrencyIndex] = useState(
    initialCurrencyIndex
  );
  const [displayCurrency, setDisplayCurrency] = useState(
    resolveDisplayCurrency(initialCurrencyIndex)
  );
  const [loadingDisplayCurrencies, setLoadingDisplayCurrencies] = useState(false);

  const displayCurrencyOptions = useMemo(
    () => buildDisplayUnitOptions(displayCurrencyIndex, { includeCode: true }),
    [displayCurrencyIndex]
  );

  const renderDisplayAmount = (yycAmount) => {
    const normalizedAmount = Number(yycAmount || 0);
    if (!Number.isFinite(normalizedAmount)) {
      return renderYYC(0, t);
    }
    const normalizedCurrency = normalizeDisplayCurrencyCode(displayCurrency);
    if (normalizedCurrency === YYC_DISPLAY_CODE) {
      return renderYYC(normalizedAmount, t);
    }
    const displayAmount = convertYYCToDisplayAmount(
      normalizedAmount,
      normalizedCurrency,
      displayCurrencyIndex
    );
    if (!Number.isFinite(displayAmount)) {
      return renderYYC(normalizedAmount, t);
    }
    return formatAmountWithUnit(displayAmount, normalizedCurrency, 6);
  };

  const loadDisplayCurrencies = useCallback(async () => {
    setLoadingDisplayCurrencies(true);
    try {
      const { currencyIndex: nextIndex, defaultCurrency } =
        await loadPublicDisplayCurrencyCatalog();
      setDisplayCurrencyIndex(nextIndex);
      setDisplayCurrency((prev) => {
        const next = resolveDisplayCurrency(
          nextIndex,
          prev || defaultCurrency || DEFAULT_FIAT_DISPLAY_CODE
        );
        storeDisplayCurrency(next);
        return next;
      });
    } finally {
      setLoadingDisplayCurrencies(false);
    }
  }, []);

  const submitRedemption = async () => {
    if (redemptionCode === '') {
      showInfo(t('topup.redeem.empty_code'));
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
            before_yyc_balance: userBalanceYYC,
            after_yyc_balance: userBalanceYYC + (Number(data ?? 0) || 0),
            redemption_id: '',
            redemption_name: '',
            group_id: '',
            group_name: '',
            face_value_amount: 0,
            face_value_unit: '',
            redeemed_at: 0,
          };
        showSuccess(t('topup.redeem.success'));
        setUserBalanceYYC(normalizedResult.after_yyc_balance);
        setRecentRedemptionResult(normalizedResult);
        setRedemptionCode('');
        loadRedemptionRecords().then();
      } else {
        showError(message);
      }
    } catch (err) {
      showError(t('topup.redeem.request_failed'));
    } finally {
      setIsSubmitting(false);
    }
  };

  const openExternalOrder = async (payload) => {
    if (!externalTopupLink) {
      showError(t('topup.external_topup.no_link'));
      return;
    }
    const popup = window.open('', '_blank', 'noopener,noreferrer');
    if (!popup) {
      showError(t('topup.external_topup.popup_blocked'));
      return;
    }
    setIsCreatingTopUpOrder(true);
    try {
      const res = await API.post('/api/v1/public/user/topup/orders', payload);
      const { success, message, data } = res.data;
      if (!success) {
        popup.close();
        showError(message);
        return;
      }
      const redirectURL = data?.redirect_url;
      if (!redirectURL) {
        popup.close();
        showError(t('topup.external_topup.request_failed'));
        return;
      }
      loadExternalTopupOrders().then();
      popup.location.href = redirectURL;
      popup.focus();
    } catch (err) {
      popup.close();
      showError(t('topup.external_topup.request_failed'));
    } finally {
      setIsCreatingTopUpOrder(false);
    }
  };

  const openExternalTopup = async () => {
    const amount = Number(topupAmount || 0);
    if (!Number.isFinite(amount) || amount <= 0) {
      showInfo(t('topup.external_topup.amount_invalid'));
      return;
    }
    await openExternalOrder({
      business_type: 'balance_topup',
      amount,
      currency: 'CNY',
      return_url: window.location.href,
    });
  };

  const openPackagePurchase = async () => {
    const packageID = (selectedPackageId || '').trim();
    if (!packageID) {
      showInfo(t('topup.external_topup.package_select_required'));
      return;
    }
    await openExternalOrder({
      business_type: 'package_purchase',
      package_id: packageID,
      return_url: window.location.href,
    });
  };

  const loadUserBalance = async () => {
    const res = await API.get(`/api/v1/public/user/self`);
    const { success, message, data } = res?.data || {};
    if (success) {
      setUserBalanceYYC(Number(data?.yyc_balance ?? data?.quota ?? 0) || 0);
    } else {
      showError(message || t('topup.external_topup.request_failed'));
    }
  };

  const loadExternalTopupOrders = async () => {
    setLoadingExternalTopupOrders(true);
    try {
      const res = await API.get('/api/v1/public/user/topup/orders?page=1&page_size=10');
      const { success, message, data } = res?.data || {};
      if (success) {
        setExternalTopupOrders(Array.isArray(data?.items) ? data.items : []);
      } else {
        showError(message || t('topup.external_topup.request_failed'));
      }
    } catch (err) {
      showError(err?.message || t('topup.external_topup.request_failed'));
    } finally {
      setLoadingExternalTopupOrders(false);
    }
  };

  const loadPackages = async () => {
    try {
      const res = await API.get('/api/v1/public/user/packages');
      const { success, message, data } = res?.data || {};
      if (!success) {
        showError(message || t('topup.external_topup.request_failed'));
        return;
      }
      const rows = Array.isArray(data) ? data : [];
      setPackages(rows);
      setSelectedPackageId((current) => {
        if (current && rows.some((item) => item?.id === current)) {
          return current;
        }
        return rows[0]?.id || '';
      });
    } catch (err) {
      showError(err?.message || t('topup.external_topup.request_failed'));
    }
  };

  const loadRedemptionRecords = async () => {
    setLoadingRedemptionRecords(true);
    try {
      const res = await API.get('/api/v1/public/log?page=1&type=1');
      const { success, message, data } = res?.data || {};
      if (success) {
        setRedemptionRecords(
          Array.isArray(data)
            ? data.map(normalizeRedemptionRecord).filter(Boolean)
            : []
        );
      } else {
        showError(message || t('topup.redeem.request_failed'));
      }
    } catch (err) {
      showError(err?.message || t('topup.redeem.request_failed'));
    } finally {
      setLoadingRedemptionRecords(false);
    }
  };

  useEffect(() => {
    const status = getStoredStatusConfig();
    if (status.top_up_link) {
      setExternalTopupLink(status.top_up_link);
    }
    loadUserBalance().then();
    loadExternalTopupOrders().then();
    loadRedemptionRecords().then();
    loadDisplayCurrencies().then();
    loadPackages().then();
  }, [loadDisplayCurrencies]);

  const estimatedTopupYYC = useMemo(() => {
    const amount = Number(topupAmount || 0);
    if (!Number.isFinite(amount) || amount <= 0) {
      return 0;
    }
    const cnyCurrency = displayCurrencyIndex?.CNY;
    const yycPerUnit = Number(cnyCurrency?.yyc_per_unit || 0);
    if (!Number.isFinite(yycPerUnit) || yycPerUnit <= 0) {
      return 0;
    }
    return Math.round(amount * yycPerUnit);
  }, [displayCurrencyIndex, topupAmount]);

  const renderExternalTopupOrderStatus = (status) => {
    switch (status) {
      case 'created':
        return (
          <Label basic color='blue' className='router-tag'>
            {t('topup.external_topup_orders.status.created')}
          </Label>
        );
      case 'pending':
        return (
          <Label basic color='orange' className='router-tag'>
            {t('topup.external_topup_orders.status.pending')}
          </Label>
        );
      case 'paid':
        return (
          <Label basic color='teal' className='router-tag'>
            {t('topup.external_topup_orders.status.paid')}
          </Label>
        );
      case 'fulfilled':
        return (
          <Label basic color='green' className='router-tag'>
            {t('topup.external_topup_orders.status.fulfilled')}
          </Label>
        );
      case 'failed':
        return (
          <Label basic color='red' className='router-tag'>
            {t('topup.external_topup_orders.status.failed')}
          </Label>
        );
      case 'canceled':
        return (
          <Label basic className='router-tag'>
            {t('topup.external_topup_orders.status.canceled')}
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
          <Card.Header className='router-card-header'>
            <div className='router-toolbar'>
              <Header as='h2' className='router-page-title'>
                {t('topup.title')}
              </Header>
              <div
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '0.5rem',
                  flexWrap: 'nowrap',
                }}
              >
                <span
                  className='router-text-muted'
                  style={{ whiteSpace: 'nowrap', fontSize: '0.92rem' }}
                >
                  {t('topup.display_currency')}
                </span>
                <UnitDropdown
                  variant='inline'
                  compact
                  style={{ minWidth: '108px' }}
                  options={displayCurrencyOptions}
                  value={displayCurrency}
                  loading={loadingDisplayCurrencies}
                  disabled={loadingDisplayCurrencies || displayCurrencyOptions.length === 0}
                  onChange={(_, { value }) => {
                    const next = resolveDisplayCurrency(displayCurrencyIndex, value);
                    setDisplayCurrency(next);
                    storeDisplayCurrency(next);
                  }}
                />
              </div>
            </div>
          </Card.Header>

          <Grid columns={2} stackable>
            <Grid.Column>
              <Card fluid className='router-soft-card router-soft-card-fill'>
                <Card.Content className='router-card-fill'>
                  <Card.Header className='router-card-header'>
                    <Header as='h3' className='router-section-title router-title-accent-primary'>
                      <i className='credit card icon'></i>
                      {t('topup.external_topup.title')}
                    </Header>
                  </Card.Header>
                  <Card.Description className='router-card-fill'>
                    <div className='router-card-body-spread'>
                      <div className='router-center-panel'>
                        <Statistic className='router-accent-statistic'>
                          <Statistic.Value>{renderDisplayAmount(userBalanceYYC)}</Statistic.Value>
                          <Statistic.Label>{t('topup.external_topup.current_balance')}</Statistic.Label>
                        </Statistic>
                        <div className='router-text-muted' style={{ marginTop: '0.75rem' }}>
                          {t('topup.external_topup.description')}
                        </div>
                        <Form.Input
                          className='router-section-input'
                          fluid
                          style={{ marginTop: '1rem', textAlign: 'left' }}
                          label={t('topup.external_topup.amount')}
                          type='number'
                          min={0}
                          step='0.01'
                          placeholder={t('topup.external_topup.amount_placeholder')}
                          value={topupAmount}
                          onChange={(e) => setTopupAmount(e.target.value || '0')}
                        />
                        <div className='router-text-muted' style={{ marginTop: '0.5rem' }}>
                          {t('topup.external_topup.credited_yyc')}：{renderDisplayAmount(estimatedTopupYYC)}
                        </div>
                      </div>

                      <div className='router-action-footer'>
                        <Button
                          className='router-section-button router-action-button-wide'
                          primary
                          onClick={openExternalTopup}
                          loading={isCreatingTopUpOrder}
                          disabled={isCreatingTopUpOrder || !externalTopupLink}
                        >
                          {isCreatingTopUpOrder
                            ? t('topup.external_topup.creating')
                            : t('topup.external_topup.button')}
                        </Button>
                      </div>
                    </div>
                  </Card.Description>
                </Card.Content>
              </Card>
            </Grid.Column>

            <Grid.Column>
              <Card fluid className='router-soft-card router-soft-card-fill'>
                <Card.Content className='router-card-fill'>
                  <Card.Header className='router-card-header'>
                    <Header as='h3' className='router-section-title router-title-accent-positive'>
                      <i className='boxes icon'></i>
                      {t('topup.external_topup.package_title')}
                    </Header>
                  </Card.Header>
                  <Card.Description className='router-card-fill'>
                    <div className='router-card-body-spread'>
                      <div style={{ display: 'grid', gap: '0.75rem' }}>
                        {packages.length === 0 ? (
                          <div className='router-text-muted'>
                            {t('topup.external_topup.package_empty')}
                          </div>
                        ) : (
                          packages.map((item) => {
                            const selected = item?.id === selectedPackageId;
                            return (
                              <div
                                key={item?.id || '-'}
                                onClick={() => setSelectedPackageId(item?.id || '')}
                                style={{
                                  border: selected ? '1px solid #2563eb' : '1px solid #e5e7eb',
                                  borderRadius: '12px',
                                  padding: '12px 14px',
                                  cursor: 'pointer',
                                  background: selected ? '#eff6ff' : '#fff',
                                  textAlign: 'left',
                                }}
                              >
                                <div style={{ fontWeight: 600 }}>{item?.name || '-'}</div>
                                <div className='router-text-muted' style={{ marginTop: '0.35rem' }}>
                                  {item?.description || '-'}
                                </div>
                                <div style={{ marginTop: '0.5rem' }}>
                                  {`${item?.sale_currency || 'CNY'} ${Number(item?.sale_price ?? 0).toFixed(2)}`}
                                </div>
                              </div>
                            );
                          })
                        )}
                      </div>

                      <div className='router-action-footer'>
                        <Button
                          className='router-section-button'
                          color='green'
                          fluid
                          onClick={openPackagePurchase}
                          loading={isCreatingTopUpOrder}
                          disabled={isCreatingTopUpOrder || !externalTopupLink || packages.length === 0}
                        >
                          {isCreatingTopUpOrder
                            ? t('topup.external_topup.creating')
                            : t('topup.external_topup.package_button')}
                        </Button>
                      </div>
                    </div>
                  </Card.Description>
                </Card.Content>
              </Card>
            </Grid.Column>
          </Grid>

          <Card fluid className='router-soft-card router-soft-card-fill' style={{ marginTop: '1rem' }}>
            <Card.Content className='router-card-fill'>
              <Card.Header className='router-card-header'>
                <Header as='h3' className='router-section-title router-title-accent-positive'>
                  <i className='ticket alternate icon'></i>
                  {t('topup.redeem.title')}
                </Header>
              </Card.Header>
              <Card.Description className='router-card-fill'>
                <div className='router-card-body-spread'>
                  <div className='router-text-muted'>
                    {t('topup.redeem.description')}
                  </div>

                  <Form.Input
                    className='router-section-input'
                    fluid
                    icon='key'
                    iconPosition='left'
                    placeholder={t('topup.redeem.placeholder')}
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
                            const text = await navigator.clipboard.readText();
                            setRedemptionCode(text.trim());
                          } catch (err) {
                            showError(t('topup.redeem.paste_error'));
                          }
                        }}
                      >
                        {t('topup.redeem.paste')}
                      </Button>
                    }
                  />

                  <div className='router-action-footer'>
                    <Button
                      className='router-section-button'
                      color='green'
                      fluid
                      onClick={submitRedemption}
                      loading={isSubmitting}
                      disabled={isSubmitting}
                    >
                      {isSubmitting
                        ? t('topup.redeem.submitting')
                        : t('topup.redeem.submit')}
                    </Button>
                  </div>
                </div>
              </Card.Description>
            </Card.Content>
          </Card>

          {recentRedemptionResult ? (
            <Card fluid className='router-soft-card' style={{ marginTop: '1rem' }}>
              <Card.Content>
                <Card.Header className='router-card-header'>
                  <div className='router-toolbar'>
                    <Header as='h3' className='router-section-title router-title-accent-warning'>
                      <i className='check circle icon'></i>
                      {t('topup.redemption_result.title')}
                    </Header>
                    <Button
                      className='router-section-button'
                      basic
                      size='small'
                      onClick={() => setRecentRedemptionResult(null)}
                    >
                      {t('topup.redemption_result.close')}
                    </Button>
                  </div>
                </Card.Header>
                <Table basic='very' compact='very' className='router-list-table'>
                  <Table.Body>
                    <Table.Row>
                      <Table.Cell width={4}>{t('topup.redemption_result.fields.redeemed_amount')}</Table.Cell>
                      <Table.Cell>{renderDisplayAmount(recentRedemptionResult.redeemed_yyc)}</Table.Cell>
                      <Table.Cell width={4}>{t('topup.redemption_result.fields.redeemed_at')}</Table.Cell>
                      <Table.Cell>
                        {recentRedemptionResult.redeemed_at
                          ? timestamp2string(recentRedemptionResult.redeemed_at)
                          : '-'}
                      </Table.Cell>
                    </Table.Row>
                    <Table.Row>
                      <Table.Cell>{t('topup.redemption_result.fields.before_balance')}</Table.Cell>
                      <Table.Cell>{renderDisplayAmount(recentRedemptionResult.before_yyc_balance)}</Table.Cell>
                      <Table.Cell>{t('topup.redemption_result.fields.after_balance')}</Table.Cell>
                      <Table.Cell>{renderDisplayAmount(recentRedemptionResult.after_yyc_balance)}</Table.Cell>
                    </Table.Row>
                    <Table.Row>
                      <Table.Cell>{t('topup.redemption_result.fields.redemption_name')}</Table.Cell>
                      <Table.Cell>{recentRedemptionResult.redemption_name || '-'}</Table.Cell>
                      <Table.Cell>{t('topup.redemption_result.fields.redemption_id')}</Table.Cell>
                      <Table.Cell>{recentRedemptionResult.redemption_id || '-'}</Table.Cell>
                    </Table.Row>
                    <Table.Row>
                      <Table.Cell>{t('topup.redemption_result.fields.group')}</Table.Cell>
                      <Table.Cell>
                        {recentRedemptionResult.group_name || recentRedemptionResult.group_id || '-'}
                      </Table.Cell>
                      <Table.Cell>{t('topup.redemption_result.fields.face_value')}</Table.Cell>
                      <Table.Cell>
                        {recentRedemptionResult.face_value_amount > 0
                          ? formatAmountWithUnit(
                              recentRedemptionResult.face_value_amount,
                              recentRedemptionResult.face_value_unit || 'YYC'
                            )
                          : '-'}
                      </Table.Cell>
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
                    {t('topup.external_topup_orders.title')}
                  </Header>
                  <Button
                    className='router-section-button'
                    onClick={loadExternalTopupOrders}
                    loading={loadingExternalTopupOrders}
                  >
                    {t('topup.external_topup_orders.refresh')}
                  </Button>
                </div>
              </Card.Header>
              <Table basic='very' compact className='router-list-table'>
                <Table.Header>
                  <Table.Row>
                    <Table.HeaderCell width={3}>
                      {t('topup.external_topup_orders.columns.time')}
                    </Table.HeaderCell>
                    <Table.HeaderCell width={2}>
                      {t('topup.external_topup_orders.columns.status')}
                    </Table.HeaderCell>
                    <Table.HeaderCell width={4}>
                      {t('topup.external_topup_orders.columns.order_id')}
                    </Table.HeaderCell>
                    <Table.HeaderCell>
                      {t('topup.external_topup_orders.columns.transaction_id')}
                    </Table.HeaderCell>
                  </Table.Row>
                </Table.Header>
                <Table.Body>
                  {externalTopupOrders.length === 0 ? (
                    <Table.Row>
                      <Table.Cell colSpan='4' className='router-text-muted'>
                        {loadingExternalTopupOrders
                          ? t('common.loading')
                          : t('topup.external_topup_orders.empty')}
                      </Table.Cell>
                    </Table.Row>
                  ) : (
                    externalTopupOrders.map((order) => (
                      <Table.Row key={order.id}>
                        <Table.Cell>
                          {order.created_at ? timestamp2string(order.created_at) : '-'}
                        </Table.Cell>
                        <Table.Cell>{renderExternalTopupOrderStatus(order.status)}</Table.Cell>
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
                    {t('topup.redemption_records.title')}
                  </Header>
                  <Button
                    className='router-section-button'
                    onClick={loadRedemptionRecords}
                    loading={loadingRedemptionRecords}
                  >
                    {t('topup.redemption_records.refresh')}
                  </Button>
                </div>
              </Card.Header>
              <Table basic='very' compact className='router-list-table'>
                <Table.Header>
                    <Table.Row>
                      <Table.HeaderCell width={3}>
                        {t('topup.redemption_records.columns.time')}
                      </Table.HeaderCell>
                      <Table.HeaderCell width={2}>
                        {t('topup.redemption_records.columns.amount')}
                      </Table.HeaderCell>
                      <Table.HeaderCell>
                        {t('topup.redemption_records.columns.detail')}
                    </Table.HeaderCell>
                  </Table.Row>
                </Table.Header>
                <Table.Body>
                  {redemptionRecords.length === 0 ? (
                    <Table.Row>
                      <Table.Cell colSpan='3' className='router-text-muted'>
                        {loadingRedemptionRecords
                          ? t('common.loading')
                          : t('topup.redemption_records.empty')}
                      </Table.Cell>
                    </Table.Row>
                  ) : (
                    redemptionRecords.map((log) => (
                      <Table.Row key={log.trace_id || `${log.created_at}-${log.content}`}>
                        <Table.Cell>
                          {log.created_at ? timestamp2string(log.created_at) : '-'}
                        </Table.Cell>
                        <Table.Cell>
                          {log.yycAmount ? (
                            <Label basic color='green' className='router-tag'>
                              {renderDisplayAmount(log.yycAmount)}
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
