import React, { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate, useSearchParams } from 'react-router-dom';
import QuotaPage from './QuotaPage';
import TopUpWorkspaceProvider from './provider.jsx';
import { AppFilterHeader } from '../../router-ui';

const TopUpLayout = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const rawTab = searchParams.get('tab');
  const rawRecord = searchParams.get('record');
  const rawHistory = searchParams.get('history');

  useEffect(() => {
    if (rawHistory || rawRecord || rawTab === 'records') {
      navigate('/workspace/topup/history', { replace: true });
    }
  }, [navigate, rawHistory, rawRecord, rawTab]);

  return (
    <TopUpWorkspaceProvider>
      <div className='dashboard-container'>
        <AppFilterHeader
          breadcrumbs={[
            { key: 'workspace', label: t('header.user_workspace') },
            { key: 'mine', label: t('header.mine') },
            { key: 'topup', label: t('topup.mine.quota'), active: true },
          ]}
          title={t('topup.mine.quota')}
        />
        <QuotaPage />
      </div>
    </TopUpWorkspaceProvider>
  );
};

export default TopUpLayout;
