import React from 'react';
import { Card } from 'semantic-ui-react';
import ModelProvidersManager from '../../components/ModelProvidersManager';

const ModelProviders = () => {
  return (
    <div className='dashboard-container'>
      <Card fluid className='chart-card'>
        <Card.Content>
          <ModelProvidersManager />
        </Card.Content>
      </Card>
    </div>
  );
};

export default ModelProviders;
