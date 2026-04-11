import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import {
  Button,
  Card,
  Dropdown,
  Header,
  Label,
  Modal,
  Statistic,
} from 'semantic-ui-react';
import { API, showError, showInfo, timestamp2string } from '../../helpers';
import {
  buildTopUpReturnURL,
  renderTopupIntegerAmountWithExactPopup,
  useTopUpWorkspace,
} from './shared.jsx';

const createEmptyActivePackage = () => ({
  has_active_subscription: false,
  subscription: null,
});

const normalizeActivePackage = (raw) => {
  const subscription = raw?.subscription
    ? {
        id: (raw.subscription.id || '').toString().trim(),
        package_id: (raw.subscription.package_id || '').toString().trim(),
        package_name: (raw.subscription.package_name || '').toString().trim(),
        group_id: (raw.subscription.group_id || '').toString().trim(),
        group_name: (raw.subscription.group_name || '').toString().trim(),
        source: (raw.subscription.source || '').toString().trim(),
        status: Number(raw.subscription.status || 0),
        daily_quota_limit: Number(raw.subscription.daily_quota_limit || 0),
        package_emergency_quota_limit: Number(
          raw.subscription.package_emergency_quota_limit || 0,
        ),
        quota_reset_timezone: (
          raw.subscription.quota_reset_timezone || ''
        ).toString().trim(),
        started_at: Number(raw.subscription.started_at || 0),
        expires_at: Number(raw.subscription.expires_at || 0),
      }
    : null;
  return {
    has_active_subscription:
      raw?.has_active_subscription === true && subscription !== null,
    subscription,
  };
};

const createEmptyDailySnapshot = () => ({
  biz_date: '',
  timezone: '',
  limit: 0,
  consumed_quota: 0,
  reserved_quota: 0,
  remaining_quota: 0,
  unlimited: false,
});

const createEmptyQuotaSummary = () => ({
  package_emergency: {
    biz_month: '',
    timezone: '',
    limit: 0,
    consumed_quota: 0,
    reserved_quota: 0,
    remaining_quota: 0,
    enabled: false,
  },
});

const normalizeDailySnapshot = (raw) => ({
  biz_date: (raw?.biz_date || '').toString().trim(),
  timezone: (raw?.timezone || '').toString().trim(),
  limit: Number(raw?.yyc_limit ?? raw?.limit ?? 0) || 0,
  consumed_quota: Number(raw?.yyc_consumed ?? raw?.consumed_quota ?? 0) || 0,
  reserved_quota: Number(raw?.yyc_reserved ?? raw?.reserved_quota ?? 0) || 0,
  remaining_quota: Number(raw?.yyc_remaining ?? raw?.remaining_quota ?? 0) || 0,
  unlimited: raw?.unlimited === true,
});

const normalizeQuotaSummary = (raw) => ({
  package_emergency: {
    biz_month: (raw?.package_emergency?.biz_month || '').toString().trim(),
    timezone: (raw?.package_emergency?.timezone || '').toString().trim(),
    limit: Number(
      raw?.package_emergency?.yyc_limit ??
        raw?.package_emergency?.limit ??
        0,
    ) || 0,
    consumed_quota:
      Number(
        raw?.package_emergency?.yyc_consumed ??
          raw?.package_emergency?.consumed_quota ??
          0,
      ) || 0,
    reserved_quota:
      Number(
        raw?.package_emergency?.yyc_reserved ??
          raw?.package_emergency?.reserved_quota ??
          0,
      ) || 0,
    remaining_quota:
      Number(
        raw?.package_emergency?.yyc_remaining ??
          raw?.package_emergency?.remaining_quota ??
          0,
      ) || 0,
    enabled: raw?.package_emergency?.enabled === true,
  },
});

const renderPackageStatus = (status, t) => {
  switch (Number(status || 0)) {
    case 1:
      return (
        <Label basic color='green' className='router-tag'>
          {t('user.detail.package_status_types.active')}
        </Label>
      );
    case 2:
      return (
        <Label basic color='grey' className='router-tag'>
          {t('user.detail.package_status_types.expired')}
        </Label>
      );
    case 3:
      return (
        <Label basic color='grey' className='router-tag'>
          {t('user.detail.package_status_types.replaced')}
        </Label>
      );
    case 4:
      return (
        <Label basic color='red' className='router-tag'>
          {t('user.detail.package_status_types.canceled')}
        </Label>
      );
    case 5:
      return (
        <Label basic color='teal' className='router-tag'>
          {t('user.detail.package_status_types.pending')}
        </Label>
      );
    default:
      return (
        <Label basic className='router-tag'>
          {t('user.detail.package_status_types.unknown')}
        </Label>
      );
  }
};

