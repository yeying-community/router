import React from 'react';
import { Card } from 'semantic-ui-react';
import { useTranslation } from 'react-i18next';
import { useLocation } from 'react-router-dom';
import LogsTable from '../../components/LogsTable';

const Log = () => {
  const { t } = useTranslation();
  const location = useLocation();
  const isAdminWorkspace = location.pathname.startsWith('/admin/');

  return (
    <div className='dashboard-container'>
      <Card fluid className='chart-card'>
        <Card.Content>
          {isAdminWorkspace ? (
            <Card.Header className='header router-page-title'>
              {t('log.title')}
            </Card.Header>
          ) : null}
          <LogsTable />
        </Card.Content>
      </Card>
    </div>
  );
};

export default Log;
