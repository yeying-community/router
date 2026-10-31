import React from 'react';
import { useTranslation } from 'react-i18next';
import { useSearchParams } from 'react-router-dom';
import QuotaPage from './QuotaPage';
import { QuotaHistoryPageInner } from './QuotaHistoryPage';
import ModelUsagePage from './ModelUsagePage';
import TopUpRecordsPage from './TopUpRecordsPage';
import Log from '../Log';
import TopUpWorkspaceProvider from './provider.jsx';
import { AppFilterHeader } from '../../router-ui';
import SectionTabs from '../../components/SectionTabs';

// Single "my usage" hub for normal users. Historical quota is a child view of
// quota (entered from the quota page), while personal model usage is a primary
// workspace tab alongside quota, payment records and the personal call log.
const TABS = [
  { key: 'quota', labelKey: 'topup.mine.quota' },
  { key: 'model_usage', labelKey: 'topup.model_usage.title' },
  { key: 'records', labelKey: 'topup.payment_history.title' },
  { key: 'logs', labelKey: 'header.log' },
];

const VIEW_LABEL_KEYS = {
  quota: 'topup.mine.quota',
  history: 'topup.quota_cards.history_title',
  model_usage: 'topup.model_usage.title',
  records: 'topup.payment_history.title',
  logs: 'header.log',
};

const VALID_VIEWS = new Set(Object.keys(VIEW_LABEL_KEYS));

const TopUpLayout = () => {
  const { t } = useTranslation();
  const [searchParams] = useSearchParams();
  const rawTab = searchParams.get('tab');
  const rawRecord = searchParams.get('record');
  const rawHistory = searchParams.get('history');

  // Legacy query aliases (?history / ?record / ?tab=records) map onto the
  // merged tabs instead of redirecting to standalone routes.
  const activeTab = rawHistory
    ? 'history'
    : rawRecord || rawTab === 'records'
      ? 'records'
      : VALID_VIEWS.has(rawTab)
        ? rawTab
        : 'quota';

  const renderActive = () => {
    switch (activeTab) {
      case 'history':
        return <QuotaHistoryPageInner embedded />;
      case 'model_usage':
        return <ModelUsagePage />;
      case 'records':
        return <TopUpRecordsPage embedded />;
      case 'logs':
        return <Log embedded />;
      case 'quota':
      default:
        return <QuotaPage />;
    }
  };

  const activeTabLabelKey = VIEW_LABEL_KEYS[activeTab] || 'topup.mine.quota';
  const navigationTab = activeTab === 'history' ? 'quota' : activeTab;

  return (
    <TopUpWorkspaceProvider>
      <div className='dashboard-container'>
        <AppFilterHeader
          breadcrumbs={[
            { key: 'workspace', label: t('header.user_workspace') },
            { key: 'mine', label: t('header.mine') },
            { key: 'topup', label: t(activeTabLabelKey), active: true },
          ]}
          query={
            <SectionTabs
              active={navigationTab}
              tabs={TABS.map((tab) => ({
                key: tab.key,
                label: t(tab.labelKey),
                to: `/workspace/topup?tab=${tab.key}`,
              }))}
            />
          }
        />
        {renderActive()}
      </div>
    </TopUpWorkspaceProvider>
  );
};

export default TopUpLayout;
