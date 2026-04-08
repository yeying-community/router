import React from 'react';
import { Card } from 'semantic-ui-react';
import TopupPlansManager from '../../components/TopupPlansManager';

const AdminTopup = () => {
  return (
    <div className='dashboard-container'>
      <Card fluid className='chart-card'>
        <Card.Content>
          <TopupPlansManager />
        </Card.Content>
      </Card>
    </div>
  );
};

export default AdminTopup;
