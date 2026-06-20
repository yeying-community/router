import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { API, showError, showInfo, showSuccess, timestamp2string } from '../helpers';
import { ITEMS_PER_PAGE } from '../constants';
import {
  PACKAGE_LIST_COLUMN_WIDTHS,
  PACKAGE_LIST_TABLE_MIN_WIDTH,
} from '../constants/tableWidthPresets';
import {
  buildBillingCurrencyIndex,
  buildDisplayUnitOptions,
  buildBillingUnitOptions,
  convertBillingInputValueUnit,
  billingInputValueToChargeAmount,
  chargeAmountToBillingInputValue,
  resolveDefaultBillingUnit,
  resolveBillingInputStep,
} from '../helpers/billing';
import UnitDropdown from './UnitDropdown';
import {
  AppButton,
  AppCompact,
  AppField,
  AppFilterHeader,
  AppFormActions,
  AppFormRow,
  AppInput,
  AppInputNumber,
  AppModal,
  AppPagination,
  AppSelect,
  AppSwitch,
  AppTable,
  AppTableActionButton,
  AppTextarea,
} from '../router-ui';
import {
  formatDecimalNumber,
} from '../helpers/render';

const createEmptyForm = (defaultBillingUnit = 'USD') => ({
  id: '',
  name: '',
  description: '',
  group_id: '',
  visibility_scope: 'all',
  visible_user_ids: [],
  sale_price: '0',
  sale_currency: 'CNY',
  daily_amount: '0',
  daily_amount_unit: defaultBillingUnit,
  emergency_amount: '0',
  emergency_amount_unit: defaultBillingUnit,
  duration_days: 30,
  reset_timezone: 'Asia/Shanghai',
  enabled: true,
  source: 'manual',
});

const toGroupOptions = (rows) =>
  (Array.isArray(rows) ? rows : []).map((item) => ({
    key: item.id,
    value: item.id,
    text: item.name || item.id,
  }));

const toUserOption = (item) => {
  const id = (item?.id || '').toString().trim();
  const username = (item?.username || '').toString().trim();
  const displayName = (item?.display_name || '').toString().trim();
  const normalizedUsername = username.toLowerCase();
  const normalizedDisplayName = displayName.toLowerCase();
  const primaryName = displayName || username;
  const secondaryName =
    primaryName &&
    normalizedUsername &&
    normalizedDisplayName &&
    normalizedUsername !== normalizedDisplayName
      ? (primaryName === displayName ? username : displayName)
      : '';
  return {
    key: id,
    value: id,
    text: [primaryName, secondaryName].filter(Boolean).join(' / ') || id,
  };
};

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

const appendUserOptionsIfMissing = (options, users) => {
  const currentOptions = Array.isArray(options) ? options : [];
  const nextOptions = [...currentOptions];
  const seen = new Set(
    currentOptions.map((item) => (item?.value || '').toString().trim()).filter(Boolean)
  );
  (Array.isArray(users) ? users : []).forEach((item) => {
    const option = toUserOption(item);
    const normalizedID = (option?.value || '').toString().trim();
    if (!normalizedID || seen.has(normalizedID)) {
      return;
    }
    seen.add(normalizedID);
    nextOptions.push(option);
  });
  return nextOptions;
};

const resolveSelectedUserListFromOptions = (userIDs, options) => {
  const optionMap = new Map(
    (Array.isArray(options) ? options : [])
      .map((item) => [
        (item?.value || '').toString().trim(),
        {
          label: (item?.text || '').toString().trim(),
          walletAddress: (item?.wallet_address || '').toString().trim(),
        },
      ])
      .filter(([key]) => key !== ''),
  );
  return (Array.isArray(userIDs) ? userIDs : [])
    .map((item) => {
      const id = (item || '').toString().trim();
      if (!id) {
        return null;
      }
      const matched = optionMap.get(id);
      return {
        key: id,
        label: matched?.label || id,
        walletAddress: matched?.walletAddress || '',
      };
    })
    .filter(Boolean);
};

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

