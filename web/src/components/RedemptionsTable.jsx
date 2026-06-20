import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import {
  API,
  copy,
  showError,
  showSuccess,
  showWarning,
  timestamp2string,
} from '../helpers';

import { ITEMS_PER_PAGE } from '../constants';
import {
  REDEMPTION_LIST_COLUMN_WIDTHS,
  REDEMPTION_LIST_TABLE_MIN_WIDTH,
} from '../constants/tableWidthPresets';
import {
  buildBillingCurrencyIndex,
  buildDisplayUnitOptions,
} from '../helpers/billing';
import {
  formatDecimalNumber,
} from '../helpers/render';
import UnitDropdown from './UnitDropdown';
import {
  AppButton,
  AppFilterHeader,
  AppInput,
  AppPagination,
  AppPopconfirm,
  AppTable,
  AppTableActionButton,
  AppTag,
} from '../router-ui';

const compareTextValue = (left, right) =>
  String(left || '').localeCompare(String(right || ''));

const compareNumberValue = (left, right) =>
  Number(left || 0) - Number(right || 0);

function renderTimestamp(timestamp) {
  return <>{timestamp2string(timestamp)}</>;
}

function renderExpiryTime(timestamp, t) {
  const normalized = Number(timestamp || 0);
  if (!Number.isFinite(normalized) || normalized <= 0) {
    return t('common.never');
  }
  return renderTimestamp(normalized);
}

function renderGroupLabel(redemption) {
  const groupName = (redemption?.group_name || '').toString().trim();
  if (groupName) {
    return groupName;
  }
  const groupID = (redemption?.group_id || '').toString().trim();
  return groupID || '-';
}

function formatByCurrencyMinorUnit(amount, currency) {
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
}

function normalizeRedemptionRow(row) {
  return {
    ...(row || {}),
    // Prefer YYC-native fields, fall back to historical quota payloads.
    creditedChargeAmount: Number(row?.credit_amount ?? row?.quota ?? 0),
    groupLabel: renderGroupLabel(row),
    createdTime: Number(row?.created_time ?? 0),
    redeemedTime: Number(row?.redeemed_time ?? 0),
  };
}

function buildDisplayValue(redemption, displayUnit, currencyIndex) {
  // Keep legacy quota fallback for older redemption records.
  const creditedChargeAmount = Number(redemption?.creditedChargeAmount ?? redemption?.credit_amount ?? redemption?.quota ?? 0);
  const targetCurrency = currencyIndex[displayUnit] || currencyIndex.YYC;
  const rate = Number(targetCurrency?.charge_rate || 0);
  if (!Number.isFinite(rate) || rate <= 0) {
    return '-';
  }
  return formatByCurrencyMinorUnit(creditedChargeAmount / rate, targetCurrency);
}

function renderDisplayFaceValue(redemption, displayUnit, currencyIndex) {
  return buildDisplayValue(redemption, displayUnit, currencyIndex);
}

function renderStatus(status, t) {
  switch (status) {
    case 1:
      return (
        <AppTag color='green' className='router-tag'>
          {t('redemption.status.unused')}
        </AppTag>
      );
    case 2:
      return (
        <AppTag color='red' className='router-tag'>
          {t('redemption.status.disabled')}
        </AppTag>
      );
    case 3:
      return (
        <AppTag color='grey' className='router-tag'>
          {t('redemption.status.used')}
        </AppTag>
      );
    default:
      return (
        <AppTag color='black' className='router-tag'>
          {t('redemption.status.unknown')}
        </AppTag>
      );
  }
}

