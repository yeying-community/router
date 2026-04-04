import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Button,
  Form,
  Label,
  Modal,
  Pagination,
  Table,
} from 'semantic-ui-react';
import { useTranslation } from 'react-i18next';
import { useSearchParams } from 'react-router-dom';
import { API, showError, showInfo, showSuccess, timestamp2string } from '../helpers';
import { ITEMS_PER_PAGE } from '../constants';
import {
  buildBillingCurrencyIndex,
  buildDisplayUnitOptions,
  buildBillingUnitOptions,
  convertBillingInputValueUnit,
  billingInputValueToYYC,
  yycToBillingInputValue,
  resolveDefaultBillingUnit,
  resolveBillingInputStep,
} from '../helpers/billing';
import UnitDropdown from './UnitDropdown';
import {
  formatDecimalNumber,
} from '../helpers/render';

const createEmptyForm = (defaultBillingUnit = 'USD') => ({
  id: '',
  name: '',
  description: '',
  group_id: '',
  sale_price: '0',
  sale_currency: 'CNY',
  daily_amount: '0',
  daily_amount_unit: defaultBillingUnit,
  emergency_amount: '0',
  emergency_amount_unit: defaultBillingUnit,
  duration_days: 30,
  reset_timezone: 'Asia/Shanghai',
  enabled: true,
  sort_order: 0,
  source: 'manual',
});

const createEmptyAssignForm = () => ({
  user_id: '',
  start_at: '',
});

const statusLabel = (enabled, t) =>
  enabled ? (
    <Label basic color='green' className='router-tag'>
      {t('package_manage.status.enabled')}
    </Label>
  ) : (
    <Label basic color='grey' className='router-tag'>
      {t('package_manage.status.disabled')}
    </Label>
  );

const parseDatetimeLocalValue = (value) => {
  if (typeof value !== 'string' || value.trim() === '') {
    return 0;
  }
  const ts = Date.parse(value.trim());
  if (!Number.isFinite(ts)) {
    return NaN;
  }
  return Math.floor(ts / 1000);
};

const toGroupOptions = (rows) =>
  (Array.isArray(rows) ? rows : []).map((item) => ({
    key: item.id,
    value: item.id,
    text: item.name || item.id,
  }));

const appendGroupOptionIfMissing = (options, groupID, groupName) => {
  const normalizedGroupID = (groupID || '').toString().trim();
  if (!normalizedGroupID) {
    return options;
  }
  const currentOptions = Array.isArray(options) ? options : [];
  if (currentOptions.some((item) => (item?.value || '').toString().trim() === normalizedGroupID)) {
    return currentOptions;
  }
  return [
    ...currentOptions,
    {
      key: normalizedGroupID,
      value: normalizedGroupID,
      text: (groupName || '').toString().trim() || normalizedGroupID,
    },
  ];
};

const toUserOptions = (rows) =>
  (Array.isArray(rows) ? rows : []).map((item) => {
    const id = (item?.id || '').toString().trim();
    const username = (item?.username || '').toString().trim();
    const displayName = (item?.display_name || '').toString().trim();
    const label = username || displayName || id;
    const shortID = id.length > 10 ? `${id.slice(0, 6)}...${id.slice(-4)}` : id;
    return {
      key: id,
      value: id,
      text: `${label} (${shortID})`,
    };
  });

const formatByCurrencyMinorUnit = (amount, currency) => {
  const normalizedAmount = Number(amount || 0);
  if (!Number.isFinite(normalizedAmount)) {
    return '-';
  }
  const minorUnit = Number(currency?.minor_unit);
  const maximumFractionDigits =
    Number.isInteger(minorUnit) && minorUnit >= 0 ? minorUnit : 8;
  const unit = (currency?.code || '').toString().trim().toUpperCase();
  if (unit === 'YYC') {
    return formatDecimalNumber(Math.round(normalizedAmount), 0);
  }
  return formatDecimalNumber(normalizedAmount, maximumFractionDigits);
};

const renderPackageAmountValue = (yycAmount, displayUnit, currencyIndex) => {
  const normalizedYYCAmount = Number(yycAmount || 0);
  if (!Number.isFinite(normalizedYYCAmount)) {
    return '-';
  }
  const targetCurrency = currencyIndex[displayUnit] || currencyIndex.YYC;
  const rate = Number(targetCurrency?.yyc_per_unit || 0);
  if (!Number.isFinite(rate) || rate <= 0) {
    return '-';
  }
  return formatByCurrencyMinorUnit(normalizedYYCAmount / rate, targetCurrency);
};

const resolvePackageYYCAmount = (row, type) => {
  if (type === 'daily') {
    return Number(row?.yyc_daily_limit ?? row?.daily_quota_limit ?? 0);
  }
  return Number(row?.yyc_package_emergency_limit ?? row?.package_emergency_quota_limit ?? 0);
};

