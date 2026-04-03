import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Form, Grid, Header, Table } from 'semantic-ui-react';
import {
  API,
  showError,
  showSuccess,
  timestamp2string,
} from '../helpers';

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
    <Grid columns={1}>
      <Grid.Column>
        <Form>
          <Header as='h3' className='router-section-title'>
            {t('setting.currency.catalog.title')}
          </Header>
          <div className='router-settings-note'>
            {t('setting.currency.catalog.subtitle')}
          </div>
          <div className='router-toolbar router-block-gap-sm'>
            <div className='router-toolbar-start'>
              <Button
                className='router-page-button'
                type='button'
                onClick={addCurrency}
                disabled={loading || currencies.some((item) => item._isNew)}
              >
                {t('setting.currency.catalog.buttons.add')}
              </Button>
            </div>
          </div>
          <div className='router-table-scroll-x'>
            <Table compact celled className='router-detail-table'>
              <Table.Header>
                <Table.Row>
                  <Table.HeaderCell collapsing style={codeColumnStyle}>
                    {t('setting.currency.catalog.columns.code')}
                  </Table.HeaderCell>
                  <Table.HeaderCell style={nameColumnStyle}>
                    {t('setting.currency.catalog.columns.name')}
                  </Table.HeaderCell>
                  <Table.HeaderCell collapsing style={symbolColumnStyle}>
                    {t('setting.currency.catalog.columns.symbol')}
                  </Table.HeaderCell>
                  <Table.HeaderCell collapsing>
                    {t('setting.currency.catalog.columns.minor_unit')}
                  </Table.HeaderCell>
                  <Table.HeaderCell collapsing>
                    {t('setting.currency.catalog.columns.status')}
                  </Table.HeaderCell>
                  <Table.HeaderCell collapsing style={createdAtColumnStyle}>
                    {t('setting.currency.catalog.columns.created_at')}
                  </Table.HeaderCell>
                  <Table.HeaderCell collapsing style={updatedAtColumnStyle}>
                    {t('setting.currency.catalog.columns.updated_at')}
                  </Table.HeaderCell>
                  <Table.HeaderCell collapsing style={actionColumnStyle}>
                    {t('setting.currency.catalog.columns.action')}
                  </Table.HeaderCell>
                </Table.Row>
              </Table.Header>
              <Table.Body>
                {loading ? (
                  <Table.Row>
                    <Table.Cell colSpan={8} textAlign='center' className='router-empty-cell'>
                      {t('common.loading')}
                    </Table.Cell>
                  </Table.Row>
                ) : currencies.length === 0 ? (
                  <Table.Row>
                    <Table.Cell colSpan={8} textAlign='center' className='router-empty-cell'>
                      {t('setting.currency.catalog.empty')}
                    </Table.Cell>
                  </Table.Row>
                ) : (
                  currencies.map((row, index) => {
                    const currentSavingKey = row._isNew ? `new-${index}` : row.code;
                    const isSaving = savingKey === currentSavingKey;
                    return (
                      <Table.Row key={row.code || `new-${index}`}>
                        <Table.Cell style={codeColumnStyle}>
                          <Form.Input
                            className='router-section-input'
                            transparent
                            value={row.code || ''}
                            onChange={(e, { value }) =>
                              updateCurrencyField(index, 'code', value)
                            }
                            readOnly={!row._isNew}
                            placeholder='USD'
                            style={codeColumnStyle}
                          />
                        </Table.Cell>
                        <Table.Cell style={nameColumnStyle}>
                          <Form.Input
                            className='router-section-input'
                            transparent
                            value={row.name || ''}
                            onChange={(e, { value }) =>
                              updateCurrencyField(index, 'name', value)
                            }
                            placeholder={t('setting.currency.catalog.placeholders.name')}
                            style={nameColumnStyle}
                          />
                        </Table.Cell>
                        <Table.Cell style={symbolColumnStyle}>
                          <Form.Input
                            className='router-section-input'
                            transparent
                            value={row.symbol || ''}
                            onChange={(e, { value }) =>
                              updateCurrencyField(index, 'symbol', value)
                            }
                            placeholder='$'
                            style={symbolColumnStyle}
                          />
                        </Table.Cell>
                        <Table.Cell>
                          <Form.Input
                            className='router-section-input'
                            transparent
                            type='number'
                            min='0'
                            max='8'
                            step='1'
                            value={row.minor_unit}
                            onChange={(e, { value }) =>
                              updateCurrencyField(index, 'minor_unit', value)
                            }
                          />
                        </Table.Cell>
                        <Table.Cell>
                          <Form.Dropdown
                            className='router-section-input'
                            compact
                            selection
                            options={statusOptions}
                            value={Number(row.status || 1)}
                            onChange={(e, { value }) =>
                              updateCurrencyField(index, 'status', value)
                            }
                          />
                        </Table.Cell>
                        <Table.Cell style={createdAtColumnStyle}>
                          {row.created_at ? timestamp2string(row.created_at) : '-'}
                        </Table.Cell>
                        <Table.Cell style={updatedAtColumnStyle}>
                          {row.updated_at ? timestamp2string(row.updated_at) : '-'}
                        </Table.Cell>
                        <Table.Cell style={actionColumnStyle}>
                          <div className='router-action-group'>
                            {row._isNew ? (
                              <Button
                                className='router-table-action-button'
                                type='button'
                                onClick={() => removeNewCurrency(index)}
                                disabled={isSaving}
                              >
                                {t('setting.currency.catalog.buttons.cancel')}
                              </Button>
                            ) : null}
                              <Button
                                className='router-table-action-button'
                                primary
                                type='button'
                                loading={isSaving}
                                disabled={isSaving}
                                onClick={() => saveCurrency(row, index)}
                              >
                                {t('setting.currency.catalog.buttons.save')}
                              </Button>
                            </div>
                          </Table.Cell>
                      </Table.Row>
                    );
                  })
                )}
              </Table.Body>
            </Table>
          </div>
        </Form>
      </Grid.Column>
    </Grid>
  );
};

export default CurrencySetting;
