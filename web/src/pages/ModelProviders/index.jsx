import React, { useRef } from 'react';
import { Button, Card } from 'semantic-ui-react';
import { useTranslation } from 'react-i18next';
import ModelProvidersManager from '../../components/ModelProvidersManager';

const ModelProviders = () => {
  const { t } = useTranslation();
  const managerRef = useRef(null);

  return (
    <div className='dashboard-container'>
      <Card fluid className='chart-card'>
        <Card.Content>
          <Card.Header
            className='header'
            style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}
          >
            <span>{t('channel.providers.title')}</span>
            <Button
              type='button'
              onClick={() => managerRef.current?.openCreateModal()}
            >
              {t('channel.providers.buttons.add_provider')}
            </Button>
          </Card.Header>
          <ModelProvidersManager ref={managerRef} />
        </Card.Content>
      </Card>
    </div>
  );
};

export default ModelProviders;