const renderPackageAmountFieldValue = (row, type, displayUnit, currencyIndex) => {
  const normalizedYYCAmount = resolvePackageYYCAmount(row, type);
  if (!Number.isFinite(normalizedYYCAmount)) {
    return '-';
  }
  return renderPackageAmountValue(normalizedYYCAmount, displayUnit, currencyIndex);
};

const PackagesManager = () => {
  const { t } = useTranslation();
  const [searchParams] = useSearchParams();
  const [rows, setRows] = useState([]);
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);

  const [searchKeyword, setSearchKeyword] = useState('');
  const [activePage, setActivePage] = useState(1);
  const [totalCount, setTotalCount] = useState(0);

  const [groupOptions, setGroupOptions] = useState([]);
  const [groupLoading, setGroupLoading] = useState(false);
  const [userOptions, setUserOptions] = useState([]);
  const [userLoading, setUserLoading] = useState(false);
  const [displayUnit, setDisplayUnit] = useState('USD');
  const [currencyIndex, setCurrencyIndex] = useState(
    buildBillingCurrencyIndex([], { activeOnly: true })
  );

  const [createOpen, setCreateOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [viewOpen, setViewOpen] = useState(false);
  const [deleteOpen, setDeleteOpen] = useState(false);
  const [assignOpen, setAssignOpen] = useState(false);

  const [form, setForm] = useState(createEmptyForm('USD'));
  const [activeRow, setActiveRow] = useState(null);
  const [assignRow, setAssignRow] = useState(null);
  const [assignForm, setAssignForm] = useState(createEmptyAssignForm());

  const displayUnitOptions = useMemo(
    () => buildDisplayUnitOptions(currencyIndex, { order: 'yyc-first' }),
    [currencyIndex]
  );

  const billingUnitOptions = useMemo(
    () => buildBillingUnitOptions(currencyIndex),
    [currencyIndex]
  );

  const normalizedKeyword = useMemo(
    () => (typeof searchKeyword === 'string' ? searchKeyword.trim() : ''),
    [searchKeyword]
  );
  const keywordFromURL = useMemo(
    () => (searchParams.get('keyword') || '').toString().trim(),
    [searchParams]
  );

  const totalPages = useMemo(() => {
    if (totalCount <= 0) return 1;
    return Math.max(1, Math.ceil(totalCount / ITEMS_PER_PAGE));
  }, [totalCount]);

  const loadGroups = useCallback(async () => {
    setGroupLoading(true);
    try {
      const items = [];
      let page = 1;
      while (page <= 50) {
        const res = await API.get('/api/v1/admin/groups', {
          params: {
            page,
            page_size: 100,
          },
        });
        const { success, message, data } = res.data || {};
        if (!success) {
          showError(message || t('package_manage.messages.group_load_failed'));
          return;
        }
        const pageItems = Array.isArray(data?.items) ? data.items : [];
        items.push(...pageItems);
        const total = Number(data?.total || pageItems.length || 0);
        if (pageItems.length === 0 || items.length >= total || pageItems.length < 100) {
          break;
        }
        page += 1;
      }
      setGroupOptions(toGroupOptions(items));
    } catch (error) {
      showError(error);
    } finally {
      setGroupLoading(false);
    }
  }, [t]);

  const loadUserOptions = useCallback(async () => {
    if (userOptions.length > 0) {
      return;
    }
    setUserLoading(true);
    try {
      const items = [];
      let page = 1;
      while (page <= 100) {
        const res = await API.get('/api/v1/admin/user/', {
          params: {
            page,
          },
        });
        const { success, message, data, meta } = res.data || {};
        if (!success) {
          showError(message || t('package_manage.messages.user_load_failed'));
          return;
        }
        const pageItems = Array.isArray(data) ? data : [];
        items.push(...pageItems);
        const total = Number(meta?.total || pageItems.length || 0);
        if (pageItems.length === 0 || items.length >= total) {
          break;
        }
        page += 1;
      }
      setUserOptions(toUserOptions(items));
    } catch (error) {
      showError(error);
    } finally {
      setUserLoading(false);
    }
  }, [t, userOptions.length]);

  const loadPackages = useCallback(
    async (page, keyword) => {
      setLoading(true);
      try {
        const res = await API.get('/api/v1/admin/packages', {
          params: {
            page: Math.max(Number(page) || 1, 1),
            page_size: ITEMS_PER_PAGE,
            keyword: keyword || undefined,
          },
        });
        const { success, message, data } = res.data || {};
        if (!success) {
          showError(message || t('package_manage.messages.load_failed'));
          return;
        }
        setRows(Array.isArray(data?.items) ? data.items : []);
        setTotalCount(Number(data?.total || 0));
      } catch (error) {
        showError(error);
      } finally {
        setLoading(false);
      }
    },
    [t]
  );

  const loadDisplayUnits = useCallback(async () => {
    try {
      const res = await API.get('/api/v1/admin/billing/currencies');
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message);
        return;
      }
      const next = buildBillingCurrencyIndex(Array.isArray(data) ? data : [], {
        activeOnly: true,
      });
      setCurrencyIndex(next);
      setDisplayUnit((current) => {
        const normalizedCurrent = (current || '').toString().trim().toUpperCase();
        if (normalizedCurrent && next[normalizedCurrent]) {
          return normalizedCurrent;
        }
        if (next.USD) {
          return 'USD';
        }
        const fallbackUnit = Object.keys(next)
          .filter((code) => code)
          .sort((a, b) => a.localeCompare(b))[0];
        return fallbackUnit || 'YYC';
      });
    } catch (error) {
      showError(error?.message || error);
    }
  }, []);

  useEffect(() => {
    loadGroups().then();
  }, [loadGroups]);

  useEffect(() => {
    loadDisplayUnits().then();
  }, [loadDisplayUnits]);

  useEffect(() => {
    const defaultBillingUnit = resolveDefaultBillingUnit(currencyIndex);
    setForm((current) => {
      if ((current?.id || '').toString().trim() !== '') {
        return current;
      }
      const nextDailyUnit = current?.daily_amount_unit || defaultBillingUnit;
      const nextEmergencyUnit = current?.emergency_amount_unit || defaultBillingUnit;
      if (
        nextDailyUnit === current?.daily_amount_unit &&
        nextEmergencyUnit === current?.emergency_amount_unit
      ) {
        return current;
      }
      return {
        ...current,
        daily_amount_unit: nextDailyUnit,
        emergency_amount_unit: nextEmergencyUnit,
      };
    });
  }, [currencyIndex]);

  useEffect(() => {
    setSearchKeyword(keywordFromURL);
    setActivePage(1);
  }, [keywordFromURL]);

  useEffect(() => {
    loadPackages(activePage, normalizedKeyword).then();
  }, [activePage, normalizedKeyword, loadPackages]);

  useEffect(() => {
    if (activePage > totalPages) {
      setActivePage(totalPages);
    }
  }, [activePage, totalPages]);

  const resetForm = () => {
    setForm(createEmptyForm(resolveDefaultBillingUnit(currencyIndex)));
  };

  const closeAllModals = () => {
    if (submitting) return;
    setCreateOpen(false);
    setEditOpen(false);
    setViewOpen(false);
    setDeleteOpen(false);
    setAssignOpen(false);
    setActiveRow(null);
    setAssignRow(null);
    setAssignForm(createEmptyAssignForm());
    resetForm();
  };

  const openCreateModal = () => {
    if (submitting) return;
    setCreateOpen(true);
    setEditOpen(false);
    setViewOpen(false);
    setActiveRow(null);
    resetForm();
  };

  const openViewModal = async (row) => {
    if (!row || submitting) return;
    const id = (row.id || '').toString().trim();
    if (id === '') return;
    try {
      const res = await API.get(`/api/v1/admin/package/${encodeURIComponent(id)}`);
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('package_manage.messages.load_failed'));
        return;
      }
      setActiveRow(data);
      setViewOpen(true);
    } catch (error) {
      showError(error);
    }
  };

  const openEditModal = async (row) => {
    if (!row || submitting) return;
    const id = (row.id || '').toString().trim();
    if (!id) {
      return;
    }
    try {
      const res = await API.get(`/api/v1/admin/package/${encodeURIComponent(id)}`);
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('package_manage.messages.load_failed'));
        return;
      }
      const detail = data || row;
      const resolvedGroupID = (detail?.group_id || row?.group_id || '').toString().trim();
      const resolvedGroupName = (detail?.group_name || row?.group_name || '').toString().trim();
      const defaultBillingUnit = resolveDefaultBillingUnit(currencyIndex);
      setGroupOptions((current) =>
        appendGroupOptionIfMissing(current, resolvedGroupID, resolvedGroupName)
      );
      setActiveRow(detail);
      setForm({
        id: detail.id || '',
        name: detail.name || '',
        description: detail.description || '',
        group_id: resolvedGroupID,
        sale_price: detail?.sale_price ?? '0',
        sale_currency: detail?.sale_currency || 'CNY',
        daily_amount: yycToBillingInputValue(
          Number(detail?.yyc_daily_limit ?? detail?.daily_quota_limit ?? 0),
          defaultBillingUnit,
          currencyIndex
        ),
        daily_amount_unit: defaultBillingUnit,
        emergency_amount: yycToBillingInputValue(
          Number(detail?.yyc_package_emergency_limit ?? detail?.package_emergency_quota_limit ?? 0),
          defaultBillingUnit,
          currencyIndex
        ),
        emergency_amount_unit: defaultBillingUnit,
        duration_days: Number(detail?.duration_days || 30),
        reset_timezone: detail?.quota_reset_timezone || 'Asia/Shanghai',
        enabled: Boolean(detail?.enabled),
        sort_order: Number(detail?.sort_order || 0),
        source: detail?.source || 'manual',
      });
      setEditOpen(true);
    } catch (error) {
      showError(error?.message || error);
    }
  };

  const openDeleteModal = (row) => {
    if (!row || submitting) return;
    setActiveRow(row);
    setDeleteOpen(true);
  };

  const openAssignModal = (row) => {
    if (!row || submitting) return;
    setAssignRow(row);
    setAssignForm({
      user_id: '',
      start_at: '',
    });
    setAssignOpen(true);
    loadUserOptions().then();
  };

  const buildPayloadFromForm = () => {
    const name = (form.name || '').trim();
    if (name === '') {
      showInfo(t('package_manage.messages.name_required'));
      return null;
    }
    const groupID = (form.group_id || '').trim();
    if (groupID === '') {
      showInfo(t('package_manage.messages.group_required'));
      return null;
    }
    const dailyStored = billingInputValueToYYC(
      form.daily_amount ?? 0,
      form.daily_amount_unit,
      currencyIndex
    );
    const emergencyStored = billingInputValueToYYC(
      form.emergency_amount ?? 0,
      form.emergency_amount_unit,
      currencyIndex
    );
    if (
      !Number.isFinite(Number(form.sale_price || 0)) ||
      Number(form.sale_price || 0) < 0 ||
      !Number.isFinite(dailyStored) ||
      dailyStored < 0 ||
      !Number.isFinite(emergencyStored) ||
      emergencyStored < 0
    ) {
      showInfo(t('package_manage.messages.quota_invalid'));
      return null;
    }
    const durationDays = Number(form.duration_days || 0);
    if (!Number.isFinite(durationDays) || durationDays <= 0) {
      showInfo(t('package_manage.messages.duration_invalid'));
      return null;
    }
    return {
      id: (form.id || '').trim(),
      name,
      description: (form.description || '').trim(),
      group_id: groupID,
      sale_price: Number(form.sale_price || 0),
      sale_currency: (form.sale_currency || 'CNY').trim().toUpperCase() || 'CNY',
      daily_quota_limit: Math.trunc(dailyStored),
      package_emergency_quota_limit: Math.trunc(emergencyStored),
      duration_days: Math.trunc(durationDays),
      quota_reset_timezone:
        (form.reset_timezone || '').trim() || 'Asia/Shanghai',
      enabled: Boolean(form.enabled),
      sort_order: Math.trunc(Number(form.sort_order || 0)),
      source: (form.source || '').trim() || 'manual',
    };
  };

  const submitCreate = async () => {
    const payload = buildPayloadFromForm();
    if (!payload) return;
    setSubmitting(true);
    try {
      const res = await API.post('/api/v1/admin/package/', payload);
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('package_manage.messages.create_failed'));
        return;
      }
      showSuccess(t('package_manage.messages.create_success'));
      setCreateOpen(false);
      resetForm();
      loadPackages(activePage, normalizedKeyword).then();
    } catch (error) {
      showError(error);
    } finally {
      setSubmitting(false);
    }
  };

  const submitEdit = async () => {
    const payload = buildPayloadFromForm();
    if (!payload) return;
    if (!payload.id) {
      showInfo(t('package_manage.messages.id_required'));
      return;
    }
    setSubmitting(true);
    try {
      const res = await API.put('/api/v1/admin/package/', payload);
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message || t('package_manage.messages.update_failed'));
        return;
      }
      showSuccess(t('package_manage.messages.update_success'));
      setEditOpen(false);
      setActiveRow(data || null);
      resetForm();
      loadPackages(activePage, normalizedKeyword).then();
    } catch (error) {
      showError(error);
    } finally {
      setSubmitting(false);
    }
  };

  const submitDelete = async () => {
    const id = (activeRow?.id || '').toString().trim();
    if (id === '' || submitting) return;
    setSubmitting(true);
    try {
      const res = await API.delete(`/api/v1/admin/package/${encodeURIComponent(id)}`);
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('package_manage.messages.delete_failed'));
        return;
      }
      showSuccess(t('package_manage.messages.delete_success'));
      setDeleteOpen(false);
      setActiveRow(null);
      if (rows.length === 1 && activePage > 1) {
        setActivePage((prev) => Math.max(1, prev - 1));
      } else {
        loadPackages(activePage, normalizedKeyword).then();
      }
    } catch (error) {
      showError(error);
    } finally {
      setSubmitting(false);
    }
  };

  const submitAssign = async () => {
    const packageID = (assignRow?.id || '').toString().trim();
    const userID = (assignForm.user_id || '').toString().trim();
    if (packageID === '' || submitting) return;
    if (userID === '') {
      showInfo(t('package_manage.messages.user_required'));
      return;
    }
    const startAt = parseDatetimeLocalValue(assignForm.start_at);
    if (!Number.isFinite(startAt)) {
      showInfo(t('package_manage.messages.start_at_invalid'));
      return;
    }
    setSubmitting(true);
    try {
      const res = await API.post(
        `/api/v1/admin/package/${encodeURIComponent(packageID)}/assign`,
        {
          user_id: userID,
          start_at: startAt > 0 ? startAt : 0,
        }
      );
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('package_manage.messages.assign_failed'));
        return;
      }
      showSuccess(t('package_manage.messages.assign_success'));
      setAssignOpen(false);
      setAssignRow(null);
      setAssignForm(createEmptyAssignForm());
    } catch (error) {
      showError(error);
    } finally {
      setSubmitting(false);
    }
  };

  const renderTable = () => (
    <>
      <div className='router-toolbar router-block-gap-sm'>
        <div className='router-toolbar-start'>
          <Button
            type='button'
            className='router-page-button'
            onClick={openCreateModal}
            disabled={submitting}
          >
            {t('package_manage.buttons.add')}
          </Button>
          <Button
            type='button'
            className='router-page-button'
            onClick={() => loadPackages(activePage, normalizedKeyword)}
            loading={loading}
            disabled={submitting}
          >
            {t('package_manage.buttons.refresh')}
          </Button>
        </div>
        <Form className='router-search-form-md'>
          <Form.Input
            className='router-section-input'
            icon='search'
            iconPosition='left'
            placeholder={t('package_manage.search')}
            value={searchKeyword}
            onChange={(e, { value }) => {
              setSearchKeyword(value || '');
              setActivePage(1);
            }}
          />
        </Form>
      </div>

      <Table basic='very' compact className='router-hover-table router-list-table'>
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell>{t('package_manage.table.name')}</Table.HeaderCell>
            <Table.HeaderCell>{t('package_manage.table.group')}</Table.HeaderCell>
            <Table.HeaderCell>{t('package_manage.table.sale_price')}</Table.HeaderCell>
            <Table.HeaderCell className='router-redemption-face-value-header'>
              <div className='router-table-header-with-control'>
                <span>{t('package_manage.table.daily_quota_limit')}</span>
                <UnitDropdown
                  variant='header'
                  compact
                  options={displayUnitOptions}
                  value={displayUnit}
                  onClick={(e) => {
                    e.stopPropagation();
                  }}
                  onChange={(_, { value }) => {
                    setDisplayUnit((value || '').toString());
                  }}
                />
              </div>
            </Table.HeaderCell>
            <Table.HeaderCell className='router-redemption-face-value-header'>
              <div className='router-table-header-with-control'>
                <span>{t('package_manage.table.package_emergency_quota_limit')}</span>
                <UnitDropdown
                  variant='header'
                  compact
                  options={displayUnitOptions}
                  value={displayUnit}
                  onClick={(e) => {
                    e.stopPropagation();
                  }}
                  onChange={(_, { value }) => {
                    setDisplayUnit((value || '').toString());
                  }}
                />
              </div>
            </Table.HeaderCell>
            <Table.HeaderCell>{t('package_manage.table.duration_days')}</Table.HeaderCell>
            <Table.HeaderCell>{t('package_manage.table.status')}</Table.HeaderCell>
            <Table.HeaderCell>{t('package_manage.table.created_at')}</Table.HeaderCell>
            <Table.HeaderCell>{t('package_manage.table.updated_at')}</Table.HeaderCell>
            <Table.HeaderCell className='router-table-action-cell'>
              {t('package_manage.table.actions')}
            </Table.HeaderCell>
          </Table.Row>
        </Table.Header>

        <Table.Body>
          {rows.length === 0 ? (
            <Table.Row>
              <Table.Cell colSpan={10} textAlign='center' className='router-empty-cell'>
                {loading
                  ? t('package_manage.messages.loading')
                  : t('package_manage.table.empty')}
              </Table.Cell>
            </Table.Row>
          ) : (
            rows.map((row) => (
              <Table.Row
                key={row.id}
                className={loading || submitting ? '' : 'router-row-clickable'}
                onClick={() => openViewModal(row)}
              >
                <Table.Cell>{row.name || '-'}</Table.Cell>
                <Table.Cell>{row.group_name || row.group_id || '-'}</Table.Cell>
                <Table.Cell>{`${row.sale_currency || 'CNY'} ${row.sale_price ?? 0}`}</Table.Cell>
                <Table.Cell>
                  {renderPackageAmountFieldValue(row, 'daily', displayUnit, currencyIndex)}
                </Table.Cell>
                <Table.Cell>
                  {renderPackageAmountFieldValue(row, 'emergency', displayUnit, currencyIndex)}
                </Table.Cell>
                <Table.Cell>{Number(row.duration_days || 0) || '-'}</Table.Cell>
                <Table.Cell>{statusLabel(Boolean(row.enabled), t)}</Table.Cell>
                <Table.Cell>{row.created_at ? timestamp2string(row.created_at) : '-'}</Table.Cell>
                <Table.Cell>{row.updated_at ? timestamp2string(row.updated_at) : '-'}</Table.Cell>
                <Table.Cell className='router-nowrap'>
                  <div className='router-action-group-tight'>
                    <Button
                      type='button'
                      className='router-inline-button'
                      disabled={submitting}
                      onClick={(e) => {
                        e.stopPropagation();
                        openEditModal(row);
                      }}
                    >
                      {t('package_manage.buttons.edit')}
                    </Button>
                    <Button
                      type='button'
                      className='router-inline-button'
                      disabled={submitting}
                      onClick={(e) => {
                        e.stopPropagation();
                        openAssignModal(row);
                      }}
                    >
                      {t('package_manage.buttons.assign')}
                    </Button>
                    <Button
                      type='button'
                      className='router-inline-button'
                      disabled={submitting}
                      onClick={(e) => {
                        e.stopPropagation();
                        openDeleteModal(row);
                      }}
                    >
                      {t('package_manage.buttons.delete')}
                    </Button>
                  </div>
                </Table.Cell>
              </Table.Row>
            ))
          )}
        </Table.Body>
      </Table>

      {totalPages > 1 ? (
        <div className='router-pagination-wrap-md'>
          <Pagination
            className='router-section-pagination'
            activePage={activePage}
            totalPages={totalPages}
            onPageChange={(e, { activePage: nextActivePage }) => {
              setActivePage(Number(nextActivePage) || 1);
            }}
          />
        </div>
      ) : null}
    </>
  );

  const renderFormFields = () => (
    <Form>
      <Form.Group widths='equal'>
        <Form.Input
          className='router-section-input'
          label={t('package_manage.form.name')}
          placeholder={t('package_manage.form.name_placeholder')}
          value={form.name}
          onChange={(e, { value }) => setForm((prev) => ({ ...prev, name: value || '' }))}
        />
        <Form.Select
          className='router-section-input'
          label={t('package_manage.form.group')}
          placeholder={t('package_manage.form.group_placeholder')}
          options={groupOptions}
          value={form.group_id}
          loading={groupLoading}
          onChange={(e, { value }) =>
            setForm((prev) => ({ ...prev, group_id: (value || '').toString() }))
          }
        />
      </Form.Group>

      <Form.TextArea
        className='router-section-input'
        label={t('package_manage.form.description')}
        value={form.description}
        onChange={(e, { value }) =>
          setForm((prev) => ({ ...prev, description: (value || '').toString() }))
        }
      />

      <Form.Group widths='equal'>
        <Form.Input
          className='router-section-input'
          label={t('package_manage.form.sale_price')}
          type='number'
          min={0}
          step='0.01'
          value={form.sale_price}
          onChange={(e) =>
            setForm((prev) => ({ ...prev, sale_price: e.target.value || '0' }))
          }
        />
        <Form.Input
          className='router-section-input'
          label={t('package_manage.form.sale_currency')}
          value={form.sale_currency}
          onChange={(e, { value }) =>
            setForm((prev) => ({ ...prev, sale_currency: (value || 'CNY').toUpperCase() }))
          }
        />
      </Form.Group>

      <Form.Group widths='equal'>
        <Form.Field>
          <label>{t('package_manage.form.daily_quota_limit')}</label>
          <div className='router-section-input-with-unit'>
            <Form.Input
              className='router-section-input router-section-input-with-unit-field'
              value={form.daily_amount}
              step={resolveBillingInputStep(form.daily_amount_unit, currencyIndex)}
              min={0}
              type='number'
              onChange={(e) =>
                setForm((prev) => ({ ...prev, daily_amount: e.target.value || '0' }))
              }
            />
            <UnitDropdown
              variant='inputUnit'
              options={billingUnitOptions}
              value={form.daily_amount_unit}
              onChange={(_, { value }) => {
                const nextUnit = (value || 'YYC').toString().trim().toUpperCase();
                setForm((prev) => ({
                  ...prev,
                  daily_amount: convertBillingInputValueUnit(
                    prev.daily_amount,
                    prev.daily_amount_unit,
                    nextUnit,
                    currencyIndex
                  ),
                  daily_amount_unit: nextUnit,
                }));
              }}
              aria-label={t('package_manage.form.daily_quota_limit')}
            />
          </div>
        </Form.Field>
        <Form.Field>
          <label>{t('package_manage.form.package_emergency_quota_limit')}</label>
          <div className='router-section-input-with-unit'>
            <Form.Input
              className='router-section-input router-section-input-with-unit-field'
              value={form.emergency_amount}
              step={resolveBillingInputStep(
                form.emergency_amount_unit,
                currencyIndex
              )}
              min={0}
              type='number'
              onChange={(e) =>
                setForm((prev) => ({
                  ...prev,
                  emergency_amount: e.target.value || '0',
                }))
              }
            />
            <UnitDropdown
              variant='inputUnit'
              options={billingUnitOptions}
              value={form.emergency_amount_unit}
              onChange={(_, { value }) => {
                const nextUnit = (value || 'YYC').toString().trim().toUpperCase();
                setForm((prev) => ({
                  ...prev,
                  emergency_amount: convertBillingInputValueUnit(
                    prev.emergency_amount,
                    prev.emergency_amount_unit,
                    nextUnit,
                    currencyIndex
                  ),
                  emergency_amount_unit: nextUnit,
                }));
              }}
              aria-label={t('package_manage.form.package_emergency_quota_limit')}
            />
          </div>
        </Form.Field>
      </Form.Group>

      <Form.Group widths='equal'>
        <Form.Input
          className='router-section-input'
          label={t('package_manage.form.duration_days')}
          type='number'
          min={1}
          step={1}
          value={form.duration_days}
          onChange={(e) =>
            setForm((prev) => ({ ...prev, duration_days: e.target.value || 0 }))
          }
        />
        <Form.Input
          className='router-section-input'
          label={t('package_manage.form.quota_reset_timezone')}
          value={form.reset_timezone}
          onChange={(e, { value }) =>
            setForm((prev) => ({ ...prev, reset_timezone: value || '' }))
          }
        />
      </Form.Group>

      <Form.Group widths='equal'>
        <Form.Select
          className='router-section-input'
          label={t('package_manage.form.enabled')}
          options={[
            { key: 'enabled', value: true, text: t('package_manage.status.enabled') },
            { key: 'disabled', value: false, text: t('package_manage.status.disabled') },
          ]}
          value={Boolean(form.enabled)}
          onChange={(e, { value }) =>
            setForm((prev) => ({ ...prev, enabled: Boolean(value) }))
          }
        />
        <Form.Input
          className='router-section-input'
          label={t('package_manage.form.sort_order')}
          type='number'
          step={1}
          value={form.sort_order}
          onChange={(e) =>
            setForm((prev) => ({ ...prev, sort_order: e.target.value || 0 }))
          }
        />
      </Form.Group>

      <Form.Input
        className='router-section-input'
        label={t('package_manage.form.source')}
        value={form.source}
        onChange={(e, { value }) =>
          setForm((prev) => ({ ...prev, source: value || '' }))
        }
      />
    </Form>
  );

  const renderDetailModal = () => (
    <Modal
      open={viewOpen}
      onClose={closeAllModals}
      size='small'
    >
      <Modal.Header>{t('package_manage.dialog.detail_title')}</Modal.Header>
      <Modal.Content>
        <Form>
          <Form.Group widths='equal'>
            <Form.Input
              className='router-section-input'
              label={t('package_manage.table.name')}
              value={activeRow?.name || '-'}
              readOnly
            />
            <Form.Input
              className='router-section-input'
              label={t('package_manage.table.group')}
              value={activeRow?.group_name || activeRow?.group_id || '-'}
              readOnly
            />
          </Form.Group>

          <Form.TextArea
            className='router-section-input'
            label={t('package_manage.form.description')}
            value={activeRow?.description || '-'}
            readOnly
          />

          <Form.Group widths='equal'>
            <Form.Input
              className='router-section-input'
              label={t('package_manage.form.sale_price')}
              value={`${activeRow?.sale_currency || 'CNY'} ${activeRow?.sale_price ?? 0}`}
              readOnly
            />
            <Form.Input
              className='router-section-input'
              label={t('package_manage.form.sale_currency')}
              value={activeRow?.sale_currency || 'CNY'}
              readOnly
            />
          </Form.Group>

          <Form.Group widths='equal'>
            <Form.Input
              className='router-section-input'
              label={t('package_manage.table.daily_quota_limit')}
              value={renderPackageAmountFieldValue(
                activeRow,
                'daily',
                displayUnit,
                currencyIndex
              )}
              readOnly
            />
            <Form.Input
              className='router-section-input'
              label={t('package_manage.table.package_emergency_quota_limit')}
              value={renderPackageAmountFieldValue(
                activeRow,
                'emergency',
                displayUnit,
                currencyIndex
              )}
              readOnly
            />
          </Form.Group>

          <Form.Group widths='equal'>
            <Form.Input
              className='router-section-input'
              label={t('package_manage.table.duration_days')}
              value={Number(activeRow?.duration_days || 0) || '-'}
              readOnly
            />
            <Form.Input
              className='router-section-input'
              label={t('package_manage.table.status')}
              value={
                activeRow?.enabled
                  ? t('package_manage.status.enabled')
                  : t('package_manage.status.disabled')
              }
              readOnly
            />
          </Form.Group>

          <Form.Group widths='equal'>
            <Form.Input
              className='router-section-input'
              label={t('package_manage.form.quota_reset_timezone')}
              value={activeRow?.quota_reset_timezone || '-'}
              readOnly
            />
            <Form.Input
              className='router-section-input'
              label={t('package_manage.table.updated_at')}
              value={activeRow?.updated_at ? timestamp2string(activeRow.updated_at) : '-'}
              readOnly
            />
          </Form.Group>
        </Form>
      </Modal.Content>
      <Modal.Actions>
        <Button type='button' onClick={closeAllModals} disabled={submitting}>
          {t('common.cancel')}
        </Button>
      </Modal.Actions>
    </Modal>
  );

  return (
    <div>
      {renderTable()}

      <Modal
        open={createOpen}
        onClose={closeAllModals}
        size='small'
      >
        <Modal.Header>{t('package_manage.dialog.create_title')}</Modal.Header>
        <Modal.Content>{renderFormFields()}</Modal.Content>
        <Modal.Actions>
          <Button type='button' onClick={closeAllModals} disabled={submitting}>
            {t('common.cancel')}
          </Button>
          <Button type='button' color='blue' loading={submitting} onClick={submitCreate}>
            {t('common.confirm')}
          </Button>
        </Modal.Actions>
      </Modal>

      <Modal
        open={editOpen}
        onClose={closeAllModals}
        size='small'
      >
        <Modal.Header>{t('package_manage.dialog.edit_title')}</Modal.Header>
        <Modal.Content>{renderFormFields()}</Modal.Content>
        <Modal.Actions>
          <Button type='button' onClick={closeAllModals} disabled={submitting}>
            {t('common.cancel')}
          </Button>
          <Button type='button' color='blue' loading={submitting} onClick={submitEdit}>
            {t('common.confirm')}
          </Button>
        </Modal.Actions>
      </Modal>

      <Modal
        open={assignOpen}
        onClose={closeAllModals}
        size='small'
      >
        <Modal.Header>{t('package_manage.dialog.assign_title')}</Modal.Header>
        <Modal.Content>
          <Form>
            <Form.Select
              className='router-section-input'
              search
              selection
              clearable
              loading={userLoading}
              label={t('package_manage.assign.user_id')}
              placeholder={t('package_manage.assign.user_id_placeholder')}
              options={userOptions}
              value={assignForm.user_id}
              onChange={(e, { value }) =>
                setAssignForm((prev) => ({
                  ...prev,
                  user_id: (value || '').toString(),
                }))
              }
            />
            <Form.Input
              className='router-section-input'
              type='datetime-local'
              label={t('package_manage.assign.start_at')}
              placeholder={t('package_manage.assign.start_at_placeholder')}
              value={assignForm.start_at}
              onChange={(e, { value }) =>
                setAssignForm((prev) => ({
                  ...prev,
                  start_at: value || '',
                }))
              }
            />
            <Form.Input
              className='router-section-input'
              label={t('package_manage.assign.package')}
              value={assignRow?.name || '-'}
              readOnly
            />
          </Form>
        </Modal.Content>
        <Modal.Actions>
          <Button type='button' onClick={closeAllModals} disabled={submitting}>
            {t('common.cancel')}
          </Button>
          <Button type='button' color='blue' loading={submitting} onClick={submitAssign}>
            {t('common.confirm')}
          </Button>
        </Modal.Actions>
      </Modal>

      <Modal
        open={deleteOpen}
        onClose={closeAllModals}
        size='tiny'
      >
        <Modal.Header>{t('package_manage.dialog.delete_title')}</Modal.Header>
        <Modal.Content>
          {t('package_manage.dialog.delete_content', {
            name: activeRow?.name || '-',
          })}
        </Modal.Content>
        <Modal.Actions>
          <Button type='button' onClick={closeAllModals} disabled={submitting}>
            {t('common.cancel')}
          </Button>
          <Button type='button' color='red' loading={submitting} onClick={submitDelete}>
            {t('common.confirm')}
          </Button>
        </Modal.Actions>
      </Modal>

      {renderDetailModal()}
    </div>
  );
};

export default PackagesManager;
