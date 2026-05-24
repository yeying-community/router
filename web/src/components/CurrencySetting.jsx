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
  AppSelect,
  AppSpin,
  AppTable,
} from '../router-ui';

const createEmptyCurrency = () => ({
  code: '',
  name: '',
  symbol: '',
  minor_unit: 6,
  yyc_per_unit: '0',
  status: 2,
  source: 'manual',
  updated_at: 0,
  _isNew: true,
});

const codeColumnStyle = { width: '34px', minWidth: '34px' };
const nameColumnStyle = { width: '120px', minWidth: '120px' };
const symbolColumnStyle = { width: '22px', minWidth: '22px' };
const createdAtColumnStyle = { width: '160px', minWidth: '160px' };
const updatedAtColumnStyle = { width: '160px', minWidth: '160px' };
const actionColumnStyle = { width: '120px', minWidth: '120px' };

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

  const statusOptions = [
    {
      key: 1,
      value: 1,
      text: t('setting.currency.catalog.status.enabled'),
    },
    {
      key: 2,
      value: 2,
      text: t('setting.currency.catalog.status.disabled'),
    },
  ];

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
          yyc_per_unit:
            item?.yyc_per_unit === 0 || item?.yyc_per_unit
              ? `${item.yyc_per_unit}`
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
    const yycPerUnit = Number.parseFloat(row.yyc_per_unit ?? '0');

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
    if (status === 1 && (!Number.isFinite(yycPerUnit) || yycPerUnit <= 0)) {
      showError(t('setting.currency.catalog.messages.enabled_rate_required'));
      return;
    }

    const payload = {
      code,
      name,
      symbol,
      minor_unit: minorUnit,
      yyc_per_unit: Number.isFinite(yycPerUnit) && yycPerUnit > 0 ? yycPerUnit : 0,
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
            className='router-detail-table'
            dataSource={currencies.map((row, index) => ({
              ...row,
              _rowKey: row.code || `new-${index}`,
              _rowIndex: index,
            }))}
            rowKey='_rowKey'
            pagination={false}
            scroll={{ x: 980 }}
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
                width: codeColumnStyle.width,
                render: (_, row) => (
                  <AppInput
                    className='router-section-input'
                    value={row.code || ''}
                    onChange={(e, { value }) =>
                      updateCurrencyField(row._rowIndex, 'code', value)
                    }
                    readOnly={!row._isNew}
                    placeholder='USD'
                    style={codeColumnStyle}
                  />
                ),
              },
              {
                title: t('setting.currency.catalog.columns.name'),
                dataIndex: 'name',
                key: 'name',
                width: nameColumnStyle.width,
                render: (_, row) => (
                  <AppInput
                    className='router-section-input'
                    value={row.name || ''}
                    onChange={(e, { value }) =>
                      updateCurrencyField(row._rowIndex, 'name', value)
                    }
                    placeholder={t('setting.currency.catalog.placeholders.name')}
                    style={nameColumnStyle}
                  />
                ),
              },
              {
                title: t('setting.currency.catalog.columns.symbol'),
                dataIndex: 'symbol',
                key: 'symbol',
                width: symbolColumnStyle.width,
                render: (_, row) => (
                  <AppInput
                    className='router-section-input'
                    value={row.symbol || ''}
                    onChange={(e, { value }) =>
                      updateCurrencyField(row._rowIndex, 'symbol', value)
                    }
                    placeholder='$'
                    style={symbolColumnStyle}
                  />
                ),
              },
              {
                title: t('setting.currency.catalog.columns.minor_unit'),
                dataIndex: 'minor_unit',
                key: 'minor_unit',
                width: 120,
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
                width: 140,
                render: (_, row) => (
                  <AppSelect
                    className='router-section-input'
                    options={statusOptions}
                    value={Number(row.status || 1)}
                    onChange={(e, { value }) =>
                      updateCurrencyField(row._rowIndex, 'status', value)
                    }
                  />
                ),
              },
              {
                title: t('setting.currency.catalog.columns.created_at'),
                dataIndex: 'created_at',
                key: 'created_at',
                width: createdAtColumnStyle.width,
                sorter: (a, b) => Number(a.created_at || 0) - Number(b.created_at || 0),
                defaultSortOrder: 'descend',
                render: (value) => (value ? timestamp2string(value) : '-'),
              },
              {
                title: t('setting.currency.catalog.columns.updated_at'),
                dataIndex: 'updated_at',
                key: 'updated_at',
                width: updatedAtColumnStyle.width,
                sorter: (a, b) => Number(a.updated_at || 0) - Number(b.updated_at || 0),
                render: (value) => (value ? timestamp2string(value) : '-'),
              },
              {
                title: t('setting.currency.catalog.columns.action'),
                key: 'action',
                width: actionColumnStyle.width,
                render: (_, row) => {
                  const currentSavingKey = row._isNew
                    ? `new-${row._rowIndex}`
                    : row.code;
                  const isSaving = savingKey === currentSavingKey;
                  return (
                    <div className='router-action-group'>
                      {row._isNew ? (
                        <AppButton
                          className='router-table-action-button'
                          type='button'
                          onClick={() => removeNewCurrency(row._rowIndex)}
                          disabled={isSaving}
                        >
                          {t('setting.currency.catalog.buttons.cancel')}
                        </AppButton>
                      ) : null}
                      <AppButton
                        className='router-table-action-button'
                        color='blue'
                        type='button'
                        loading={isSaving}
                        disabled={isSaving}
                        onClick={() => saveCurrency(row, row._rowIndex)}
                      >
                        {t('setting.currency.catalog.buttons.save')}
                      </AppButton>
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
