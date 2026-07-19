import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { ITEMS_PER_PAGE } from '../../constants';
import { API, showError, showSuccess, timestamp2string } from '../../helpers';
import { formatDecimalNumber } from '../../helpers/render';
import {
  SERVICE_PACKAGE_PERIOD_DAILY,
  SERVICE_PACKAGE_PERIOD_MONTHLY,
  SERVICE_PACKAGE_PERIOD_PACKAGE_TOTAL,
  SERVICE_PACKAGE_PERIOD_WEEKLY,
  SERVICE_PACKAGE_QUOTA_METRIC_REQUEST_COUNT,
  SERVICE_PACKAGE_QUOTA_METRIC_YYC,
} from '../../helpers/package';
import {
  AppButton,
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
  AppTag,
  AppTextarea,
} from '../../router-ui';
import { normalizeSupportedModels } from '../TopUp/shared.jsx';

const PRODUCT_KIND_BALANCE = 'balance';
const PRODUCT_KIND_SUBSCRIPTION = 'subscription';

const PRODUCT_KIND_OPTIONS = [
  { key: 'all', value: '', text: '全部类型' },
  { key: PRODUCT_KIND_BALANCE, value: PRODUCT_KIND_BALANCE, text: '充值' },
  { key: PRODUCT_KIND_SUBSCRIPTION, value: PRODUCT_KIND_SUBSCRIPTION, text: '订阅' },
];

const PRODUCT_LIST_TABLE_MIN_WIDTH = 1000;
const PRODUCT_FORM_KIND_OPTIONS = PRODUCT_KIND_OPTIONS.filter((item) => item.value);
const QUOTA_METRIC_OPTIONS = [
  { key: SERVICE_PACKAGE_QUOTA_METRIC_YYC, value: SERVICE_PACKAGE_QUOTA_METRIC_YYC, text: 'YYC 额度' },
  { key: SERVICE_PACKAGE_QUOTA_METRIC_REQUEST_COUNT, value: SERVICE_PACKAGE_QUOTA_METRIC_REQUEST_COUNT, text: '请求次数' },
];
const PERIOD_TYPE_OPTIONS = [
  { key: SERVICE_PACKAGE_PERIOD_MONTHLY, value: SERVICE_PACKAGE_PERIOD_MONTHLY, text: '每月' },
  { key: SERVICE_PACKAGE_PERIOD_WEEKLY, value: SERVICE_PACKAGE_PERIOD_WEEKLY, text: '每周' },
  { key: SERVICE_PACKAGE_PERIOD_DAILY, value: SERVICE_PACKAGE_PERIOD_DAILY, text: '每天' },
  { key: SERVICE_PACKAGE_PERIOD_PACKAGE_TOTAL, value: SERVICE_PACKAGE_PERIOD_PACKAGE_TOTAL, text: '套餐总量' },
];
const VISIBILITY_OPTIONS = [
  { key: 'all', value: 'all', text: '全部用户' },
  { key: 'partial_users', value: 'partial_users', text: '部分用户' },
];

const createEmptyForm = () => ({
  id: '',
  kind: PRODUCT_KIND_BALANCE,
  name: '',
  description: '',
  group_id: '',
  sale_price: 0,
  sale_currency: 'CNY',
  quota_metric: SERVICE_PACKAGE_QUOTA_METRIC_YYC,
  quota_amount: 0,
  quota_currency: 'YYC',
  period_type: SERVICE_PACKAGE_PERIOD_MONTHLY,
  period_limit: 0,
  duration_days: 30,
  validity_days: 0,
  max_concurrency_per_user: 0,
  max_concurrency_per_package: 0,
  allow_balance_fallback: false,
  visibility_scope: 'all',
  visible_user_ids: [],
  enabled: true,
  sort_order: 0,
  source: 'manual',
});

const getProductKindLabel = (kind) =>
  kind === PRODUCT_KIND_SUBSCRIPTION ? '订阅' : '充值';