const renderPackageAmountValue = (chargeAmount, displayUnit, currencyIndex) => {
  const normalizedChargeAmount = Number(chargeAmount || 0);
  if (!Number.isFinite(normalizedChargeAmount)) {
    return '-';
  }
  const targetCurrency = currencyIndex[displayUnit] || currencyIndex.YYC;
  const rate = Number(targetCurrency?.charge_rate || 0);
  if (!Number.isFinite(rate) || rate <= 0) {
    return '-';
  }
  return formatByCurrencyMinorUnit(normalizedChargeAmount / rate, targetCurrency);
};

const resolvePackageChargeAmount = (row, type) => {
  if (type === 'daily') {
    return Number(row?.daily_quota_limit ?? 0);
  }
  return Number(row?.package_emergency_quota_limit ?? 0);
};

const ensureUnitOption = (options, value) => {
  const normalized = (value || '').toString().trim().toUpperCase();
  const items = Array.isArray(options) ? options : [];
  if (!normalized || items.some((item) => item?.value === normalized)) {
    return items;
  }
  return [...items, { value: normalized, label: normalized }];
};

const renderPackageAmountFieldValue = (row, type, displayUnit, currencyIndex) => {
  const normalizedChargeAmount = resolvePackageChargeAmount(row, type);
  if (!Number.isFinite(normalizedChargeAmount)) {
    return '-';
  }
  return renderPackageAmountValue(normalizedChargeAmount, displayUnit, currencyIndex);
};

