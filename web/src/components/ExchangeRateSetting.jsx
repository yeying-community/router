import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  API,
  showError,
  showSuccess,
  timestamp2string,
} from '../helpers';
import { formatDecimalNumber } from '../helpers/render';
import {
  AppButton,
  AppField,
  AppFilterHeader,
  AppFormActions,
  AppFormRow,
  AppInputNumber,
  AppModal,
  AppSpin,
  AppTable,
  AppTableActionButton,
} from '../router-ui';

const YYC_CODE = 'YYC';
const MANUAL_RATE_ROW_PREFIX = 'manual:';
const MARKET_RATE_ROW_PREFIX = 'market:';
const pairColumnStyle = { width: 112 };
const rateColumnStyle = { width: 240 };
const providerColumnStyle = { width: 132 };
const rateDateColumnStyle = { width: '160px', minWidth: '160px' };
const createdAtColumnStyle = { width: '160px', minWidth: '160px' };
const updatedAtColumnStyle = { width: '168px', minWidth: '168px' };
const actionColumnStyle = { width: 76 };

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
            item?.charge_rate === 0 || item?.charge_rate
              ? `${item.charge_rate}`
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
          charge_rate: rate,
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
    <AppSpin spinning={loading}>
      <div>
        <AppFilterHeader
          title={t('setting.exchange.title')}
          titleClassName='router-ui-section-title'
          className='router-toolbar-compact'
        />
        <div className='router-settings-note'>
          {t('setting.exchange.subtitle')}
        </div>
        <div className='router-table-scroll-x'>
          <AppTable
            className='router-detail-table'
            pagination={false}
            rowKey={(row) => row.id || row.pair || `${row.base}-${row.quote}`}
            dataSource={rates}
            locale={{
              emptyText: loading ? t('common.loading') : t('setting.exchange.empty'),
            }}
            columns={[
              {
                title: t('setting.exchange.columns.pair'),
                dataIndex: 'pair',
                key: 'pair',
                width: pairColumnStyle.width,
                render: (value) => value || '-',
              },
              {
                title: t('setting.exchange.columns.rate'),
                key: 'rate',
                width: rateColumnStyle.width,
                render: (_, row) =>
                  t('setting.exchange.rate_display', {
                    base: row.base || '-',
                    value: formatDecimalNumber(row.rate || 0, 6),
                    quote: row.quote || '-',
                  }),
              },
              {
                title: t('setting.exchange.columns.provider'),
                dataIndex: 'provider',
                key: 'provider',
                width: providerColumnStyle.width,
                render: (value) => value || '-',
              },
              {
                title: t('setting.exchange.columns.rate_date'),
                dataIndex: 'rate_date',
                key: 'rate_date',
                width: rateDateColumnStyle.width,
                render: (value) => value || '-',
              },
              {
                title: t('setting.exchange.columns.created_at'),
                dataIndex: 'created_at',
                key: 'created_at',
                width: createdAtColumnStyle.width,
                sorter: (a, b) => Number(a.created_at || 0) - Number(b.created_at || 0),
                defaultSortOrder: 'descend',
                render: (value) => (value ? timestamp2string(value) : '-'),
              },
              {
                title: t('setting.exchange.columns.updated_at'),
                dataIndex: 'updated_at',
                key: 'updated_at',
                width: updatedAtColumnStyle.width,
                sorter: (a, b) => Number(a.updated_at || 0) - Number(b.updated_at || 0),
                render: (value) => (value ? timestamp2string(value) : '-'),
              },
              {
                title: t('setting.exchange.columns.action'),
                key: 'action',
                className: 'router-table-col-actions-icon',
                width: actionColumnStyle.width,
                render: (_, row) =>
                  row.row_type === 'manual' ? (
                    <div className='router-action-group-tight router-table-actions-icon-compact'>
                      <AppTableActionButton
                        icon='edit'
                        title={t('setting.exchange.buttons.edit')}
                        color='blue'
                        loading={savingKey === row.id}
                        disabled={syncing || savingKey === row.id}
                        onClick={() => openEditModal(row)}
                      />
                    </div>
                  ) : (
                    <div className='router-action-group-tight router-table-actions-icon-compact'>
                      <AppTableActionButton
                        icon='sync'
                        title={t('setting.exchange.buttons.sync')}
                        loading={syncing}
                        disabled={syncing || savingKey !== ''}
                        onClick={syncRates}
                      />
                    </div>
                  ),
              },
            ]}
          />
        </div>
      </div>
      <AppModal
        size='tiny'
        open={!!editingRow}
        onClose={closeEditModal}
        closeOnDimmerClick={!savingKey}
        title={t('setting.exchange.dialogs.edit_title')}
        footer={null}
      >
        <div className='router-page-stack'>
          <AppFormRow className='router-modal-form-row'>
            <AppField label={t('setting.exchange.columns.pair')} readOnly>
              <div className='router-readonly-value'>
                {editingRow?.pair || '-'}
              </div>
            </AppField>
          </AppFormRow>
          <AppFormRow className='router-modal-form-row'>
            <AppField label={t('setting.exchange.dialogs.rate_label')}>
              <AppInputNumber
                className='router-section-input'
                min={0}
                step={0.000001}
                precision={6}
                fluid
                value={editingRate}
                onChange={(_, { value }) => setEditingRate(value)}
                placeholder='0.000000'
                autoFocus
              />
            </AppField>
          </AppFormRow>
          <AppFormActions>
            <AppButton
              type='button'
              onClick={closeEditModal}
              disabled={!!savingKey}
            >
              {t('common.cancel')}
            </AppButton>
            <AppButton
              color='blue'
              type='button'
              loading={!!savingKey}
              disabled={!!savingKey}
              onClick={saveManualRate}
            >
              {t('common.confirm')}
            </AppButton>
          </AppFormActions>
        </div>
      </AppModal>
    </AppSpin>
  );
};

export default ExchangeRateSetting;
