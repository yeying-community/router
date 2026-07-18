import React from 'react';
import { useTranslation } from 'react-i18next';
import AdminChannelAlertsPanel from '../../components/AdminChannelAlertsPanel';
import { AppFilterHeader } from '../../router-ui';
import '../Dashboard/Dashboard.css';
import '../AdminDashboard/AdminDashboard.css';

function AdminAlerts() {
  const { t } = useTranslation();

  return (
    <div className='dashboard-container admin-dashboard-container'>
      <AppFilterHeader
        className='admin-dashboard-toolbar'
        breadcrumbs={[
          { key: 'admin', label: t('header.admin_workspace') },
          { key: 'dashboard', label: t('header.system_overview') },
          { key: 'alerts', label: t('dashboard.admin.nav.alerts'), active: true },
        ]}
        title={t('dashboard.admin.nav.alerts')}
      />
      <AdminChannelAlertsPanel />
    </div>
  );
}

export default AdminAlerts;