const PackagesManager = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
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
  const [deleteOpen, setDeleteOpen] = useState(false);

  const [form, setForm] = useState(createEmptyForm('USD'));
  const [activeRow, setActiveRow] = useState(null);

  const displayUnitOptions = useMemo(
    () => buildDisplayUnitOptions(currencyIndex, { order: 'charge-first' }),
    [currencyIndex]
  );

  const billingUnitOptions = useMemo(
    () => buildBillingUnitOptions(currencyIndex),
    [currencyIndex]
  );

  const saleCurrencyOptions = useMemo(
    () => ensureUnitOption(displayUnitOptions, form.sale_currency || 'CNY'),
    [displayUnitOptions, form.sale_currency]
  );

  const selectedVisibleUsers = useMemo(
    () => resolveSelectedUserListFromOptions(form.visible_user_ids, userOptions),
    [form.visible_user_ids, userOptions]
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

  const searchUsers = useCallback(
    async (keyword) => {
      const normalizedValue = (keyword || '').toString().trim();
      if (normalizedValue === '') {
        return;
      }
      setUserLoading(true);
      try {
        const res = await API.get('/api/v1/admin/user/search', {
          params: {
            keyword: normalizedValue,
          },
        });
        const { success, message, data } = res?.data || {};
        if (!success) {
          showError(message || t('package_manage.messages.user_load_failed'));
          return;
        }
        setUserOptions((current) => appendUserOptionsIfMissing(current, data));
      } catch (error) {
        showError(error?.message || t('package_manage.messages.user_load_failed'));
      } finally {
        setUserLoading(false);
      }
    },
    [t]
  );

  const loadInitialUsers = useCallback(async () => {
    setUserLoading(true);
    try {
      const res = await API.get('/api/v1/admin/user', {
        params: {
          page: 1,
        },
      });
      const { success, message, data } = res?.data || {};
      if (!success) {
        showError(message || t('package_manage.messages.user_load_failed'));
        return;
      }
      setUserOptions((current) => appendUserOptionsIfMissing(current, data));
    } catch (error) {
      showError(error?.message || t('package_manage.messages.user_load_failed'));
    } finally {
      setUserLoading(false);
    }
  }, [t]);

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
    setDeleteOpen(false);
    setActiveRow(null);
    resetForm();
  };

  const openCreateModal = () => {
    if (submitting) return;
    setCreateOpen(true);
    setEditOpen(false);
    setActiveRow(null);
    resetForm();
    loadInitialUsers().then();
  };

  const openViewPage = (row) => {
    if (!row || submitting) return;
    const id = (row.id || '').toString().trim();
    if (id === '') return;
    navigate(`/admin/package/detail/${encodeURIComponent(id)}`);
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
        visibility_scope: detail?.visibility_scope || 'all',
        visible_user_ids: Array.isArray(detail?.visible_user_ids)
          ? detail.visible_user_ids.map((item) => (item || '').toString()).filter(Boolean)
          : [],
        sale_price: detail?.sale_price ?? '0',
        sale_currency: detail?.sale_currency || 'CNY',
        daily_amount: chargeAmountToBillingInputValue(
          Number(detail?.daily_quota_limit ?? 0),
          defaultBillingUnit,
          currencyIndex
        ),
        daily_amount_unit: defaultBillingUnit,
        emergency_amount: chargeAmountToBillingInputValue(
          Number(detail?.package_emergency_quota_limit ?? 0),
          defaultBillingUnit,
          currencyIndex
        ),
        emergency_amount_unit: defaultBillingUnit,
        duration_days: Number(detail?.duration_days || 30),
        reset_timezone: detail?.quota_reset_timezone || 'Asia/Shanghai',
        enabled: Boolean(detail?.enabled),
        source: detail?.source || 'manual',
      });
      setUserOptions((current) =>
        appendUserOptionsIfMissing(current, detail?.visible_users)
      );
      loadInitialUsers().then();
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
    const visibilityScope = (form.visibility_scope || 'all').toString().trim() || 'all';
    const visibleUserIDs = Array.isArray(form.visible_user_ids)
      ? [...new Set(form.visible_user_ids.map((item) => (item || '').toString().trim()).filter(Boolean))]
      : [];
    const dailyStored = billingInputValueToChargeAmount(
      form.daily_amount ?? 0,
      form.daily_amount_unit,
      currencyIndex
    );
    const emergencyStored = billingInputValueToChargeAmount(
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
      visibility_scope: visibilityScope,
      visible_user_ids: visibleUserIDs,
      sale_price: Number(form.sale_price || 0),
      sale_currency: (form.sale_currency || 'CNY').trim().toUpperCase() || 'CNY',
      daily_quota_limit: Math.trunc(dailyStored),
      package_emergency_quota_limit: Math.trunc(emergencyStored),
      duration_days: Math.trunc(durationDays),
      quota_reset_timezone:
        (form.reset_timezone || '').trim() || 'Asia/Shanghai',
      enabled: Boolean(form.enabled),
      sort_order: Math.trunc(Number(activeRow?.sort_order || 0)),
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

  const updatePackageRow = useCallback(
    async (row, patch) => {
      const id = (row?.id || '').toString().trim();
      if (id === '' || submitting) return;
      setSubmitting(true);
      try {
        const visibleUserIDs = Array.isArray(row?.visible_user_ids)
          ? row.visible_user_ids.map((item) => (item || '').toString().trim()).filter(Boolean)
          : Array.isArray(row?.visible_users)
            ? row.visible_users
              .map((item) => (item?.id || '').toString().trim())
              .filter(Boolean)
            : [];
        const payload = {
          id,
          name: row?.name || '',
          description: row?.description || '',
          group_id: row?.group_id || '',
          visibility_scope: row?.visibility_scope || 'all',
          visible_user_ids: visibleUserIDs,
          sale_price: Number(row?.sale_price || 0),
          sale_currency: row?.sale_currency || 'CNY',
          daily_quota_limit: Number(row?.daily_quota_limit || 0),
          package_emergency_quota_limit: Number(row?.package_emergency_quota_limit || 0),
          duration_days: Number(row?.duration_days || 0),
          quota_reset_timezone: row?.quota_reset_timezone || 'Asia/Shanghai',
          enabled: Boolean(row?.enabled),
          source: row?.source || 'manual',
          ...patch,
        };
        const res = await API.put('/api/v1/admin/package/', payload);
        const { success, message, data } = res.data || {};
        if (!success) {
          showError(message || t('package_manage.messages.update_failed'));
          return;
        }
        setRows((current) =>
          current.map((item) => (((item?.id || '') === (data?.id || '')) ? data : item)),
        );
        if ((activeRow?.id || '') === (data?.id || '')) {
          setActiveRow(data);
        }
        showSuccess(t('package_manage.messages.update_success'));
      } catch (error) {
        showError(error?.message || t('package_manage.messages.update_failed'));
      } finally {
        setSubmitting(false);
      }
    },
    [activeRow, submitting, t],
  );

  const toggleEnabled = useCallback(
    async (row, checked) => {
      await updatePackageRow(row, {
        enabled: Boolean(checked),
      });
    },
    [updatePackageRow],
  );

  const renderTable = () => (
    <>
      <AppFilterHeader
        breadcrumbs={[
          { key: 'admin', label: t('header.admin_workspace') },
          { key: 'business', label: t('header.business_operation') },
          { key: 'package', label: t('header.package'), active: true },
        ]}
        title={t('header.package')}
        actions={
          <div className='router-list-toolbar-actions'>
            <AppButton
              type='button'
              className='router-page-button'
              color='blue'
              onClick={openCreateModal}
              disabled={submitting}
            >
              {t('package_manage.buttons.add')}
            </AppButton>
            <AppButton
              type='button'
              className='router-page-button'
              onClick={() => loadPackages(activePage, normalizedKeyword)}
              loading={loading}
              disabled={submitting}
            >
              {t('package_manage.buttons.refresh')}
            </AppButton>
          </div>
        }
        query={
          <AppInput
            className='router-section-input router-search-form-sm'
            placeholder={t('package_manage.search')}
            value={searchKeyword}
            onChange={(e, { value }) => {
              setSearchKeyword(value || '');
              setActivePage(1);
            }}
          />
        }
      />

      <div className='router-table-scroll-x'>
        <AppTable
          className='router-hover-table router-list-table router-table-fit-page router-package-list-table'
          pagination={false}
          scroll={{ x: PACKAGE_LIST_TABLE_MIN_WIDTH }}
          rowKey='id'
          dataSource={rows}
          locale={{
            emptyText: loading
              ? t('package_manage.messages.loading')
              : t('package_manage.table.empty'),
          }}
          onRow={(row) => ({
            className: loading || submitting ? '' : 'router-row-clickable',
            onClick: () => openViewPage(row),
          })}
          columns={[
            {
              title: t('package_manage.table.name'),
              dataIndex: 'name',
              key: 'name',
              className: 'router-package-name-cell',
              width: PACKAGE_LIST_COLUMN_WIDTHS.name,
              ellipsis: true,
              render: (value) => value || '-',
            },
            {
              title: t('package_manage.table.group'),
              dataIndex: 'group_name',
              key: 'group_name',
              width: PACKAGE_LIST_COLUMN_WIDTHS.group,
              ellipsis: true,
              render: (_, row) => row.group_name || row.group_id || '-',
            },
            {
              title: t('package_manage.table.sale_price'),
              dataIndex: 'sale_price',
              key: 'sale_price',
              className: 'router-package-sale-price-cell',
              width: PACKAGE_LIST_COLUMN_WIDTHS.salePrice,
              render: (_, row) => `${row.sale_currency || 'CNY'} ${row.sale_price ?? 0}`,
            },
            {
              title: (
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
              ),
              key: 'daily_quota_limit',
              className: 'router-package-daily-quota-cell',
              width: PACKAGE_LIST_COLUMN_WIDTHS.dailyQuota,
              render: (_, row) =>
                renderPackageAmountFieldValue(row, 'daily', displayUnit, currencyIndex),
            },
            {
              title: (
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
              ),
              key: 'package_emergency_quota_limit',
              className: 'router-package-emergency-quota-cell',
              width: PACKAGE_LIST_COLUMN_WIDTHS.emergencyQuota,
              render: (_, row) =>
                renderPackageAmountFieldValue(row, 'emergency', displayUnit, currencyIndex),
            },
            {
              title: t('package_manage.table.duration_days'),
              dataIndex: 'duration_days',
              key: 'duration_days',
              className: 'router-table-col-status-narrow router-package-duration-cell',
              width: PACKAGE_LIST_COLUMN_WIDTHS.durationDays,
              render: (value) => Number(value || 0) || '-',
            },
            {
              title: t('package_manage.table.status'),
              dataIndex: 'enabled',
              key: 'enabled',
              className: 'router-table-col-status-narrow router-package-status-cell',
              width: PACKAGE_LIST_COLUMN_WIDTHS.status,
              render: (_, row) => (
                <div
                  onClick={(e) => {
                    e.stopPropagation();
                  }}
                >
                  <AppSwitch
                    checked={Boolean(row.enabled)}
                    disabled={submitting}
                    onChange={(_, { checked }) => toggleEnabled(row, Boolean(checked))}
                  />
                </div>
              ),
            },
            {
              title: t('package_manage.table.created_at'),
              dataIndex: 'created_at',
              key: 'created_at',
              className: 'router-table-col-datetime router-package-created-at-cell',
              width: PACKAGE_LIST_COLUMN_WIDTHS.createdAt,
              sorter: (a, b) => Number(a.created_at || 0) - Number(b.created_at || 0),
              defaultSortOrder: 'descend',
              render: (value) => (value ? timestamp2string(value) : '-'),
            },
            {
              title: t('package_manage.table.updated_at'),
              dataIndex: 'updated_at',
              key: 'updated_at',
              className: 'router-table-col-datetime router-package-updated-at-cell',
              width: PACKAGE_LIST_COLUMN_WIDTHS.updatedAt,
              sorter: (a, b) => Number(a.updated_at || 0) - Number(b.updated_at || 0),
              render: (value) => (value ? timestamp2string(value) : '-'),
            },
            {
              title: t('package_manage.table.actions'),
              key: 'actions',
              className: 'router-table-col-actions-icon router-package-action-cell',
              width: 52,
              render: (_, row) => (
                <div
                  className='router-action-group-tight router-table-actions-icon-compact router-nowrap'
                  onClick={(e) => {
                    e.stopPropagation();
                  }}
                >
                  <AppTableActionButton
                    icon='trash'
                    title={t('package_manage.buttons.delete')}
                    color='red'
                    disabled={submitting}
                    onClick={() => {
                      openDeleteModal(row);
                    }}
                  />
                </div>
              ),
            },
          ]}
        >
        </AppTable>
      </div>

      {totalPages > 1 ? (
        <div className='router-pagination-wrap-md'>
          <AppPagination
            className='router-section-pagination'
            current={activePage}
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
    <div>
      <AppFormRow className='router-modal-form-row'>
        <AppField label={t('package_manage.form.name')} required>
          <AppInput
            className='router-section-input'
            placeholder={t('package_manage.form.name_placeholder')}
            value={form.name}
            onChange={(e, { value }) =>
              setForm((prev) => ({ ...prev, name: value || '' }))
            }
          />
        </AppField>
        <AppField label={t('package_manage.form.group')}>
          <AppSelect
            className='router-section-input'
            placeholder={t('package_manage.form.group_placeholder')}
            options={groupOptions}
            value={form.group_id}
            loading={groupLoading}
            onChange={(e, { value }) =>
              setForm((prev) => ({ ...prev, group_id: (value || '').toString() }))
            }
          />
        </AppField>
      </AppFormRow>

      <AppFormRow className='router-modal-form-row'>
        <AppField label={t('package_manage.form.description')}>
          <AppTextarea
            className='router-section-input'
            value={form.description}
            onChange={(e, { value }) =>
              setForm((prev) => ({ ...prev, description: (value || '').toString() }))
            }
          />
        </AppField>
      </AppFormRow>

      <AppFormRow className='router-modal-form-row'>
        <AppField label={t('package_manage.form.sale_price')}>
          <AppCompact className='router-section-input-with-unit' block>
            <AppInputNumber
              className='router-section-input router-section-input-with-unit-field'
              min={0}
              step={0.01}
              precision={2}
              fluid
              value={form.sale_price}
              onChange={(e, { value }) =>
                setForm((prev) => ({ ...prev, sale_price: value ?? '0' }))
              }
            />
            <UnitDropdown
              variant='inputUnit'
              options={saleCurrencyOptions}
              value={form.sale_currency || 'CNY'}
              onChange={(_, { value }) =>
                setForm((prev) => ({
                  ...prev,
                  sale_currency: (value || 'CNY').toString().trim().toUpperCase(),
                }))
              }
              aria-label={t('package_manage.form.sale_currency')}
            />
          </AppCompact>
        </AppField>
        <AppField />
      </AppFormRow>

      <AppFormRow className='router-modal-form-row'>
        <AppField label={t('package_manage.form.daily_quota_limit')}>
          <AppCompact className='router-section-input-with-unit' block>
            <AppInputNumber
              className='router-section-input router-section-input-with-unit-field'
              value={form.daily_amount}
              step={resolveBillingInputStep(form.daily_amount_unit, currencyIndex)}
              min={0}
              precision={6}
              fluid
              onChange={(e, { value }) =>
                setForm((prev) => ({ ...prev, daily_amount: value ?? '0' }))
              }
            />
            <UnitDropdown
              variant='inputUnit'
              bordered={false}
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
          </AppCompact>
        </AppField>
        <AppField label={t('package_manage.form.package_emergency_quota_limit')}>
          <AppCompact className='router-section-input-with-unit' block>
            <AppInputNumber
              className='router-section-input router-section-input-with-unit-field'
              value={form.emergency_amount}
              step={resolveBillingInputStep(
                form.emergency_amount_unit,
                currencyIndex
              )}
              min={0}
              precision={6}
              fluid
              onChange={(e, { value }) =>
                setForm((prev) => ({
                  ...prev,
                  emergency_amount: value ?? '0',
                }))
              }
            />
            <UnitDropdown
              variant='inputUnit'
              bordered={false}
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
          </AppCompact>
        </AppField>
      </AppFormRow>

      <AppFormRow className='router-modal-form-row'>
        <AppField label={t('package_manage.form.duration_days')}>
          <AppInputNumber
            className='router-section-input'
            min={1}
            step={1}
            precision={0}
            fluid
            value={form.duration_days}
            onChange={(e, { value }) =>
              setForm((prev) => ({ ...prev, duration_days: value || 0 }))
            }
          />
        </AppField>
        <AppField label={t('package_manage.form.quota_reset_timezone')}>
          <AppInput
            className='router-section-input'
            value={form.reset_timezone}
            onChange={(e, { value }) =>
              setForm((prev) => ({ ...prev, reset_timezone: value || '' }))
            }
          />
        </AppField>
      </AppFormRow>

      <AppFormRow className='router-modal-form-row'>
        <AppField label={t('package_manage.form.enabled')}>
          <AppSwitch
            checked={Boolean(form.enabled)}
            onChange={(e, { checked }) =>
              setForm((prev) => ({ ...prev, enabled: Boolean(checked) }))
            }
          />
        </AppField>
        <AppField />
      </AppFormRow>

      <AppFormRow className='router-modal-form-row'>
        <AppField label={t('package_manage.form.source')}>
          <AppInput
            className='router-section-input'
            value={form.source}
            onChange={(e, { value }) =>
              setForm((prev) => ({ ...prev, source: value || '' }))
            }
          />
        </AppField>
      </AppFormRow>
    </div>
  );

  return (
    <div>
      {renderTable()}

      <AppModal
        open={createOpen}
        onClose={closeAllModals}
        size='small'
        title={t('package_manage.dialog.create_title')}
        footer={null}
      >
        <div className='router-page-stack'>
          {renderFormFields()}
          <AppFormActions>
            <AppButton type='button' onClick={closeAllModals} disabled={submitting}>
              {t('common.cancel')}
            </AppButton>
            <AppButton type='button' color='blue' loading={submitting} onClick={submitCreate}>
              {t('common.confirm')}
            </AppButton>
          </AppFormActions>
        </div>
      </AppModal>

      <AppModal
        open={editOpen}
        onClose={closeAllModals}
        size='small'
        title={t('package_manage.dialog.edit_title')}
        footer={null}
      >
        <div className='router-page-stack'>
          {renderFormFields()}
          <AppFormActions>
            <AppButton type='button' onClick={closeAllModals} disabled={submitting}>
              {t('common.cancel')}
            </AppButton>
            <AppButton type='button' color='blue' loading={submitting} onClick={submitEdit}>
              {t('common.confirm')}
            </AppButton>
          </AppFormActions>
        </div>
      </AppModal>

      <AppModal
        open={deleteOpen}
        onClose={closeAllModals}
        size='tiny'
        title={t('package_manage.dialog.delete_title')}
        footer={null}
      >
        <div className='router-page-stack'>
          <div>
            {t('package_manage.dialog.delete_content', {
              name: activeRow?.name || '-',
            })}
          </div>
          <AppFormActions>
            <AppButton type='button' onClick={closeAllModals} disabled={submitting}>
              {t('common.cancel')}
            </AppButton>
            <AppButton type='button' color='red' loading={submitting} onClick={submitDelete}>
              {t('common.confirm')}
            </AppButton>
          </AppFormActions>
        </div>
      </AppModal>
    </div>
  );
};

export default PackagesManager;
