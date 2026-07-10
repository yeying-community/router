import React, { useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate, useSearchParams } from 'react-router-dom';
import QuotaPage from './QuotaPage';
import {
  normalizeTopUpHistory,
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
  const rawHistory = searchParams.get('history');

  useEffect(() => {
    const normalizedTab = normalizeTopUpTab(rawTab);
    const normalizedHistory = normalizeTopUpHistory(rawHistory, rawRecord);
    const nextSearchParams = sanitizeTopUpSearchParams(searchParamsString);
    let changed = false;

    if (searchParamsString !== nextSearchParams.toString()) {
      changed = true;
    }

    if (rawTab !== normalizedTab) {
      nextSearchParams.set('tab', normalizedTab);
      changed = true;
    }
    if (rawHistory !== normalizedHistory) {
      nextSearchParams.set('history', normalizedHistory);
      changed = true;
    }
    if (rawRecord) {
      changed = true;
    }

    if (!changed) {
      return;
    }
    navigate(`/workspace/topup?${nextSearchParams.toString()}`, {
      replace: true,
    });
  }, [navigate, rawHistory, rawRecord, rawTab, searchParamsString]);

  const activeHistory = normalizeTopUpHistory(rawHistory, rawRecord);

  const activeContent = useMemo(
    () => <QuotaPage historyKey={activeHistory} />,
    [activeHistory]
  );

  const activeTitle = useMemo(() => t('topup.mine.quota'), [t]);

  const breadcrumbParent = useMemo(
    () => ({ key: 'mine', label: t('header.mine') }),
    [t]
  );

  return (
    <TopUpWorkspaceProvider>
      <div className='dashboard-container'>
        <AppFilterHeader
          breadcrumbs={[
            { key: 'workspace', label: t('header.user_workspace') },
            breadcrumbParent,
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
