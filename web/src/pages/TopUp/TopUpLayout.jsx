import React, { useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate, useSearchParams } from 'react-router-dom';
import BalanceStatusPage from './BalanceStatusPage';
import CurrentPackagePage from './CurrentPackagePage';
import TopUpRecordsPage from './TopUpRecordsPage';
import {
  normalizeTopUpRecord,
  normalizeTopUpTab,
  sanitizeTopUpSearchParams,
} from './shared.jsx';
import TopUpWorkspaceProvider from './provider.jsx';
import { AppFilterHeader } from '../../router-ui';

const TopUpLayout = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const searchParamsString = searchParams.toString();
  const rawTab = searchParams.get('tab');
  const rawRecord = searchParams.get('record');

  useEffect(() => {
    const normalizedTab = normalizeTopUpTab(rawTab);
    const normalizedRecord = normalizeTopUpRecord(rawRecord);
    const nextSearchParams = sanitizeTopUpSearchParams(searchParamsString);
    let changed = false;

    if (searchParamsString !== nextSearchParams.toString()) {
      changed = true;
    }

    if (rawTab !== normalizedTab) {
      nextSearchParams.set('tab', normalizedTab);
      changed = true;
    }
    if (normalizedTab === 'records') {
      if (rawRecord !== normalizedRecord) {
        nextSearchParams.set('record', normalizedRecord);
        changed = true;
      }
    } else if (nextSearchParams.has('record')) {
      nextSearchParams.delete('record');
      changed = true;
    }

    if (!changed) {
      return;
    }
    navigate(`/workspace/topup?${nextSearchParams.toString()}`, { replace: true });
  }, [navigate, rawRecord, rawTab, searchParamsString]);

  const activeKey = normalizeTopUpTab(rawTab);
  const activeRecord = normalizeTopUpRecord(rawRecord);

  const activeContent = useMemo(() => {
    switch (activeKey) {
      case 'package':
        return <CurrentPackagePage />;
      case 'records':
        return <TopUpRecordsPage recordKey={activeRecord} />;
      case 'balance':
      default:
        return <BalanceStatusPage />;
    }
  }, [activeKey, activeRecord]);

  const activeTitle = useMemo(() => {
    if (activeKey === 'package') {
      return t('topup.mine.package');
    }
    if (activeKey === 'records') {
      switch (activeRecord) {
        case 'package':
          return t('topup.records.package_title', '套餐订单');
        case 'redeem':
          return t('topup.redemption_records.title', '兑换记录');
        case 'topup':
        default:
          return t('topup.records.title', '充值订单');
      }
    }
    return t('topup.mine.balance');
  }, [activeKey, activeRecord, t]);

  return (
    <TopUpWorkspaceProvider>
      <div className='dashboard-container'>
        <AppFilterHeader
          breadcrumbs={[
            { key: 'workspace', label: t('header.user_workspace') },
            { key: 'mine', label: t('header.mine') },
            { key: 'topup', label: activeTitle, active: true },
          ]}
          title={activeTitle}
        />
        {activeContent}
      </div>
    </TopUpWorkspaceProvider>
  );
};

export default TopUpLayout;
