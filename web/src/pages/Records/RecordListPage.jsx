import React from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import BusinessRecordsTable from '../../components/BusinessRecordsTable';

const RECORD_CONFIG = {
  topup: {
    title: '充值记录',
    parentKey: 'header.topup',
    parentPath: '/admin/topup',
    detailBasePath: '/admin/topup/records',
  },
  package: {
    title: '购买记录',
    parentKey: 'header.package',
    parentPath: '/admin/package',
    detailBasePath: '/admin/package/records',
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

  return (
    <div className='dashboard-container'>
      <BusinessRecordsTable
        kind={kind}
        title={config.title}
        detailBasePath={config.detailBasePath}
        breadcrumbs={[
          { key: 'admin', label: t('header.admin_workspace') },
          { key: 'business', label: t('header.operation') },
          {
            key: `${kind}-parent`,
            label: t(config.parentKey),
            onClick: () => navigate(config.parentPath),
          },
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
