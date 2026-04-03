import React from 'react';
import { Card } from 'semantic-ui-react';
import BusinessFlowTable from '../../components/BusinessFlowTable';

const FlowPage = ({ kind }) => {
  return (
    <div className='dashboard-container'>
      <Card fluid className='chart-card'>
        <Card.Content>
          <BusinessFlowTable kind={kind} />
        </Card.Content>
      </Card>
    </div>
  );
};

export default FlowPage;