const formatAmount = (amount, currency) => {
  const normalizedCurrency = (currency || '').toString().trim().toUpperCase();
  const displayAmount = formatDecimalNumber(amount || 0, 6);
  return normalizedCurrency ? `${normalizedCurrency} ${displayAmount}` : displayAmount;
};

const formatDuration = (row, t) => {
  const days = Number(row?.validity_days || row?.duration_days || 0) || 0;
  if (days <= 0) {
    return row?.kind === PRODUCT_KIND_BALANCE ? t('common.never') : '-';
  }
  return `${days} ${t('common.day')}`;
};

const formatVisibility = (row) =>
  row?.visibility_scope === 'partial_users' ? '部分用户' : '全部用户';

const SupportedModelsCount = ({ models, onOpen }) => {
  const normalizedModels = useMemo(
    () => normalizeSupportedModels(models),
    [models],
  );
  if (normalizedModels.length === 0) {
    return 0;
  }
  return (
    <button
      type='button'
      className='router-link-button router-link-inline'
      onClick={(event) => {
        event.stopPropagation();
        onOpen?.(normalizedModels);
      }}
    >
      {normalizedModels.length}
    </button>
  );
};

const toGroupOptions = (items) =>
  (Array.isArray(items) ? items : [])
    .map((item) => {
      const value = (item?.id || '').toString().trim();
      if (!value) {
        return null;
      }
      return {
        key: value,
        value,
        text: (item?.name || value).toString(),
      };
    })
    .filter(Boolean);

const toUserOption = (item) => {
  const id = (item?.id || '').toString().trim();
  const username = (item?.username || '').toString().trim();
  const displayName = (item?.display_name || '').toString().trim();
  const walletAddress = (item?.wallet_address || '').toString().trim();
  const primaryName = displayName || username;
  return {
    key: id,
    value: id,
    text: [primaryName, walletAddress].filter(Boolean).join(' / ') || id,
  };
};

const appendUserOptionsIfMissing = (options, users) => {
  const currentOptions = Array.isArray(options) ? options : [];
  const nextOptions = [...currentOptions];
  const seen = new Set(
    currentOptions.map((item) => (item?.value || '').toString().trim()).filter(Boolean),
  );
  (Array.isArray(users) ? users : []).forEach((item) => {
    const option = toUserOption(item);
    const value = (option?.value || '').toString().trim();
    if (!value || seen.has(value)) {
      return;
    }
    seen.add(value);
    nextOptions.push(option);
  });
  return nextOptions;
};

const buildProductPayload = (form) => {
  const kind = form.kind === PRODUCT_KIND_SUBSCRIPTION ? PRODUCT_KIND_SUBSCRIPTION : PRODUCT_KIND_BALANCE;
  const quotaAmount = Number(form.quota_amount || 0) || 0;
  const periodLimit = Number(form.period_limit || 0) || 0;
  return {
    id: form.id || undefined,
    kind,
    name: (form.name || '').toString().trim(),
    description: (form.description || '').toString().trim(),
    group_id: (form.group_id || '').toString().trim(),
    sale_price: Number(form.sale_price || 0) || 0,
    sale_currency: (form.sale_currency || 'CNY').toString().trim().toUpperCase(),
    quota_metric: kind === PRODUCT_KIND_SUBSCRIPTION
      ? form.quota_metric || SERVICE_PACKAGE_QUOTA_METRIC_YYC
      : SERVICE_PACKAGE_QUOTA_METRIC_YYC,
    quota_amount: kind === PRODUCT_KIND_SUBSCRIPTION ? (periodLimit || quotaAmount) : quotaAmount,
    quota_currency: (form.quota_currency || 'YYC').toString().trim().toUpperCase(),
    period_type: kind === PRODUCT_KIND_SUBSCRIPTION
      ? form.period_type || SERVICE_PACKAGE_PERIOD_MONTHLY
      : 'none',
    period_limit: kind === PRODUCT_KIND_SUBSCRIPTION ? (periodLimit || quotaAmount) : 0,
    duration_days: kind === PRODUCT_KIND_SUBSCRIPTION
      ? Number(form.duration_days || 0) || 30
      : Number(form.validity_days || 0) || 0,
    validity_days: kind === PRODUCT_KIND_BALANCE
      ? Number(form.validity_days || 0) || 0
      : Number(form.duration_days || 0) || 30,
    max_concurrency_per_user: Number(form.max_concurrency_per_user || 0) || 0,
    max_concurrency_per_package: Number(form.max_concurrency_per_package || 0) || 0,
    allow_balance_fallback: kind === PRODUCT_KIND_SUBSCRIPTION
      ? Boolean(form.allow_balance_fallback)
      : false,
    visibility_scope: form.visibility_scope || 'all',
    visible_user_ids: Array.isArray(form.visible_user_ids) ? form.visible_user_ids : [],
    enabled: form.enabled !== false,
    sort_order: Number(form.sort_order || 0) || 0,
    source: form.source || 'manual',
  };
};

