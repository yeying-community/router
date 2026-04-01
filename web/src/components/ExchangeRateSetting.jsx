import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Form, Grid, Header, Table } from 'semantic-ui-react';
import {
  API,
  showError,
  showSuccess,
  timestamp2string,
} from '../helpers';

const defaultInputs = {
  FXAutoSyncEnabled: 'false',
  FXAutoSyncIntervalSeconds: '21600',
  FXAutoSyncProvider: 'frankfurter',
};

const ExchangeRateSetting = ({ section = '' }) => {
  const { t } = useTranslation();
  const [inputs, setInputs] = useState(defaultInputs);
  const [originInputs, setOriginInputs] = useState(defaultInputs);
  const [loading, setLoading] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [status, setStatus] = useState({
    last_run_at: 0,
    last_success_at: 0,
    last_error: '',
    min_interval: 60,
  });
  const [ratesLoading, setRatesLoading] = useState(false);
  const [rateRows, setRateRows] = useState([]);
  const [rateMeta, setRateMeta] = useState({
    date: '',
    provider: 'frankfurter',
    currencies: [],
  });

  const normalizedSection = (section || '').trim().toLowerCase();
  const sectionVisible =
    normalizedSection === '' ||
    normalizedSection === 'all' ||
    normalizedSection === 'sync';

  const getOptions = async () => {
    const res = await API.get('/api/v1/admin/option/');
    const { success, message, data } = res.data || {};
    if (!success) {
      showError(message);
      return;
    }
    const optionMap = {};
    (Array.isArray(data) ? data : []).forEach((item) => {
      optionMap[item.key] = item.value;
    });
    const next = {
      FXAutoSyncEnabled:
        `${optionMap.FXAutoSyncEnabled ?? defaultInputs.FXAutoSyncEnabled}`,
      FXAutoSyncIntervalSeconds: `${
        optionMap.FXAutoSyncIntervalSeconds ??
        defaultInputs.FXAutoSyncIntervalSeconds
      }`,
      FXAutoSyncProvider:
        (optionMap.FXAutoSyncProvider || defaultInputs.FXAutoSyncProvider)
          .toString()
          .trim() || defaultInputs.FXAutoSyncProvider,
    };
    setInputs(next);
    setOriginInputs(next);
  };

  const loadStatus = async () => {
    try {
      const res = await API.get('/api/v1/admin/billing/fx/status');
      const { success, data } = res.data || {};
      if (!success || !data) {
        return;
      }
      setStatus({
        last_run_at: Number(data.last_run_at || 0),
        last_success_at: Number(data.last_success_at || 0),
        last_error: (data.last_error || '').toString(),
        min_interval: Number(data.min_interval || 60),
      });
    } catch (error) {
      // keep page usable even when status fetch fails
    }
  };

  useEffect(() => {
    getOptions().then();
    loadStatus().then();
    loadRates().then();
  }, []);

  const loadRates = async () => {
    setRatesLoading(true);
    try {
      const res = await API.get('/api/v1/admin/billing/fx/rates');
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('setting.exchange.messages.rates_load_failed'));
        return;
      }
      setRateRows(Array.isArray(data?.items) ? data.items : []);
      setRateMeta({
        date: (data?.date || '').toString(),
        provider: (data?.provider || 'frankfurter').toString(),
        currencies: Array.isArray(data?.currencies) ? data.currencies : [],
      });
    } catch (error) {
      showError(error?.message || t('setting.exchange.messages.rates_load_failed'));
    } finally {
      setRatesLoading(false);
    }
  };

  const updateOption = async (key, value) => {
    setLoading(true);
    let nextValue = value;
    if (key.endsWith('Enabled')) {
      nextValue = inputs[key] === 'true' ? 'false' : 'true';
    }
    const res = await API.put('/api/v1/admin/option/', {
      key,
      value: nextValue,
    });
    const { success, message } = res.data || {};
    if (!success) {
      showError(message);
      setLoading(false);
      return false;
    }
    setInputs((previous) => ({ ...previous, [key]: nextValue }));
    setOriginInputs((previous) => ({ ...previous, [key]: nextValue }));
    setLoading(false);
    return true;
  };

  const handleInputChange = async (e, { name, value }) => {
    const normalizedValue = value ?? '';
    if (name.endsWith('Enabled')) {
      await updateOption(name, normalizedValue);
      await loadStatus();
      return;
    }
    setInputs((previous) => ({ ...previous, [name]: normalizedValue }));
  };

  const submitConfig = async () => {
    const minInterval = status.min_interval || 60;
    const intervalSeconds = Number.parseInt(
      inputs.FXAutoSyncIntervalSeconds ?? '',
      10,
    );
    if (!Number.isFinite(intervalSeconds) || intervalSeconds < minInterval) {
      showError(
        t('setting.exchange.messages.interval_invalid', {
          min: minInterval,
        }),
      );
      return;
    }

    if (
      `${originInputs.FXAutoSyncIntervalSeconds || ''}` !== `${intervalSeconds}`
    ) {
      const ok = await updateOption(
        'FXAutoSyncIntervalSeconds',
        `${intervalSeconds}`,
      );
      if (!ok) {
        return;
      }
    }
    if (`${originInputs.FXAutoSyncProvider || ''}` !== 'frankfurter') {
      const ok = await updateOption('FXAutoSyncProvider', 'frankfurter');
      if (!ok) {
        return;
      }
    }
    showSuccess(t('setting.exchange.messages.save_success'));
    await getOptions();
    await loadStatus();
  };

  const syncNow = async () => {
    setSyncing(true);
    try {
      const res = await API.post('/api/v1/admin/billing/fx/sync');
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('setting.exchange.messages.sync_failed'));
        return;
      }
      const updatedCount = Number(data?.updated_count || 0);
      showSuccess(
        t('setting.exchange.messages.sync_success', { count: updatedCount }),
      );
      await loadStatus();
      await getOptions();
      await loadRates();
    } catch (error) {
      showError(error?.message || t('setting.exchange.messages.sync_failed'));
    } finally {
      setSyncing(false);
    }
  };

  const renderTimestamp = (value) => {
    const num = Number(value || 0);
    if (!Number.isFinite(num) || num <= 0) {
      return '-';
    }
    return timestamp2string(num);
  };

  const renderRateValue = (value) => {
    const num = Number(value);
    if (!Number.isFinite(num) || num <= 0) {
      return '-';
    }
    if (num >= 100) {
      return num.toFixed(4);
    }
    if (num >= 1) {
      return num.toFixed(6);
    }
    return num.toFixed(8);
  };

  if (!sectionVisible) {
    return (
      <div className='router-empty-cell'>
        {t('setting.empty_admin', '暂无可配置项')}
      </div>
    );
  }

  return (
    <Grid columns={1}>
      <Grid.Column>
        <Form loading={loading}>
          <Header as='h3' className='router-section-title'>
            {t('setting.exchange.title')}
          </Header>
          <div className='router-settings-note'>
            {t('setting.exchange.subtitle')}
          </div>
          <Form.Group widths='equal'>
            <Form.Checkbox
              className='router-section-checkbox'
              checked={inputs.FXAutoSyncEnabled === 'true'}
              label={t('setting.exchange.auto_sync.enabled')}
              name='FXAutoSyncEnabled'
              onChange={handleInputChange}
            />
            <Form.Input
              className='router-section-input'
              label={t('setting.exchange.auto_sync.interval_seconds')}
              name='FXAutoSyncIntervalSeconds'
              onChange={handleInputChange}
              autoComplete='new-password'
              value={inputs.FXAutoSyncIntervalSeconds}
              type='number'
              min={status.min_interval || 60}
              step='1'
              placeholder='21600'
            />
            <Form.Input
              className='router-section-input'
              label={t('setting.exchange.auto_sync.provider')}
              name='FXAutoSyncProvider'
              value='frankfurter'
              readOnly
            />
          </Form.Group>
          <Form.Button
            className='router-section-button'
            onClick={() => {
              submitConfig().then();
            }}
          >
            {t('setting.exchange.buttons.save')}
          </Form.Button>
          <div className='router-settings-note'>
            {t('setting.exchange.auto_sync.last_run', {
              value: renderTimestamp(status.last_run_at),
            })}
          </div>
          <div className='router-settings-note'>
            {t('setting.exchange.auto_sync.last_success', {
              value: renderTimestamp(status.last_success_at),
            })}
          </div>
          {status.last_error ? (
            <div className='router-settings-note'>
              {t('setting.exchange.auto_sync.last_error', {
                value: status.last_error,
              })}
            </div>
          ) : null}
          <div className='router-toolbar router-block-gap-sm'>
            <div className='router-toolbar-start'>
              <Button
                className='router-page-button'
                type='button'
                onClick={syncNow}
                loading={syncing}
                disabled={syncing}
              >
                {t('setting.exchange.buttons.sync_now')}
              </Button>
              <Button
                className='router-page-button'
                type='button'
                onClick={() => {
                  loadRates().then();
                }}
                loading={ratesLoading}
                disabled={ratesLoading || syncing}
              >
                {t('setting.exchange.buttons.refresh_rates')}
              </Button>
            </div>
          </div>
          <Header as='h3' className='router-section-title'>
            {t('setting.exchange.rates.title')}
          </Header>
          <div className='router-settings-note'>
            {t('setting.exchange.rates.subtitle')}
          </div>
          <div className='router-settings-note'>
            {t('setting.exchange.rates.currencies', {
              value:
                Array.isArray(rateMeta.currencies) && rateMeta.currencies.length > 0
                  ? rateMeta.currencies.join(', ')
                  : '-',
            })}
          </div>
          <div className='router-table-scroll-x'>
            <Table compact celled className='router-detail-table'>
              <Table.Header>
                <Table.Row>
                  <Table.HeaderCell>
                    {t('setting.exchange.rates.columns.pair')}
                  </Table.HeaderCell>
                  <Table.HeaderCell>
                    {t('setting.exchange.rates.columns.rate')}
                  </Table.HeaderCell>
                  <Table.HeaderCell>
                    {t('setting.exchange.rates.columns.date')}
                  </Table.HeaderCell>
                  <Table.HeaderCell>
                    {t('setting.exchange.rates.columns.provider')}
                  </Table.HeaderCell>
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {ratesLoading ? (
                  <Table.Row>
                    <Table.Cell colSpan={4} textAlign='center' className='router-empty-cell'>
                      {t('common.loading')}
                    </Table.Cell>
                  </Table.Row>
                ) : rateRows.length === 0 ? (
                  <Table.Row>
                    <Table.Cell colSpan={4} textAlign='center' className='router-empty-cell'>
                      {t('setting.exchange.rates.empty')}
                    </Table.Cell>
                  </Table.Row>
                ) : (
                  rateRows.map((item, index) => (
                    <Table.Row key={`${item.pair || ''}-${index}`}>
                      <Table.Cell>{item.pair || '-'}</Table.Cell>
                      <Table.Cell>{renderRateValue(item.rate)}</Table.Cell>
                      <Table.Cell>{rateMeta.date || '-'}</Table.Cell>
                      <Table.Cell>{rateMeta.provider || '-'}</Table.Cell>
                    </Table.Row>
                  ))
                )}
              </Table.Body>
            </Table>
          </div>
        </Form>
      </Grid.Column>
    </Grid>
  );
};

export default ExchangeRateSetting;
