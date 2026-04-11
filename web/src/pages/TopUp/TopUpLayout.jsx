import React, { useEffect, useMemo } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { Card } from 'semantic-ui-react';
import BalanceStatusPage from './BalanceStatusPage';
import CurrentPackagePage from './CurrentPackagePage';
import TopUpRecordsPage from './TopUpRecordsPage';
import {
  normalizeTopUpRecord,
  normalizeTopUpTab,
  sanitizeTopUpSearchParams,
} from './shared.jsx';
import TopUpWorkspaceProvider from './provider.jsx';

const TopUpLayout = () => {
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

  return (
    <TopUpWorkspaceProvider>
      <div className='dashboard-container'>
        <Card fluid className='chart-card'>
          <Card.Content>
            {activeContent}
          </Card.Content>
        </Card>
      </div>
    </TopUpWorkspaceProvider>
  );
};

export default TopUpLayout;
