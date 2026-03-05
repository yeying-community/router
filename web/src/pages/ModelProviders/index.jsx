import React, { useRef, useState } from 'react';
import { Button, Card, Form } from 'semantic-ui-react';
import { useTranslation } from 'react-i18next';
import ModelProvidersManager from '../../components/ModelProvidersManager';

const ModelProviders = () => {
  const { t } = useTranslation();
  const managerRef = useRef(null);
  const [searchKeyword, setSearchKeyword] = useState('');

  return (
    <div className='dashboard-container'>
      <Card fluid className='chart-card'>
        <Card.Content>
          <Card.Header
            className='header'
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              gap: '12px',
              flexWrap: 'wrap',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Button type='button' onClick={() => managerRef.current?.openCreateModal()}>
                {t('channel.providers.buttons.add_provider')}
              </Button>
            </div>
            <Form style={{ width: '320px', maxWidth: '100%' }}>
              <Form.Input
                icon='search'
                iconPosition='left'
                placeholder={t('channel.providers.search')}
                value={searchKeyword}
                onChange={(e, { value }) => setSearchKeyword(value || '')}
              />
            </Form>
          </Card.Header>
          <ModelProvidersManager ref={managerRef} searchKeyword={searchKeyword} />
        </Card.Content>
      </Card>
    </div>
  );
};

export default ModelProviders;
