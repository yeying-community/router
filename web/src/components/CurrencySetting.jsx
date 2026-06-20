import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  API,
  showError,
  showSuccess,
  timestamp2string,
} from '../helpers';
import {
  AppButton,
  AppFilterHeader,
  AppInput,
  AppInputNumber,
  AppSpin,
  AppSwitch,
  AppTable,
  AppTableActionButton,
} from '../router-ui';

const createEmptyCurrency = () => ({
  code: '',
  name: '',
  symbol: '',
  minor_unit: 6,
  charge_rate: '0',
  status: 2,
  source: 'manual',
  updated_at: 0,
  _isNew: true,
});

const codeColumnWidth = 96;
const nameColumnWidth = 180;
const symbolColumnWidth = 88;
const minorUnitColumnWidth = 108;
const statusColumnWidth = 92;
const createdAtColumnWidth = 148;
const updatedAtColumnWidth = 148;
const actionColumnWidth = 84;
const currencyTableMinWidth =
  codeColumnWidth +
  nameColumnWidth +
  symbolColumnWidth +
  minorUnitColumnWidth +
  statusColumnWidth +
  createdAtColumnWidth +
  updatedAtColumnWidth +
  actionColumnWidth;

const CurrencySetting = ({ section = '' }) => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(false);
  const [savingKey, setSavingKey] = useState('');
  const [currencies, setCurrencies] = useState([]);

  const normalizedSection = (section || '').trim().toLowerCase();
  const sectionVisible =
    normalizedSection === '' ||
    normalizedSection === 'all' ||
    normalizedSection === 'catalog';

  const loadCurrencies = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/v1/admin/billing/currencies');
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('setting.currency.catalog.messages.load_failed'));
        return;
      }
      const rows = (Array.isArray(data) ? data : [])
        .map((item) => ({
          ...item,
          minor_unit: Number(item?.minor_unit ?? 6),
          charge_rate:
            item?.charge_rate === 0 || item?.charge_rate
              ? `${item.charge_rate}`
              : '0',
          status: Number(item?.status || 1),
          _isNew: false,
        }))
        .sort((a, b) => (a.code || '').localeCompare(b.code || ''));
      setCurrencies(rows);
    } catch (error) {
      showError(
        error?.message || t('setting.currency.catalog.messages.load_failed'),
      );
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!sectionVisible) {
      return;
    }
    loadCurrencies().then();
  }, [sectionVisible]);

  const addCurrency = () => {
    setCurrencies((prev) => {
      if (prev.some((item) => item._isNew)) {
        return prev;
      }
      return [createEmptyCurrency(), ...prev];
    });
  };

  const removeNewCurrency = (index) => {
    setCurrencies((prev) => prev.filter((_, rowIndex) => rowIndex !== index));
  };

  const updateCurrencyField = (index, name, value) => {
    setCurrencies((prev) =>
      prev.map((row, rowIndex) =>
        rowIndex === index ? { ...row, [name]: value } : row,
      ),
    );
  };

  const saveCurrency = async (row, index) => {
    const code = (row.code || '').toString().trim().toUpperCase();
    const name = (row.name || '').toString().trim();
    const symbol = (row.symbol || '').toString().trim();
    const minorUnit = Number.parseInt(row.minor_unit ?? 6, 10);
    const status = Number(row.status || 1);
    const chargeRate = Number.parseFloat(row.charge_rate ?? '0');

    if (!code) {
      showError(t('setting.currency.catalog.messages.code_required'));
      return;
    }
    if (!name) {
      showError(t('setting.currency.catalog.messages.name_required'));
      return;
    }
    if (!Number.isFinite(minorUnit) || minorUnit < 0) {
      showError(t('setting.currency.catalog.messages.minor_unit_invalid'));
      return;
    }
    if (status === 1 && (!Number.isFinite(chargeRate) || chargeRate <= 0)) {
      showError(t('setting.currency.catalog.messages.enabled_rate_required'));
      return;
    }

    const payload = {
      code,
      name,
      symbol,
      minor_unit: minorUnit,
      charge_rate: Number.isFinite(chargeRate) && chargeRate > 0 ? chargeRate : 0,
      status,
      source: (row.source || 'manual').toString().trim() || 'manual',
    };

    const currentSavingKey = row._isNew ? `new-${index}` : code;
    setSavingKey(currentSavingKey);
    try {
      const res = row._isNew
        ? await API.post('/api/v1/admin/billing/currencies', payload)
        : await API.put(
            `/api/v1/admin/billing/currencies/${encodeURIComponent(code)}`,
            payload,
          );
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('setting.currency.catalog.messages.save_failed'));
        return;
      }
      showSuccess(t('setting.currency.catalog.messages.save_success'));
      await loadCurrencies();
    } catch (error) {
      showError(
        error?.message || t('setting.currency.catalog.messages.save_failed'),
      );
    } finally {
      setSavingKey('');
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
          title={t('setting.currency.catalog.title')}
          titleClassName='router-ui-section-title'
          meta={t('setting.currency.catalog.subtitle')}
          className='router-toolbar-compact'
          actions={
            <AppButton
              className='router-page-button'
              type='button'
              onClick={addCurrency}
              disabled={loading || currencies.some((item) => item._isNew)}
            >
              {t('setting.currency.catalog.buttons.add')}
            </AppButton>
          }
        />
        <div className='router-table-scroll-x'>
          <AppTable
            className='router-detail-table router-billing-currency-table'
            dataSource={currencies.map((row, index) => ({
              ...row,
              _rowKey: row.code || `new-${index}`,
              _rowIndex: index,
            }))}
            rowKey='_rowKey'
            pagination={false}
            scroll={{ x: currencyTableMinWidth }}
            locale={{
              emptyText: loading
                ? t('common.loading')
                : t('setting.currency.catalog.empty'),
            }}
            columns={[
              {
                title: t('setting.currency.catalog.columns.code'),
                dataIndex: 'code',
                key: 'code',
                width: codeColumnWidth,
                className: 'router-billing-code-cell',
                render: (_, row) => (
                  <AppInput
                    className='router-section-input router-billing-code-input'
                    value={row.code || ''}
                    onChange={(e, { value }) =>
                      updateCurrencyField(row._rowIndex, 'code', value)
                    }
                    readOnly={!row._isNew}
                    placeholder='USD'
                  />
                ),
              },
              {
                title: t('setting.currency.catalog.columns.name'),
                dataIndex: 'name',
                key: 'name',
                width: nameColumnWidth,
                render: (_, row) => (
                  <AppInput
                    className='router-section-input'
                    value={row.name || ''}
                    onChange={(e, { value }) =>
                      updateCurrencyField(row._rowIndex, 'name', value)
                    }
                    placeholder={t('setting.currency.catalog.placeholders.name')}
                  />
                ),
              },
              {
                title: t('setting.currency.catalog.columns.symbol'),
                dataIndex: 'symbol',
                key: 'symbol',
                width: symbolColumnWidth,
                className: 'router-billing-symbol-cell',
                render: (_, row) => (
                  <AppInput
                    className='router-section-input router-billing-symbol-input'
                    value={row.symbol || ''}
                    onChange={(e, { value }) =>
                      updateCurrencyField(row._rowIndex, 'symbol', value)
                    }
                    placeholder='$'
                  />
                ),
              },
              {
                title: t('setting.currency.catalog.columns.minor_unit'),
                dataIndex: 'minor_unit',
                key: 'minor_unit',
                width: minorUnitColumnWidth,
                render: (_, row) => (
                  <AppInputNumber
                    className='router-section-input'
                    min={0}
                    max={8}
                    step={1}
                    precision={0}
                    fluid
                    value={row.minor_unit}
                    onChange={(e, { value }) =>
                      updateCurrencyField(row._rowIndex, 'minor_unit', value)
                    }
                  />
                ),
              },
              {
                title: t('setting.currency.catalog.columns.status'),
                dataIndex: 'status',
                key: 'status',
                width: statusColumnWidth,
                className: 'router-table-col-status-compact',
                render: (_, row) => {
                  const currentSavingKey = row._isNew
                    ? `new-${row._rowIndex}`
                    : row.code;
                  const isSaving = savingKey === currentSavingKey;
                  return (
                    <AppSwitch
                      checked={Number(row.status || 1) === 1}
                      disabled={isSaving}
                      onChange={(_, { checked }) =>
                        updateCurrencyField(row._rowIndex, 'status', checked ? 1 : 2)
                      }
                    />
                  );
                },
              },
              {
                title: t('setting.currency.catalog.columns.created_at'),
                dataIndex: 'created_at',
                key: 'created_at',
                width: createdAtColumnWidth,
                sorter: (a, b) => Number(a.created_at || 0) - Number(b.created_at || 0),
                defaultSortOrder: 'descend',
                render: (value) => (value ? timestamp2string(value) : '-'),
              },
              {
                title: t('setting.currency.catalog.columns.updated_at'),
                dataIndex: 'updated_at',
                key: 'updated_at',
                width: updatedAtColumnWidth,
                sorter: (a, b) => Number(a.updated_at || 0) - Number(b.updated_at || 0),
                render: (value) => (value ? timestamp2string(value) : '-'),
              },
              {
                title: t('setting.currency.catalog.columns.action'),
                key: 'action',
                width: actionColumnWidth,
                className: 'router-table-col-actions-icon router-billing-action-cell',
                render: (_, row) => {
                  const currentSavingKey = row._isNew
                    ? `new-${row._rowIndex}`
                    : row.code;
                  const isSaving = savingKey === currentSavingKey;
                  return (
                    <div className='router-action-group-tight router-table-actions-icon-compact'>
                      {row._isNew ? (
                        <AppTableActionButton
                          icon='close'
                          title={t('setting.currency.catalog.buttons.cancel')}
                          onClick={() => removeNewCurrency(row._rowIndex)}
                          disabled={isSaving}
                        />
                      ) : null}
                      <AppTableActionButton
                        icon='save'
                        title={t('setting.currency.catalog.buttons.save')}
                        color='blue'
                        loading={isSaving}
                        disabled={isSaving}
                        onClick={() => saveCurrency(row, row._rowIndex)}
                      />
                    </div>
                  );
                },
              },
            ]}
          />
        </div>
      </div>
    </AppSpin>
  );
};

export default CurrencySetting;