const PackageUsageCard = ({ title, period, timezone, items, footer }) => (
  <Card fluid className='router-soft-card'>
    <Card.Content>
      <Card.Header className='router-card-header'>
        <Header as='h3' className='router-section-title'>
          {title}
        </Header>
      </Card.Header>
      <div
        style={{
          display: 'grid',
          gap: '1rem',
        }}
      >
        <Statistic.Group widths='three' size='small'>
          {items.map((item) => (
            <Statistic key={item.key}>
              <Statistic.Value className='router-topup-statistic-value'>
                {item.value}
              </Statistic.Value>
              <Statistic.Label>{item.label}</Statistic.Label>
            </Statistic>
          ))}
        </Statistic.Group>
        <div
          style={{
            display: 'flex',
            gap: '1rem',
            flexWrap: 'wrap',
            color: '#6b7280',
            fontSize: '0.92rem',
          }}
        >
          <span>{period}</span>
          <span>{timezone}</span>
          {footer ? <span>{footer}</span> : null}
        </div>
      </div>
    </Card.Content>
  </Card>
);

const CurrentPackagePage = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const {
    displayCurrency,
    displayCurrencyIndex,
    previewPackagePurchase,
    createTopupOrder,
  } = useTopUpWorkspace();
  const [loading, setLoading] = useState(false);
  const [activePackage, setActivePackage] = useState(createEmptyActivePackage());
  const [dailySnapshot, setDailySnapshot] = useState(createEmptyDailySnapshot());
  const [quotaSummary, setQuotaSummary] = useState(createEmptyQuotaSummary());
  const [renewing, setRenewing] = useState(false);
  const [upgradeModalOpen, setUpgradeModalOpen] = useState(false);
  const [loadingUpgradeTargets, setLoadingUpgradeTargets] = useState(false);
  const [upgradeTargets, setUpgradeTargets] = useState([]);
  const [selectedUpgradePackageId, setSelectedUpgradePackageId] = useState('');
  const [submittingUpgrade, setSubmittingUpgrade] = useState(false);

  const renderIntegerAmount = useCallback(
    (yycAmount) =>
      renderTopupIntegerAmountWithExactPopup({
        yycAmount,
        displayCurrency,
        displayCurrencyIndex,
      }),
    [displayCurrency, displayCurrencyIndex],
  );

  const loadQuotaSummary = useCallback(async () => {
    const res = await API.get('/api/v1/public/user/quota/summary');
    const { success, message, data } = res?.data || {};
    if (!success) {
      throw new Error(message || t('user.messages.operation_failed'));
    }
    setQuotaSummary(normalizeQuotaSummary(data));
  }, [t]);

  const loadDailySnapshot = useCallback(
    async (groupId) => {
      const normalizedGroupId = (groupId || '').toString().trim();
      if (normalizedGroupId === '') {
        setDailySnapshot(createEmptyDailySnapshot());
        return;
      }
      const res = await API.get('/api/v1/public/user/quota/daily', {
        params: {
          group_id: normalizedGroupId,
        },
      });
      const { success, message, data } = res?.data || {};
      if (!success) {
        throw new Error(message || t('user.messages.operation_failed'));
      }
      setDailySnapshot(normalizeDailySnapshot(data));
    },
    [t],
  );

  const loadPackageStatus = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/v1/public/user/package/subscription');
      const { success, message, data } = res?.data || {};
      if (!success) {
        throw new Error(message || t('user.messages.active_package_load_failed'));
      }
      const normalizedPackage = normalizeActivePackage(data);
      setActivePackage(normalizedPackage);
      if (normalizedPackage.has_active_subscription) {
        await Promise.all([
          loadDailySnapshot(normalizedPackage.subscription?.group_id),
          loadQuotaSummary(),
        ]);
        return;
      }
      setDailySnapshot(createEmptyDailySnapshot());
      setQuotaSummary(createEmptyQuotaSummary());
    } catch (error) {
      showError(error?.message || t('user.messages.active_package_load_failed'));
    } finally {
      setLoading(false);
    }
  }, [loadDailySnapshot, loadQuotaSummary, t]);

  useEffect(() => {
    loadPackageStatus().then();
  }, [loadPackageStatus]);

  const activeSubscription = activePackage.has_active_subscription
    ? activePackage.subscription
    : null;

  const infoItems = useMemo(() => {
    if (!activeSubscription) {
      return [];
    }
    return [
      {
        key: 'package_name',
        label: t('user.detail.package_name'),
        value: activeSubscription.package_name || '-',
      },
      {
        key: 'status',
        label: t('user.detail.package_status'),
        value: renderPackageStatus(activeSubscription.status, t),
      },
      {
        key: 'daily_limit',
        label: t('user.detail.package_daily_limit'),
        value: renderIntegerAmount(activeSubscription.daily_quota_limit || 0),
      },
      {
        key: 'emergency_limit',
        label: t('user.detail.package_emergency_limit'),
        value: renderIntegerAmount(
          activeSubscription.package_emergency_quota_limit || 0,
        ),
      },
      {
        key: 'timezone',
        label: t('user.detail.package_timezone'),
        value: activeSubscription.quota_reset_timezone || '-',
      },
      {
        key: 'started_at',
        label: t('user.detail.package_started_at'),
        value: activeSubscription.started_at
          ? timestamp2string(activeSubscription.started_at)
          : '-',
      },
      {
        key: 'expires_at',
        label: t('user.detail.package_expires_at'),
        value: activeSubscription.expires_at
          ? timestamp2string(activeSubscription.expires_at)
          : '-',
      },
    ];
  }, [activeSubscription, renderIntegerAmount, t]);

  const dailyItems = useMemo(() => {
    if (!activeSubscription) {
      return [];
    }
    return [
      {
        key: 'daily_limit',
        label: t('user.detail.package_daily_limit'),
        value: dailySnapshot.unlimited
          ? t('common.unlimited')
          : renderIntegerAmount(dailySnapshot.limit),
      },
      {
        key: 'daily_used',
        label: t('user.detail.used_amount'),
        value: renderIntegerAmount(dailySnapshot.consumed_quota),
      },
      {
        key: 'daily_remaining',
        label: t('user.detail.remaining_amount'),
        value: dailySnapshot.unlimited
          ? t('common.unlimited')
          : renderIntegerAmount(dailySnapshot.remaining_quota),
      },
    ];
  }, [activeSubscription, dailySnapshot, renderIntegerAmount, t]);

  const emergencySnapshot = quotaSummary.package_emergency;
  const emergencyItems = useMemo(() => {
    if (!activeSubscription) {
      return [];
    }
    return [
      {
        key: 'emergency_limit',
        label: t('user.detail.package_emergency_limit'),
        value: emergencySnapshot.enabled
          ? renderIntegerAmount(emergencySnapshot.limit)
          : '-',
      },
      {
        key: 'emergency_used',
        label: t('user.detail.used_amount'),
        value: emergencySnapshot.enabled
          ? renderIntegerAmount(emergencySnapshot.consumed_quota)
          : '-',
      },
      {
        key: 'emergency_remaining',
        label: t('user.detail.remaining_amount'),
        value: emergencySnapshot.enabled
          ? renderIntegerAmount(emergencySnapshot.remaining_quota)
          : '-',
      },
    ];
  }, [activeSubscription, emergencySnapshot, renderIntegerAmount, t]);

  const goPricing = useCallback(
    (intent = '') => {
      const normalizedIntent = String(intent || '').trim().toLowerCase();
      const search = new URLSearchParams();
      if (normalizedIntent === 'renew' || normalizedIntent === 'upgrade') {
        search.set('intent', normalizedIntent);
      }
      navigate(`/workspace/service/pricing${search.toString() ? `?${search.toString()}` : ''}`);
    },
    [navigate],
  );

  const quickPurchasePackage = useCallback(
    async (packageID, requestedOperationType = '') => {
      const normalizedPackageID = (packageID || '').toString().trim();
      if (normalizedPackageID === '') {
        showInfo(t('topup.external_topup.package_select_required'));
        return false;
      }
      const preview = await previewPackagePurchase({
        package_id: normalizedPackageID,
        operation_type: (requestedOperationType || '').toString().trim(),
      });
      if (!preview) {
        return false;
      }
      const operationType = String(
        preview?.operation_type || requestedOperationType || '',
      ).trim();
      return createTopupOrder({
        business_type: 'package_purchase',
        operation_type: operationType,
        package_id: normalizedPackageID,
        return_url: buildTopUpReturnURL(),
      });
    },
    [createTopupOrder, previewPackagePurchase, t],
  );

  const handleRenew = useCallback(async () => {
    const packageID = (activeSubscription?.package_id || '').toString().trim();
    if (packageID === '') {
      showInfo(t('topup.package_status.no_active_package'));
      return;
    }
    setRenewing(true);
    try {
      await quickPurchasePackage(packageID, 'renew');
    } finally {
      setRenewing(false);
    }
  }, [activeSubscription?.package_id, quickPurchasePackage, t]);

  const buildUpgradeOptionText = useCallback((item) => {
    const name = String(item?.name || item?.package_name || item?.id || '-').trim();
    const price = Number(item?.sale_price ?? 0);
    const currency = String(item?.sale_currency || 'USD').toUpperCase();
    if (!Number.isFinite(price) || price <= 0) {
      return name;
    }
    return `${name} (${currency} ${price.toFixed(2)})`;
  }, []);

  const loadUpgradeTargets = useCallback(async () => {
    const currentPackageID = (activeSubscription?.package_id || '').toString().trim();
    if (currentPackageID === '') {
      showInfo(t('topup.package_status.no_active_package'));
      return [];
    }
    setLoadingUpgradeTargets(true);
    try {
      const res = await API.get('/api/v1/public/user/packages');
      const { success, message, data } = res?.data || {};
      if (!success) {
        throw new Error(message || t('topup.external_topup.request_failed'));
      }
      const rows = Array.isArray(data) ? data : [];
      const candidates = rows.filter((row) => {
        const rowID = String(row?.id || '').trim();
        if (rowID === '' || rowID === currentPackageID) {
          return false;
        }
        const status = Number(row?.status ?? 1);
        return !Number.isFinite(status) || status === 1;
      });
      return candidates;
    } catch (error) {
      showError(error?.message || t('topup.external_topup.request_failed'));
      return [];
    } finally {
      setLoadingUpgradeTargets(false);
    }
  }, [activeSubscription?.package_id, t]);

  const handleUpgrade = useCallback(async () => {
    const candidates = await loadUpgradeTargets();
    if (candidates.length === 0) {
      showInfo(t('topup.package_status.no_upgrade_target'));
      return;
    }
    if (candidates.length === 1) {
      setSubmittingUpgrade(true);
      try {
        await quickPurchasePackage(candidates[0]?.id, 'upgrade');
      } finally {
        setSubmittingUpgrade(false);
      }
      return;
    }
    const defaultTargetID = String(candidates[0]?.id || '').trim();
    setUpgradeTargets(candidates);
    setSelectedUpgradePackageId(defaultTargetID);
    setUpgradeModalOpen(true);
  }, [loadUpgradeTargets, quickPurchasePackage, t]);

  const handleConfirmUpgrade = useCallback(async () => {
    const targetPackageID = (selectedUpgradePackageId || '').trim();
    if (targetPackageID === '') {
      showInfo(t('topup.external_topup.package_select_required'));
      return;
    }
    setSubmittingUpgrade(true);
    try {
      const created = await quickPurchasePackage(targetPackageID, 'upgrade');
      if (created) {
        setUpgradeModalOpen(false);
      }
    } finally {
      setSubmittingUpgrade(false);
    }
  }, [quickPurchasePackage, selectedUpgradePackageId, t]);

  const upgradeOptions = useMemo(
    () =>
      (upgradeTargets || []).map((item) => ({
        key: String(item?.id || ''),
        value: String(item?.id || ''),
        text: buildUpgradeOptionText(item),
      })),
    [buildUpgradeOptionText, upgradeTargets],
  );

  return (
    <div style={{ display: 'grid', gap: '1rem' }}>
      <Card fluid className='router-soft-card'>
        <Card.Content>
          <Card.Header className='router-card-header'>
            <div className='router-toolbar'>
              <Header as='h3' className='router-section-title router-title-accent-positive'>
                {t('user.detail.package_title')}
              </Header>
              <div className='router-toolbar-end'>
                <Button
                  className='router-section-button'
                  onClick={() => goPricing('')}
                >
                  {t('topup.package_status.view_pricing')}
                </Button>
                <Button
                  className='router-section-button'
                  basic
                  disabled={!activeSubscription || loadingUpgradeTargets}
                  loading={renewing}
                  onClick={handleRenew}
                >
                  {t('topup.external_topup.package_operation.renew')}
                </Button>
                <Button
                  className='router-section-button'
                  basic
                  disabled={!activeSubscription}
                  loading={loadingUpgradeTargets || submittingUpgrade}
                  onClick={handleUpgrade}
                >
                  {t('topup.external_topup.package_operation.upgrade')}
                </Button>
              </div>
            </div>
          </Card.Header>

          {loading ? (
            <div className='router-text-muted'>{t('common.loading')}</div>
          ) : !activeSubscription ? (
            <div
              style={{
                display: 'grid',
                gap: '0.75rem',
              }}
            >
              <div className='router-text-muted'>
                {t('topup.package_status.empty_description')}
              </div>
            </div>
          ) : (
            <div
              style={{
                display: 'grid',
                gap: '0.85rem',
                gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))',
              }}
            >
              {infoItems.map((item) => (
                <div
                  key={item.key}
                  style={{
                    border: '1px solid #e5e7eb',
                    borderRadius: '12px',
                    padding: '0.85rem 1rem',
                    background: '#ffffff',
                  }}
                >
                  <div
                    style={{
                      fontSize: '0.85rem',
                      color: '#6b7280',
                      marginBottom: '0.35rem',
                    }}
                  >
                    {item.label}
                  </div>
                  <div style={{ fontSize: '1rem', color: '#111827' }}>{item.value}</div>
                </div>
              ))}
            </div>
          )}
        </Card.Content>
      </Card>

      {activeSubscription ? (
        <>
          <PackageUsageCard
            title={t('topup.package_status.daily_title')}
            period={`${t('topup.package_status.period')}: ${dailySnapshot.biz_date || '-'}`}
            timezone={`${t('user.detail.package_timezone')}: ${dailySnapshot.timezone || activeSubscription.quota_reset_timezone || '-'}`}
            footer={
              <>
                {t('topup.package_status.reserved')}: {renderIntegerAmount(dailySnapshot.reserved_quota)}
              </>
            }
            items={dailyItems}
          />
          <PackageUsageCard
            title={t('topup.package_status.emergency_title')}
            period={`${t('topup.package_status.period')}: ${emergencySnapshot.biz_month || '-'}`}
            timezone={`${t('user.detail.package_timezone')}: ${emergencySnapshot.timezone || activeSubscription.quota_reset_timezone || '-'}`}
            footer={
              <>
                {t('topup.package_status.reserved')}:{' '}
                {emergencySnapshot.enabled
                  ? renderIntegerAmount(emergencySnapshot.reserved_quota)
                  : '-'}
              </>
            }
            items={emergencyItems}
          />
        </>
      ) : null}

      <Modal
        size='small'
        open={upgradeModalOpen}
        onClose={() => {
          if (submittingUpgrade) {
            return;
          }
          setUpgradeModalOpen(false);
        }}
      >
        <Modal.Header>{t('topup.package_status.select_upgrade_target')}</Modal.Header>
        <Modal.Content>
          <div style={{ display: 'grid', gap: '0.8rem' }}>
            <div className='router-text-muted'>
              {t('topup.package_status.select_upgrade_target_hint')}
            </div>
            <Dropdown
              className='router-page-dropdown'
              fluid
              selection
              options={upgradeOptions}
              value={selectedUpgradePackageId}
              onChange={(_, data) =>
                setSelectedUpgradePackageId(String(data?.value || ''))
              }
            />
          </div>
        </Modal.Content>
        <Modal.Actions>
          <Button
            className='router-section-button'
            onClick={() => setUpgradeModalOpen(false)}
            disabled={submittingUpgrade}
          >
            {t('common.cancel')}
          </Button>
          <Button
            primary
            className='router-section-button'
            loading={submittingUpgrade}
            disabled={submittingUpgrade || selectedUpgradePackageId === ''}
            onClick={handleConfirmUpgrade}
          >
            {t('topup.package_status.upgrade_now')}
          </Button>
        </Modal.Actions>
      </Modal>
    </div>
  );
};

export default CurrentPackagePage;