const Entitlement = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [rows, setRows] = useState([]);
  const [loading, setLoading] = useState(false);
  const [activePage, setActivePage] = useState(1);
  const [total, setTotal] = useState(0);
  const [kind, setKind] = useState('');
  const [searchKeyword, setSearchKeyword] = useState('');
  const [groupOptions, setGroupOptions] = useState([]);
  const [groupLoading, setGroupLoading] = useState(false);
  const [userOptions, setUserOptions] = useState([]);
  const [userLoading, setUserLoading] = useState(false);
  const [formOpen, setFormOpen] = useState(false);
  const [form, setForm] = useState(createEmptyForm);
  const [submitting, setSubmitting] = useState(false);
  const [deleteRow, setDeleteRow] = useState(null);
  const [modelsDialog, setModelsDialog] = useState({
    open: false,
    title: '',
    models: [],
    keyword: '',
  });

  const normalizedKeyword = searchKeyword.trim();
  const totalPages = Math.max(
    1,
    Math.ceil((Number(total || 0) || 0) / ITEMS_PER_PAGE),
  );

  const loadProducts = useCallback(async () => {
    setLoading(true);
    try {
      const response = await API.get('/api/v1/admin/entitlement/products', {
        params: {
          page: activePage,
          page_size: ITEMS_PER_PAGE,
          kind,
          keyword: normalizedKeyword,
        },
      });
      const payload = response.data || {};
      if (!payload.success) {
        showError(payload.message || t('common.failed'));
        return;
      }
      const data = payload.data || {};
      setRows(Array.isArray(data.items) ? data.items : []);
      setTotal(Number(data.total || 0) || 0);
    } catch (error) {
      showError(error.message || t('common.failed'));
    } finally {
      setLoading(false);
    }
  }, [activePage, kind, normalizedKeyword, t]);

  useEffect(() => {
    loadProducts();
  }, [loadProducts]);

  const loadGroups = useCallback(async () => {
    setGroupLoading(true);
    try {
      const items = [];
      let page = 1;
      while (page <= 50) {
        const response = await API.get('/api/v1/admin/groups', {
          params: { page, page_size: 100 },
        });
        const payload = response.data || {};
        if (!payload.success) {
          showError(payload.message || t('common.failed'));
          return;
        }
        const pageItems = Array.isArray(payload.data?.items) ? payload.data.items : [];
        items.push(...pageItems);
        const totalItems = Number(payload.data?.total || pageItems.length || 0);
        if (pageItems.length === 0 || items.length >= totalItems || pageItems.length < 100) {
          break;
        }
        page += 1;
      }
      setGroupOptions(toGroupOptions(items));
    } catch (error) {
      showError(error.message || t('common.failed'));
    } finally {
      setGroupLoading(false);
    }
  }, [t]);

  useEffect(() => {
    loadGroups();
  }, [loadGroups]);

  const loadInitialUsers = useCallback(async () => {
    setUserLoading(true);
    try {
      const response = await API.get('/api/v1/admin/user', {
        params: { page: 1 },
      });
      const payload = response.data || {};
      if (!payload.success) {
        showError(payload.message || t('common.failed'));
        return;
      }
      setUserOptions((current) => appendUserOptionsIfMissing(current, payload.data));
    } catch (error) {
      showError(error.message || t('common.failed'));
    } finally {
      setUserLoading(false);
    }
  }, [t]);

  const searchUsers = useCallback(
    async (keyword) => {
      const normalizedKeyword = (keyword || '').toString().trim();
      if (!normalizedKeyword) {
        return;
      }
      setUserLoading(true);
      try {
        const response = await API.get('/api/v1/admin/user/search', {
          params: { keyword: normalizedKeyword },
        });
        const payload = response.data || {};
        if (!payload.success) {
          showError(payload.message || t('common.failed'));
          return;
        }
        setUserOptions((current) => appendUserOptionsIfMissing(current, payload.data));
      } catch (error) {
        showError(error.message || t('common.failed'));
      } finally {
        setUserLoading(false);
      }
    },
    [t],
  );

  useEffect(() => {
    loadInitialUsers();
  }, [loadInitialUsers]);

  const openModelsDialog = useCallback((row, models) => {
    setModelsDialog({
      open: true,
      title: row?.name || '-',
      models: Array.isArray(models) ? models : [],
      keyword: '',
    });
  }, []);

  const closeModelsDialog = useCallback(() => {
    setModelsDialog({
      open: false,
      title: '',
      models: [],
      keyword: '',
    });
  }, []);

  const filteredDialogModels = useMemo(() => {
    const dialogModels = Array.isArray(modelsDialog.models) ? modelsDialog.models : [];
    const keywordText = (modelsDialog.keyword || '').toString().trim().toLowerCase();
    if (!keywordText) {
      return dialogModels;
    }
    return dialogModels.filter((modelName) =>
      modelName.toLowerCase().includes(keywordText),
    );
  }, [modelsDialog.keyword, modelsDialog.models]);

  const openCreate = useCallback(() => {
    setForm(createEmptyForm());
    setFormOpen(true);
  }, []);

  const submitForm = useCallback(async () => {
    setSubmitting(true);
    try {
      const payload = buildProductPayload(form);
      const response = payload.id
        ? await API.put(`/api/v1/admin/entitlement/products/${encodeURIComponent(payload.id)}`, payload)
        : await API.post('/api/v1/admin/entitlement/products', payload);
      const data = response.data || {};
      if (!data.success) {
        showError(data.message || t('common.failed'));
        return;
      }
      showSuccess('操作成功');
      setFormOpen(false);
      await loadProducts();
    } catch (error) {
      showError(error.message || t('common.failed'));
    } finally {
      setSubmitting(false);
    }
  }, [form, loadProducts, t]);

  const deleteProduct = useCallback(async () => {
    const id = (deleteRow?.id || '').toString().trim();
    if (!id) {
      setDeleteRow(null);
      return;
    }
    setSubmitting(true);
    try {
      const response = await API.delete(`/api/v1/admin/entitlement/products/${encodeURIComponent(id)}`);
      const data = response.data || {};
      if (!data.success) {
        showError(data.message || t('common.failed'));
        return;
      }
      showSuccess('操作成功');
      setDeleteRow(null);
      await loadProducts();
    } catch (error) {
      showError(error.message || t('common.failed'));
    } finally {
      setSubmitting(false);
    }
  }, [deleteRow, loadProducts, t]);

  const openDetail = useCallback(
    (row) => {
      const productID = (row?.id || '').toString().trim();
      if (!productID) {
        return;
      }
      if (row.kind === PRODUCT_KIND_SUBSCRIPTION) {
        navigate(`/admin/entitlement/package/detail/${encodeURIComponent(productID)}`);
        return;
      }
      navigate(`/admin/entitlement/topup/detail/${encodeURIComponent(productID)}`);
    },
    [navigate],
  );

  const columns = useMemo(
    () => [
      {
        title: '名称',
        dataIndex: 'name',
        key: 'name',
        width: 180,
        ellipsis: true,
        render: (value) => value || '-',
      },
      {
        title: '类型',
        dataIndex: 'kind',
        key: 'kind',
        width: 84,
        render: (value) => (
          <AppTag color={value === PRODUCT_KIND_SUBSCRIPTION ? 'blue' : 'green'}>
            {getProductKindLabel(value)}
          </AppTag>
        ),
      },
      {
        title: '分组',
        dataIndex: 'group_name',
        key: 'group',
        width: 150,
        ellipsis: true,
        render: (_, row) => row.group_name || row.group_id || '-',
      },
      {
        title: '适用模型',
        key: 'supported_models',
        width: 92,
        render: (_, row) => (
          <SupportedModelsCount
            models={row.supported_models}
            onOpen={(models) => openModelsDialog(row, models)}
          />
        ),
      },
      {
        title: '售价',
        key: 'sale_price',
        width: 130,
        render: (_, row) => formatAmount(row.sale_price, row.sale_currency || 'CNY'),
      },
      {
        title: '有效期',
        key: 'duration',
        width: 100,
        render: (_, row) => formatDuration(row, t),
      },
      {
        title: '可见范围',
        key: 'visibility_scope',
        width: 100,
        render: (_, row) => formatVisibility(row),
      },
      {
        title: '状态',
        dataIndex: 'enabled',
        key: 'enabled',
        width: 84,
        render: (value) => (
          <AppTag color={value ? 'green' : 'default'}>
            {value ? '启用' : '停用'}
          </AppTag>
        ),
      },
      {
        title: t('common.updated_at', '更新时间'),
        dataIndex: 'updated_at',
        key: 'updated_at',
        className: 'router-table-col-datetime',
        width: 168,
        render: (value) => (value ? timestamp2string(value) : '-'),
      },
      {
        title: t('common.operation'),
        key: 'action',
        className: 'router-table-col-actions-icon',
        width: 52,
        render: (_, row) => (
          <div
            className='router-action-group-tight router-table-actions-icon-compact'
            onClick={(event) => {
              event.stopPropagation();
            }}
          >
            <AppTableActionButton
              icon='trash'
              title={t('common.delete')}
              color='red'
              disabled={submitting}
              onClick={() => setDeleteRow(row)}
            />
          </div>
        ),
      },
    ],
    [openModelsDialog, submitting, t],
  );

  const renderForm = () => {
    const isSubscription = form.kind === PRODUCT_KIND_SUBSCRIPTION;
    const isRequestQuota = form.quota_metric === SERVICE_PACKAGE_QUOTA_METRIC_REQUEST_COUNT;
    return (
      <div className='router-page-stack'>
        <AppFormRow className='router-modal-form-row'>
          <AppField label='类型' required>
            <AppSelect
              className='router-section-input'
              options={PRODUCT_FORM_KIND_OPTIONS}
              value={form.kind}
              disabled={Boolean(form.id)}
              onChange={(_, { value }) =>
                setForm((current) => ({
                  ...current,
                  kind: value === PRODUCT_KIND_SUBSCRIPTION
                    ? PRODUCT_KIND_SUBSCRIPTION
                    : PRODUCT_KIND_BALANCE,
                }))
              }
            />
          </AppField>
          <AppField label='名称' required>
            <AppInput
              className='router-section-input'
              value={form.name}
              onChange={(_, { value }) =>
                setForm((current) => ({ ...current, name: value || '' }))
              }
            />
          </AppField>
        </AppFormRow>

        <AppFormRow className='router-modal-form-row'>
          <AppField label='分组' required>
            <AppSelect
              className='router-section-input'
              options={groupOptions}
              value={form.group_id}
              loading={groupLoading}
              search
              onChange={(_, { value }) =>
                setForm((current) => ({ ...current, group_id: value || '' }))
              }
            />
          </AppField>
          <AppField label='排序'>
            <AppInputNumber
              className='router-section-input'
              min={0}
              precision={0}
              fluid
              value={form.sort_order}
              onChange={(_, { value }) =>
                setForm((current) => ({ ...current, sort_order: Number(value || 0) }))
              }
            />
          </AppField>
        </AppFormRow>

        <AppFormRow className='router-modal-form-row'>
          <AppField label='说明'>
            <AppTextarea
              className='router-section-input'
              value={form.description}
              onChange={(_, { value }) =>
                setForm((current) => ({ ...current, description: value || '' }))
              }
            />
          </AppField>
        </AppFormRow>

        <AppFormRow className='router-modal-form-row'>
          <AppField label='售价' required>
            <AppInputNumber
              className='router-section-input'
              min={0}
              precision={2}
              step={0.01}
              fluid
              value={form.sale_price}
              onChange={(_, { value }) =>
                setForm((current) => ({ ...current, sale_price: Number(value || 0) }))
              }
            />
          </AppField>
          <AppField label='售价币种' required>
            <AppInput
              className='router-section-input'
              value={form.sale_currency}
              onChange={(_, { value }) =>
                setForm((current) => ({
                  ...current,
                  sale_currency: (value || '').toString().toUpperCase(),
                }))
              }
            />
          </AppField>
        </AppFormRow>

        {isSubscription ? (
          <AppFormRow className='router-modal-form-row'>
            <AppField label='权益类型' required>
              <AppSelect
                className='router-section-input'
                options={QUOTA_METRIC_OPTIONS}
                value={form.quota_metric}
                onChange={(_, { value }) =>
                  setForm((current) => ({
                    ...current,
                    quota_metric: value || SERVICE_PACKAGE_QUOTA_METRIC_YYC,
                    quota_currency:
                      value === SERVICE_PACKAGE_QUOTA_METRIC_REQUEST_COUNT
                        ? 'REQUEST'
                        : current.quota_currency,
                  }))
                }
              />
            </AppField>
            <AppField label='周期' required>
              <AppSelect
                className='router-section-input'
                options={PERIOD_TYPE_OPTIONS}
                value={form.period_type}
                onChange={(_, { value }) =>
                  setForm((current) => ({
                    ...current,
                    period_type: value || SERVICE_PACKAGE_PERIOD_MONTHLY,
                  }))
                }
              />
            </AppField>
          </AppFormRow>
        ) : null}

        <AppFormRow className='router-modal-form-row'>
          <AppField label={isSubscription ? '周期额度' : '到账额度'} required>
            <AppInputNumber
              className='router-section-input'
              min={0}
              precision={isSubscription && isRequestQuota ? 0 : 6}
              step={isSubscription && isRequestQuota ? 1 : 0.01}
              fluid
              value={isSubscription ? form.period_limit : form.quota_amount}
              onChange={(_, { value }) =>
                setForm((current) => ({
                  ...current,
                  period_limit: isSubscription ? Number(value || 0) : current.period_limit,
                  quota_amount: isSubscription ? current.quota_amount : Number(value || 0),
                }))
              }
            />
          </AppField>
          <AppField label='额度币种'>
            <AppInput
              className='router-section-input'
              value={form.quota_currency}
              disabled={isSubscription && isRequestQuota}
              onChange={(_, { value }) =>
                setForm((current) => ({
                  ...current,
                  quota_currency: (value || '').toString().toUpperCase(),
                }))
              }
            />
          </AppField>
        </AppFormRow>

        <AppFormRow className='router-modal-form-row'>
          <AppField label={isSubscription ? '订阅天数' : '有效天数'}>
            <AppInputNumber
              className='router-section-input'
              min={0}
              precision={0}
              step={1}
              fluid
              value={isSubscription ? form.duration_days : form.validity_days}
              onChange={(_, { value }) =>
                setForm((current) => ({
                  ...current,
                  duration_days: isSubscription ? Number(value || 0) : current.duration_days,
                  validity_days: isSubscription ? current.validity_days : Number(value || 0),
                }))
              }
            />
          </AppField>
          <AppField label='可见范围'>
            <AppSelect
              className='router-section-input'
              options={VISIBILITY_OPTIONS}
              value={form.visibility_scope || 'all'}
              onChange={(_, { value }) =>
                setForm((current) => ({ ...current, visibility_scope: value || 'all' }))
              }
            />
          </AppField>
        </AppFormRow>

        {form.visibility_scope === 'partial_users' ? (
          <AppFormRow className='router-modal-form-row'>
            <AppField label='可见用户'>
              <AppSelect
                className='router-section-input'
                options={userOptions}
                value={form.visible_user_ids}
                loading={userLoading}
                multiple
                search
                filterOption={false}
                onSearch={searchUsers}
                onChange={(_, { value }) =>
                  setForm((current) => ({
                    ...current,
                    visible_user_ids: Array.isArray(value) ? value : [],
                  }))
                }
              />
            </AppField>
          </AppFormRow>
        ) : null}

        <AppFormRow className='router-modal-form-row'>
          <AppField label='单用户并发'>
            <AppInputNumber
              className='router-section-input'
              min={0}
              precision={0}
              step={1}
              fluid
              value={form.max_concurrency_per_user}
              onChange={(_, { value }) =>
                setForm((current) => ({
                  ...current,
                  max_concurrency_per_user: Number(value || 0),
                }))
              }
            />
          </AppField>
          <AppField label='总并发'>
            <AppInputNumber
              className='router-section-input'
              min={0}
              precision={0}
              step={1}
              fluid
              value={form.max_concurrency_per_package}
              onChange={(_, { value }) =>
                setForm((current) => ({
                  ...current,
                  max_concurrency_per_package: Number(value || 0),
                }))
              }
            />
          </AppField>
        </AppFormRow>

        <AppFormRow className='router-modal-form-row'>
          <AppField label='余额兜底'>
            <AppSwitch
              checked={isSubscription && Boolean(form.allow_balance_fallback)}
              disabled={!isSubscription}
              onChange={(_, { checked }) =>
                setForm((current) => ({
                  ...current,
                  allow_balance_fallback: Boolean(checked),
                }))
              }
            />
          </AppField>
          <AppField label='启用'>
            <AppSwitch
              checked={form.enabled !== false}
              onChange={(_, { checked }) =>
                setForm((current) => ({ ...current, enabled: Boolean(checked) }))
              }
            />
          </AppField>
        </AppFormRow>

        <AppFormActions>
          <AppButton
            type='button'
            className='router-page-button'
            onClick={() => setFormOpen(false)}
            disabled={submitting}
          >
            {t('common.cancel')}
          </AppButton>
          <AppButton
            type='button'
            className='router-page-button'
            color='blue'
            onClick={submitForm}
            loading={submitting}
          >
            {t('common.confirm')}
          </AppButton>
        </AppFormActions>
      </div>
    );
  };

  return (
    <div className='dashboard-container'>
      <AppFilterHeader
        className='router-block-gap-md'
        breadcrumbs={[
          { key: 'admin', label: t('header.admin_workspace') },
          { key: 'model', label: t('header.model') },
          { key: 'entitlement', label: t('header.entitlement'), active: true },
        ]}
        meta={
          <button
            type='button'
            className='router-breadcrumb-link router-page-header-link'
            onClick={() => navigate('/admin/entitlement/purchase-records')}
          >
            购买记录
          </button>
        }
        metaClassName='router-page-header-meta-links'
        query={
          <>
            <AppSelect
              className='router-search-form-xs'
              options={PRODUCT_KIND_OPTIONS}
              value={kind}
              onChange={(_, { value }) => {
                setKind((value || '').toString());
                setActivePage(1);
              }}
            />
            <AppInput
              className='router-section-input router-search-form-sm'
              placeholder='搜索名称、说明、分组'
              value={searchKeyword}
              onChange={(_, { value }) => {
                setSearchKeyword(value || '');
                setActivePage(1);
              }}
            />
          </>
        }
        actions={
          <div className='router-list-toolbar-actions'>
            <AppButton
              type='button'
              className='router-page-button'
              color='blue'
              onClick={openCreate}
              disabled={submitting}
            >
              {t('common.add')}
            </AppButton>
            <AppButton
              type='button'
              className='router-page-button'
              onClick={loadProducts}
              loading={loading}
              disabled={submitting}
            >
              {t('common.refresh')}
            </AppButton>
          </div>
        }
      />

      <div className='router-table-scroll-x'>
        <AppTable
          className='router-hover-table router-list-table router-table-fit-page'
          pagination={false}
          scroll={{ x: PRODUCT_LIST_TABLE_MIN_WIDTH }}
          rowKey='id'
          dataSource={rows}
          loading={loading}
          locale={{
            emptyText: loading ? t('common.loading') : t('common.no_data', '暂无数据'),
          }}
          onRow={(row) => ({
            className: row?.id ? 'router-row-clickable' : '',
            onClick: () => openDetail(row),
          })}
          columns={columns}
        />
      </div>

      {totalPages > 1 ? (
        <div className='router-pagination-wrap-md'>
          <AppPagination
            className='router-section-pagination'
            current={activePage}
            totalPages={totalPages}
            onPageChange={(_, { activePage: nextActivePage }) => {
              setActivePage(Number(nextActivePage) || 1);
            }}
          />
        </div>
      ) : null}

      <AppModal
        open={formOpen}
        size='large'
        title='新增权益'
        onClose={() => setFormOpen(false)}
        footer={null}
      >
        {renderForm()}
      </AppModal>

      <AppModal
        open={Boolean(deleteRow)}
        size='tiny'
        title='删除权益'
        onClose={() => setDeleteRow(null)}
        footer={null}
      >
        <div className='router-page-stack'>
          <div>确认删除 {deleteRow?.name || '-'}？</div>
          <AppFormActions>
            <AppButton
              type='button'
              className='router-page-button'
              onClick={() => setDeleteRow(null)}
              disabled={submitting}
            >
              {t('common.cancel')}
            </AppButton>
            <AppButton
              type='button'
              className='router-page-button'
              color='red'
              onClick={deleteProduct}
              loading={submitting}
            >
              {t('common.delete')}
            </AppButton>
          </AppFormActions>
        </div>
      </AppModal>

      <AppModal
        open={modelsDialog.open}
        size='small'
        title={t('topup.manage.columns.applicable_models')}
        onClose={closeModelsDialog}
        footer={[
          <AppButton key='close' onClick={closeModelsDialog}>
            {t('common.close')}
          </AppButton>,
        ]}
      >
        <div className='router-supported-models-dialog'>
          <div className='router-detail-value'>{modelsDialog.title}</div>
          <AppInput
            className='router-supported-models-search'
            type='search'
            value={modelsDialog.keyword}
            placeholder={t('topup.pricing.supported_models_search_placeholder')}
            onChange={(_, { value }) =>
              setModelsDialog((current) => ({ ...current, keyword: value || '' }))
            }
          />
          <div className='router-supported-models-dialog-meta'>
            {t('topup.pricing.supported_models_dialog_count', {
              count: filteredDialogModels.length,
              total: modelsDialog.models.length,
            })}
          </div>
          {filteredDialogModels.length === 0 ? (
            <div className='router-text-muted router-supported-models-empty'>
              {t('topup.pricing.supported_models_search_empty')}
            </div>
          ) : (
            <div className='router-supported-models-dialog-list'>
              {filteredDialogModels.map((modelName, index) => (
                <div
                  key={`${modelName}-${index}`}
                  className='router-supported-models-dialog-item'
                >
                  {modelName}
                </div>
              ))}
            </div>
          )}
        </div>
      </AppModal>
    </div>
  );
};

export default Entitlement;
