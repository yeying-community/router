import React from 'react';
import { useTranslation } from 'react-i18next';
import { Navigate, useLocation, useSearchParams } from 'react-router-dom';
import ChannelsTable from '../../components/ChannelsTable';
import ChannelHealthSection from '../AdminDashboard/sections/ChannelHealthSection';
import PublisherOperations from '../PublisherOperations';
import { AppFilterHeader } from '../../router-ui';
import SectionTabs from '../../components/SectionTabs';
import '../AdminDashboard/Dashboard.css';
import '../AdminDashboard/AdminDashboard.css';

// Single channel shell: list / health live under
// `/admin/channel?tab=...` so switching a tab only changes the query string —
// the tab strip stays mounted and the body swaps in place instead of tearing
// down a whole route (the old cross-route SectionTabs behavior).
const TABS = [
  { key: 'list', labelKey: 'channel.tabs.list' },
  { key: 'health', labelKey: 'channel.tabs.health' },
  { key: 'supply', labelKey: 'publisher_operations.title' },
];

const VALID_TABS = new Set(TABS.map((tab) => tab.key));

const ChannelLayout = () => {
  const { t } = useTranslation();
  const location = useLocation();
  const [searchParams] = useSearchParams();
  const rawTab = searchParams.get('tab');
  const activeTab = VALID_TABS.has(rawTab) ? rawTab : 'list';

  if (rawTab === 'alerts') {
    const nextSearchParams = new URLSearchParams(searchParams);
    const legacyStatus = (nextSearchParams.get('status') || '')
      .trim()
      .toLowerCase();
    nextSearchParams.delete('tab');
    nextSearchParams.set(
      'view',
      legacyStatus === 'resolved' ? 'resolved' : 'incidents'
    );
    nextSearchParams.set('domain', 'channels');
    return (
      <Navigate
        to={`/admin/dashboard?${nextSearchParams.toString()}${location.hash}`}
        state={location.state}
        replace
      />
    );
  }

  const renderActive = () => {
    switch (activeTab) {
      case 'health':
        return <ChannelHealthSection />;
      case 'supply':
        return <PublisherOperations embedded />;
      case 'list':
      default:
        return <ChannelsTable embedded />;
    }
  };

  return (
    <div className='dashboard-container admin-dashboard-container admin-dashboard-channel-shell'>
      <AppFilterHeader
        className='admin-dashboard-toolbar'
        query={
          <SectionTabs
            active={activeTab}
            tabs={TABS.map((tab) => ({
              key: tab.key,
              label: t(tab.labelKey),
              to: `/admin/channel?tab=${tab.key}`,
            }))}
          />
        }
      />
      {renderActive()}
    </div>
  );
};

export default ChannelLayout;
