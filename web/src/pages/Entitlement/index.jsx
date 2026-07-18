import React from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import TopupPlansManager from '../../components/TopupPlansManager';
import PackagesManager from '../../components/PackagesManager';
import { AppTabs } from '../../router-ui';

const TAB_KEYS = {
  topup: 'topup',
  package: 'package',
};

const Entitlement = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const requestedTab = (searchParams.get('tab') || TAB_KEYS.topup)
    .trim()
    .toLowerCase();
  const activeTab =
    requestedTab === TAB_KEYS.package ? TAB_KEYS.package : TAB_KEYS.topup;

  const handleTabChange = (nextKey) => {
    const normalizedKey =
      (nextKey || TAB_KEYS.topup).toString() === TAB_KEYS.package
        ? TAB_KEYS.package
        : TAB_KEYS.topup;
    setSearchParams({ tab: normalizedKey });
  };

  return (
    <div className='dashboard-container'>
      <div className='router-entity-detail-tabs router-block-gap-sm'>
        <AppTabs
          className='router-detail-tab-menu'
          activeKey={activeTab}
          onChange={handleTabChange}
          items={[
            { key: TAB_KEYS.topup, label: t('header.topup') },
            { key: TAB_KEYS.package, label: t('header.package') },
          ]}
        />
      </div>
      {activeTab === TAB_KEYS.package ? (
        <PackagesManager
          headerMeta={
            <button
              type='button'
              className='router-breadcrumb-link router-page-header-link'
              onClick={() => navigate('/admin/entitlement/package/records')}
            >
              购买记录
            </button>
          }
          headerBreadcrumbs={[
            { key: 'admin', label: t('header.admin_workspace') },
            { key: 'model', label: t('header.model') },
            { key: 'entitlement', label: t('header.entitlement') },
            { key: 'package', label: t('header.package'), active: true },
          ]}
          headerTitle={t('header.package')}
        />
      ) : (
        <TopupPlansManager
          headerMeta={
            <button
              type='button'
              className='router-breadcrumb-link router-page-header-link'
              onClick={() => navigate('/admin/entitlement/topup/records')}
            >
              充值记录
            </button>
          }
          headerBreadcrumbs={[
            { key: 'admin', label: t('header.admin_workspace') },
            { key: 'model', label: t('header.model') },
            { key: 'entitlement', label: t('header.entitlement') },
            { key: 'topup', label: t('header.topup'), active: true },
          ]}
          headerTitle={t('topup.manage.title')}
        />
      )}
    </div>
  );
};

export default Entitlement;
