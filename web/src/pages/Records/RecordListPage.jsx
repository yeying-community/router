import React from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import BusinessRecordsTable from '../../components/BusinessRecordsTable';

const RECORD_CONFIG = {
  topup: {
    title: '充值记录',
    parentKey: 'header.topup',
    parentPath: '/admin/entitlement',
    detailBasePath: '/admin/entitlement/topup/records',
    scope: 'entitlement',
  },
  package: {
    title: '购买记录',
    parentPath: '/admin/entitlement',
    detailBasePath: '/admin/entitlement/package/records',
    tableKind: 'topup-reconcile',
    scope: 'entitlement',
    hideParentBreadcrumb: true,
  },
  redemption: {
    title: '兑换记录',
    parentKey: 'header.redemption',
    parentPath: '/admin/redemption',
    detailBasePath: '/admin/redemption/records',
  },
};

const RecordListPage = ({ kind }) => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const config = RECORD_CONFIG[kind] || RECORD_CONFIG.topup;
  const parentBreadcrumbs =
    config.scope === 'entitlement'
      ? [
          { key: 'model', label: t('header.model') },
          {
            key: 'entitlement',
            label: t('header.entitlement'),
            onClick: () => navigate('/admin/entitlement'),
          },
        ]
      : [
          { key: 'business', label: t('header.operation') },
        ];

  return (
    <div className='dashboard-container'>
      <BusinessRecordsTable
        kind={config.tableKind || kind}
        title={config.title}
        detailBasePath={config.detailBasePath}
        breadcrumbs={[
          { key: 'admin', label: t('header.admin_workspace') },
          ...parentBreadcrumbs,
          ...(config.hideParentBreadcrumb
            ? []
            : [{
                key: `${kind}-parent`,
                label: t(config.parentKey),
                onClick: () => navigate(config.parentPath),
              }]),
          {
            key: `${kind}-records`,
            label: config.title,
            active: true,
          },
        ]}
      />
    </div>
  );
};

export default RecordListPage;