const RedemptionsTable = () => {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const currentPagePath = `${location.pathname}${location.search}${location.hash}`;
  const [redemptions, setRedemptions] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activePage, setActivePage] = useState(1);
  const [totalCount, setTotalCount] = useState(0);
  const [isSearchMode, setIsSearchMode] = useState(false);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [searching, setSearching] = useState(false);
  const [tableSorter, setTableSorter] = useState({
    columnKey: 'created_time',
    order: 'descend',
  });
  const [displayUnit, setDisplayUnit] = useState('USD');
  const [currencyIndex, setCurrencyIndex] = useState(
    buildBillingCurrencyIndex([], { placeholderCodes: ['USD', 'CNY'] })
  );

  const displayUnitOptions = useMemo(
    () => buildDisplayUnitOptions(currencyIndex, { order: 'charge-first' }),
    [currencyIndex]
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

  const loadRedemptions = useCallback(async (page) => {
    const normalizedPage = Number(page) > 0 ? Number(page) : 1;
    const res = await API.get(`/api/v1/admin/redemption/?page=${normalizedPage}`);
    const { success, message, data, meta } = res.data;
    if (success) {
      setIsSearchMode(false);
      setTotalCount(Number(meta?.total || data?.length || 0));
      const nextRows = (Array.isArray(data) ? data : []).map(normalizeRedemptionRow);
      if (normalizedPage === 1) {
        setRedemptions(nextRows);
      } else {
        setRedemptions((prev) => {
          const next = [...prev];
          next.splice(
            (normalizedPage - 1) * ITEMS_PER_PAGE,
            nextRows.length,
            ...nextRows,
          );
          return next;
        });
      }
    } else {
      showError(message);
    }
    setLoading(false);
  }, []);

  const onPaginationChange = (e, { activePage }) => {
    (async () => {
      const nextPage = Number(activePage) > 0 ? Number(activePage) : 1;
      const hasLoadedPageRows = redemptions
        .slice((nextPage - 1) * ITEMS_PER_PAGE, nextPage * ITEMS_PER_PAGE)
        .some(Boolean);
      if (!isSearchMode && !hasLoadedPageRows) {
        await loadRedemptions(nextPage);
      }
      setActivePage(nextPage);
    })();
  };

  useEffect(() => {
    loadRedemptions(1)
      .then()
      .catch((reason) => {
        showError(reason);
      });
  }, [loadRedemptions]);

  useEffect(() => {
    loadDisplayUnits().then();
  }, [loadDisplayUnits]);

  const manageRedemption = async (id, action, idx) => {
    let data = { id };
    let res;
    switch (action) {
      case 'delete':
        res = await API.delete(`/api/v1/admin/redemption/${id}/`);
        break;
      case 'enable':
        data.status = 1;
        res = await API.put('/api/v1/admin/redemption/?status_only=true', data);
        break;
      case 'disable':
        data.status = 2;
        res = await API.put('/api/v1/admin/redemption/?status_only=true', data);
        break;
      default:
        return;
    }
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('token.messages.operation_success'));
      let redemption = res.data.data;
      let newRedemptions = [...redemptions];
      let realIdx = (activePage - 1) * ITEMS_PER_PAGE + idx;
      if (action === 'delete') {
        newRedemptions[realIdx].deleted = true;
        setTotalCount((prev) => Math.max(prev - 1, 0));
      } else {
        newRedemptions[realIdx].status = redemption.status;
      }
      setRedemptions(newRedemptions);
    } else {
      showError(message);
    }
  };

  const searchRedemptions = async () => {
    if (searchKeyword === '') {
      // if keyword is blank, load files instead.
      await loadRedemptions(1);
      setActivePage(1);
      return;
    }
    setSearching(true);
    const res = await API.get(
      `/api/v1/admin/redemption/search?keyword=${searchKeyword}`
    );
    const { success, message, data } = res.data;
    if (success) {
      setIsSearchMode(true);
      setTotalCount(Array.isArray(data) ? data.length : 0);
      setRedemptions((Array.isArray(data) ? data : []).map(normalizeRedemptionRow));
      setActivePage(1);
    } else {
      showError(message);
    }
    setSearching(false);
  };

  const handleKeywordChange = async (e, { value }) => {
    setSearchKeyword(value.trim());
  };

  const handleTableChange = (_, __, sorter) => {
    if (!sorter || Array.isArray(sorter) || !sorter.columnKey || !sorter.order) {
      setTableSorter({ columnKey: null, order: null });
      return;
    }
    setTableSorter({
      columnKey: sorter.columnKey,
      order: sorter.order,
    });
  };

  const refresh = async () => {
    setLoading(true);
    await loadRedemptions(1);
    setActivePage(1);
  };

  const visibleRedemptionCount = redemptions.filter((row) => !row?.deleted).length;
  const totalPages = Math.max(
    Math.ceil((isSearchMode ? visibleRedemptionCount : totalCount) / ITEMS_PER_PAGE),
    1,
  );

  return (
    <>
      <AppFilterHeader
        className='router-block-gap-md'
        breadcrumbs={[
          { key: 'workspace', label: t('header.admin_workspace') },
          { key: 'business', label: t('header.business_operation') },
          { key: 'redemption', label: t('header.redemption'), active: true },
        ]}
        title={t('header.redemption')}
        actions={
          <div className='router-list-toolbar-actions'>
            <AppButton
              className='router-page-button'
              color='blue'
              onClick={() => navigate('/admin/redemption/add')}
            >
              {t('redemption.buttons.add')}
            </AppButton>
            <AppButton className='router-page-button' onClick={refresh} loading={loading}>
              {t('redemption.buttons.refresh')}
            </AppButton>
          </div>
        }
        query={
          <div className='router-list-toolbar-query'>
            <AppInput
              className='router-section-input'
              icon='search'
              fluid
              iconPosition='left'
              placeholder={t('redemption.search')}
              value={searchKeyword}
              loading={searching}
              onChange={handleKeywordChange}
            />
          </div>
        }
      />

      <div className='router-table-scroll-x'>
        <AppTable
          className='router-hover-table router-list-table router-table-fit-page router-redemption-list-table'
          pagination={false}
          scroll={{ x: REDEMPTION_LIST_TABLE_MIN_WIDTH }}
          rowKey={(redemption) => redemption.id}
          onChange={handleTableChange}
          dataSource={redemptions
            .slice(
              (activePage - 1) * ITEMS_PER_PAGE,
              activePage * ITEMS_PER_PAGE,
            )
            .filter((redemption) => !redemption?.deleted)}
          onRow={(redemption) => ({
            className: 'router-row-clickable',
            onClick: () => {
              navigate(`/admin/redemption/${redemption.id}`, {
                state: {
                  from: currentPagePath,
                },
              });
            },
          })}
          columns={[
          {
            title: t('redemption.table.name'),
            dataIndex: 'name',
            key: 'name',
            width: REDEMPTION_LIST_COLUMN_WIDTHS.name,
            ellipsis: true,
            sorter: (a, b) => compareTextValue(a.name, b.name),
            sortDirections: ['ascend', 'descend'],
            sortOrder: tableSorter.columnKey === 'name' ? tableSorter.order : null,
            render: (value) => value || t('redemption.table.no_name'),
          },
          {
            title: t('redemption.table.group'),
            dataIndex: 'groupLabel',
            key: 'groupLabel',
            width: REDEMPTION_LIST_COLUMN_WIDTHS.group,
            ellipsis: true,
            sorter: (a, b) => compareTextValue(a.groupLabel, b.groupLabel),
            sortDirections: ['ascend', 'descend'],
            sortOrder:
              tableSorter.columnKey === 'groupLabel' ? tableSorter.order : null,
            render: (value) => value || '-',
          },
          {
            title: t('redemption.table.status'),
            dataIndex: 'status',
            key: 'status',
            className: 'router-table-col-status-compact',
            width: REDEMPTION_LIST_COLUMN_WIDTHS.status,
            sorter: (a, b) => compareNumberValue(a.status, b.status),
            sortDirections: ['ascend', 'descend'],
            sortOrder: tableSorter.columnKey === 'status' ? tableSorter.order : null,
            render: (value) => renderStatus(value, t),
          },
          {
            title: (
              <div className='router-table-header-with-control router-redemption-face-value-header'>
                <span>{t('redemption.table.face_value')}</span>
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
            key: 'face_value',
            width: REDEMPTION_LIST_COLUMN_WIDTHS.faceValue,
            sorter: (a, b) => compareNumberValue(a.creditedChargeAmount, b.creditedChargeAmount),
            sortDirections: ['ascend', 'descend'],
            sortOrder:
              tableSorter.columnKey === 'face_value' ? tableSorter.order : null,
            render: (_, redemption) =>
              renderDisplayFaceValue(redemption, displayUnit, currencyIndex),
          },
          {
            title: t('redemption.table.created_time'),
            key: 'created_time',
            className: 'router-table-col-datetime',
            width: REDEMPTION_LIST_COLUMN_WIDTHS.createdTime,
            sorter: (a, b) => compareNumberValue(a.createdTime, b.createdTime),
            sortDirections: ['ascend', 'descend'],
            sortOrder:
              tableSorter.columnKey === 'created_time' ? tableSorter.order : null,
            render: (_, redemption) =>
              renderTimestamp(redemption.createdTime || redemption.created_time),
          },
          {
            title: t('redemption.table.code_expires_at'),
            dataIndex: 'code_expires_at',
            key: 'code_expires_at',
            className: 'router-table-col-datetime',
            width: REDEMPTION_LIST_COLUMN_WIDTHS.codeExpiresAt,
            sorter: (a, b) =>
              compareNumberValue(a.code_expires_at, b.code_expires_at),
            sortDirections: ['ascend', 'descend'],
            sortOrder:
              tableSorter.columnKey === 'code_expires_at'
                ? tableSorter.order
                : null,
            render: (value) => renderExpiryTime(value, t),
          },
          {
            title: t('redemption.table.redeemed_time'),
            key: 'redeemed_time',
            className: 'router-table-col-datetime',
            width: REDEMPTION_LIST_COLUMN_WIDTHS.redeemedTime,
            sorter: (a, b) => compareNumberValue(a.redeemedTime, b.redeemedTime),
            sortDirections: ['ascend', 'descend'],
            sortOrder:
              tableSorter.columnKey === 'redeemed_time'
                ? tableSorter.order
                : null,
            render: (_, redemption) =>
              redemption.redeemedTime
                ? renderTimestamp(redemption.redeemedTime)
                : t('redemption.table.not_redeemed'),
          },
          {
            title: t('redemption.table.actions'),
            key: 'actions',
            className: 'router-table-col-actions-icon',
            width: 120,
            render: (_, redemption, idx) => (
              <div
                className='router-action-group-tight router-table-actions-icon-compact'
                onClick={(e) => {
                  e.stopPropagation();
                }}
              >
                <AppTableActionButton
                  icon='copy outline'
                  title={t('redemption.buttons.copy')}
                  color='blue'
                  onClick={async () => {
                    if (await copy(redemption.code)) {
                      showSuccess(t('token.messages.copy_success'));
                    } else {
                      showWarning(t('token.messages.copy_failed'));
                      setSearchKeyword(redemption.code);
                    }
                  }}
                />
                <AppPopconfirm
                  title={t('redemption.buttons.confirm_delete')}
                  onConfirm={() => {
                    manageRedemption(redemption.id, 'delete', idx);
                  }}
                >
                  <span>
                    <AppTableActionButton
                      icon='trash'
                      title={t('redemption.buttons.delete')}
                      color='red'
                    />
                  </span>
                </AppPopconfirm>
                <AppTableActionButton
                  icon={redemption.status === 1 ? 'close' : 'check'}
                  title={
                    redemption.status === 1
                      ? t('redemption.buttons.disable')
                      : t('redemption.buttons.enable')
                  }
                  disabled={redemption.status === 3}
                  onClick={() => {
                    manageRedemption(
                      redemption.id,
                      redemption.status === 1 ? 'disable' : 'enable',
                      idx,
                    );
                  }}
                />
              </div>
            ),
          },
          ]}
        />
      </div>
      <div className='router-pagination-wrap'>
        <AppPagination
          className='router-page-pagination'
          activePage={activePage}
          onPageChange={onPaginationChange}
          siblingRange={1}
          totalPages={totalPages}
        />
      </div>
    </>
  );
};

export default RedemptionsTable;
