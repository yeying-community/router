import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Form, Grid, Header, Modal, Table } from 'semantic-ui-react';
import {
  API,
  showError,
  showSuccess,
  timestamp2string,
} from '../helpers';
import { formatDecimalNumber } from '../helpers/render';

const YYC_CODE = 'YYC';
const MANUAL_RATE_ROW_PREFIX = 'manual:';
const MARKET_RATE_ROW_PREFIX = 'market:';
const rateColumnStyle = { width: '180px', minWidth: '180px' };
const rateDateColumnStyle = { width: '160px', minWidth: '160px' };
const createdAtColumnStyle = { width: '160px', minWidth: '160px' };
const updatedAtColumnStyle = { width: '168px', minWidth: '168px' };

const ExchangeRateSetting = ({ section = '' }) => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [savingKey, setSavingKey] = useState('');
  const [syncing, setSyncing] = useState(false);
  const [rates, setRates] = useState([]);
  const [editingRow, setEditingRow] = useState(null);
  const [editingRate, setEditingRate] = useState('');

  const normalizedSection = (section || '').trim().toLowerCase();
  const sectionVisible =
    normalizedSection === '' ||
    normalizedSection === 'all' ||
    normalizedSection === 'rates';

  const loadRates = async () => {
    setLoading(true);
    try {
      const [currencyRes, fxRes] = await Promise.all([
        API.get('/api/v1/admin/billing/currencies'),
        API.get('/api/v1/admin/billing/fx/rates'),
      ]);
      const currencyPayload = currencyRes.data || {};
      const fxPayload = fxRes.data || {};
      if (!currencyPayload.success) {
        showError(
          currencyPayload.message || t('setting.exchange.messages.load_failed'),
        );
        return;
      }
      if (!fxPayload.success) {
        showError(fxPayload.message || t('setting.exchange.messages.load_failed'));
        return;
      }

      const currencies = Array.isArray(currencyPayload.data)
        ? currencyPayload.data
        : [];
      const marketData = fxPayload.data || {};
      const marketItems = Array.isArray(marketData.items) ? marketData.items : [];

      const manualRows = currencies
        .map((item) => ({
          ...item,
          code: (item?.code || '').toString().trim().toUpperCase(),
        }))
        .filter((item) => item.code && item.code !== YYC_CODE)
        .map((item) => ({
          id: `${MANUAL_RATE_ROW_PREFIX}${item.code}`,
          pair: `${item.code}/${YYC_CODE}`,
          base: item.code,
          quote: YYC_CODE,
          rate:
            item?.yyc_per_unit === 0 || item?.yyc_per_unit
              ? `${item.yyc_per_unit}`
              : '0',
          provider: t('setting.exchange.providers.manual'),
          rate_date: '',
          created_at: Number(item?.created_at || 0),
          updated_at: Number(item?.updated_at || 0),
          row_type: 'manual',
          currency: {
            ...item,
            minor_unit: Number(item?.minor_unit ?? 6),
            status: Number(item?.status || 1),
          },
        }))
        .sort((a, b) => a.pair.localeCompare(b.pair));

      const marketRows = marketItems
        .map((item) => ({
          id: `${MARKET_RATE_ROW_PREFIX}${item?.pair || `${item?.base || ''}/${item?.quote || ''}`}`,
          pair: (item?.pair || '').toString().trim(),
          base: (item?.base || '').toString().trim().toUpperCase(),
          quote: (item?.quote || '').toString().trim().toUpperCase(),
          rate: Number(item?.rate || 0),
          provider:
            (item?.provider || marketData.provider || '').toString().trim(),
          rate_date:
            (item?.rate_date || marketData.date || '').toString().trim(),
          created_at: Number(item?.created_at || 0),
          updated_at: Number(item?.updated_at || 0),
          row_type: 'market',
        }))
        .sort((a, b) => a.pair.localeCompare(b.pair));

      setRates([...manualRows, ...marketRows]);
    } catch (error) {
      showError(error?.message || t('setting.exchange.messages.load_failed'));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!sectionVisible) {
      return;
    }
    loadRates().then();
  }, [sectionVisible]);

  const closeEditModal = () => {
    if (savingKey) {
      return;
    }
    setEditingRow(null);
    setEditingRate('');
  };

  const openEditModal = (row) => {
    setEditingRow(row);
    setEditingRate(`${row?.rate ?? ''}`);
  };

  const saveManualRate = async () => {
    const row = editingRow;
    const rate = Number.parseFloat(editingRate ?? '');
    if (!Number.isFinite(rate) || rate <= 0) {
      showError(t('setting.exchange.messages.rate_invalid'));
      return;
    }
    const currency = row?.currency || {};
    const code = (currency?.code || row?.base || '').toString().trim().toUpperCase();
    if (!code) {
      showError(t('setting.exchange.messages.save_failed'));
      return;
    }

    setSavingKey(row.id || code);
    try {
      const res = await API.put(
        `/api/v1/admin/billing/currencies/${encodeURIComponent(code)}`,
        {
          code,
          name: currency?.name,
          symbol: currency?.symbol,
          minor_unit: Number(currency?.minor_unit ?? 6),
          yyc_per_unit: rate,
          status: Number(currency?.status || 1),
          source: (currency?.source || 'manual').toString().trim() || 'manual',
        },
      );
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('setting.exchange.messages.save_failed'));
        return;
      }
      showSuccess(t('setting.exchange.messages.save_success'));
      closeEditModal();
      await loadRates();
    } catch (error) {
      showError(error?.message || t('setting.exchange.messages.save_failed'));
    } finally {
      setSavingKey('');
    }
  };

  const syncRates = async () => {
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
        t('setting.exchange.messages.sync_success', {
          count: Number.isFinite(updatedCount) ? updatedCount : 0,
        }),
      );
      await loadRates();
    } catch (error) {
      showError(error?.message || t('setting.exchange.messages.sync_failed'));
    } finally {
      setSyncing(false);
    }
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
        <Form>
          <Header as='h3' className='router-section-title'>
            {t('setting.exchange.title')}
          </Header>
          <div className='router-settings-note'>
            {t('setting.exchange.subtitle')}
          </div>
          <div className='router-table-scroll-x'>
            <Table compact celled className='router-detail-table'>
              <Table.Header>
                <Table.Row>
                  <Table.HeaderCell collapsing>
                    {t('setting.exchange.columns.pair')}
                  </Table.HeaderCell>
                  <Table.HeaderCell>
                    {t('setting.exchange.columns.rate')}
                  </Table.HeaderCell>
                  <Table.HeaderCell collapsing>
                    {t('setting.exchange.columns.provider')}
                  </Table.HeaderCell>
                  <Table.HeaderCell style={rateDateColumnStyle}>
                    {t('setting.exchange.columns.rate_date')}
                  </Table.HeaderCell>
                  <Table.HeaderCell style={createdAtColumnStyle}>
                    {t('setting.exchange.columns.created_at')}
                  </Table.HeaderCell>
                  <Table.HeaderCell style={updatedAtColumnStyle}>
                    {t('setting.exchange.columns.updated_at')}
                  </Table.HeaderCell>
                  <Table.HeaderCell collapsing>
                    {t('setting.exchange.columns.action')}
                  </Table.HeaderCell>
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {loading ? (
                  <Table.Row>
                    <Table.Cell colSpan={7} textAlign='center' className='router-empty-cell'>
                      {t('common.loading')}
                    </Table.Cell>
                  </Table.Row>
                ) : rates.length === 0 ? (
                  <Table.Row>
                    <Table.Cell colSpan={7} textAlign='center' className='router-empty-cell'>
                      {t('setting.exchange.empty')}
                    </Table.Cell>
                  </Table.Row>
                ) : (
                  rates.map((row) => (
                      <Table.Row key={row.id || row.pair || `${row.base}-${row.quote}`}>
                        <Table.Cell>{row.pair || '-'}</Table.Cell>
                        <Table.Cell style={rateColumnStyle}>
                          {t('setting.exchange.rate_display', {
                            base: row.base || '-',
                            value: formatDecimalNumber(row.rate || 0, 6),
                            quote: row.quote || '-',
                          })}
                        </Table.Cell>
                        <Table.Cell>{row.provider || '-'}</Table.Cell>
                        <Table.Cell style={rateDateColumnStyle}>
                          {row.rate_date || '-'}
                        </Table.Cell>
                        <Table.Cell style={createdAtColumnStyle}>
                          {row.created_at ? timestamp2string(row.created_at) : '-'}
                        </Table.Cell>
                        <Table.Cell style={updatedAtColumnStyle}>
                          {row.updated_at ? timestamp2string(row.updated_at) : '-'}
                        </Table.Cell>
                        <Table.Cell>
                          {row.row_type === 'manual' ? (
                            <Button
                              className='router-table-action-button'
                              type='button'
                              primary
                              loading={savingKey === row.id}
                              disabled={syncing || savingKey === row.id}
                              onClick={() => openEditModal(row)}
                            >
                              {t('setting.exchange.buttons.edit')}
                            </Button>
                          ) : (
                            <Button
                              className='router-table-action-button'
                              type='button'
                              loading={syncing}
                              disabled={syncing || savingKey !== ''}
                              onClick={syncRates}
                            >
                              {t('setting.exchange.buttons.sync')}
                            </Button>
                          )}
                        </Table.Cell>
                      </Table.Row>
                    ))
                )}
              </Table.Body>
            </Table>
          </div>
        </Form>
      </Grid.Column>
      <Modal
        size='tiny'
        open={!!editingRow}
        onClose={closeEditModal}
        closeOnDimmerClick={!savingKey}
        closeOnEscape={!savingKey}
      >
        <Modal.Header>
          {t('setting.exchange.dialogs.edit_title')}
        </Modal.Header>
        <Modal.Content>
          <Form>
            <Form.Field>
              <label>{t('setting.exchange.columns.pair')}</label>
              <div className='router-readonly-value'>
                {editingRow?.pair || '-'}
              </div>
            </Form.Field>
            <Form.Input
              className='router-section-input'
              label={t('setting.exchange.dialogs.rate_label')}
              type='number'
              min='0'
              step='0.000001'
              value={editingRate}
              onChange={(e, { value }) => setEditingRate(value)}
              placeholder='0.000000'
              autoFocus
            />
          </Form>
        </Modal.Content>
        <Modal.Actions>
          <Button type='button' onClick={closeEditModal} disabled={!!savingKey}>
            {t('common.cancel')}
          </Button>
          <Button
            primary
            type='button'
            loading={!!savingKey}
            disabled={!!savingKey}
            onClick={saveManualRate}
          >
            {t('common.confirm')}
          </Button>
        </Modal.Actions>
      </Modal>
    </Grid>
  );
};

export default ExchangeRateSetting;
