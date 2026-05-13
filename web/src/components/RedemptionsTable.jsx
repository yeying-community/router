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
  AppTag,
} from '../router-ui';

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
    creditedYYC: Number(row?.yyc_value ?? row?.quota ?? 0),
    groupLabel: renderGroupLabel(row),
    createdTime: Number(row?.created_time ?? 0),
    redeemedTime: Number(row?.redeemed_time ?? 0),
  };
}

function buildDisplayValue(redemption, displayUnit, currencyIndex) {
  // Keep legacy quota fallback for older redemption records.
  const creditedYYC = Number(redemption?.creditedYYC ?? redemption?.yyc_value ?? redemption?.quota ?? 0);
  const targetCurrency = currencyIndex[displayUnit] || currencyIndex.YYC;
  const rate = Number(targetCurrency?.yyc_per_unit || 0);
  if (!Number.isFinite(rate) || rate <= 0) {
    return '-';
  }
  return formatByCurrencyMinorUnit(creditedYYC / rate, targetCurrency);
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
  const [displayUnit, setDisplayUnit] = useState('USD');
  const [currencyIndex, setCurrencyIndex] = useState(
    buildBillingCurrencyIndex([], { placeholderCodes: ['USD', 'CNY'] })
  );

  const displayUnitOptions = useMemo(
    () => buildDisplayUnitOptions(currencyIndex, { order: 'yyc-first' }),
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

  const sortRedemption = (key) => {
    if (redemptions.length === 0) return;
    setLoading(true);
    let sortedRedemptions = [...redemptions];
    sortedRedemptions.sort((a, b) => {
      if (!isNaN(a[key])) {
        // If the value is numeric, subtract to sort
        return a[key] - b[key];
      } else {
        // If the value is not numeric, sort as strings
        return ('' + a[key]).localeCompare(b[key]);
      }
    });
    if (sortedRedemptions[0].id === redemptions[0].id) {
      sortedRedemptions.reverse();
    }
    setRedemptions(sortedRedemptions);
    setLoading(false);
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
        title={t('header.redemption')}
        actions={
          <div className='router-list-toolbar-actions'>
            <AppButton
              className='router-page-button'
              color='blue'
              onClick={() => navigate('/redemption/add')}
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

      <AppTable
        className='router-hover-table router-list-table'
        pagination={false}
        rowKey={(redemption) => redemption.id}
        dataSource={redemptions
          .slice(
            (activePage - 1) * ITEMS_PER_PAGE,
            activePage * ITEMS_PER_PAGE,
          )
          .filter((redemption) => !redemption?.deleted)}
        onRow={(redemption) => ({
          className: 'router-row-clickable',
          onClick: () => {
            navigate(`/redemption/${redemption.id}`, {
              state: {
                from: currentPagePath,
              },
            });
          },
        })}
        columns={[
          {
            title: (
              <span
                className='router-sortable-header'
                onClick={() => {
                  sortRedemption('name');
                }}
              >
                {t('redemption.table.name')}
              </span>
            ),
            dataIndex: 'name',
            key: 'name',
            render: (value) => value || t('redemption.table.no_name'),
          },
          {
            title: (
              <span
                className='router-sortable-header'
                onClick={() => {
                  sortRedemption('group_name');
                }}
              >
                {t('redemption.table.group')}
              </span>
            ),
            dataIndex: 'groupLabel',
            key: 'groupLabel',
            render: (value) => value || '-',
          },
          {
            title: (
              <span
                className='router-sortable-header'
                onClick={() => {
                  sortRedemption('status');
                }}
              >
                {t('redemption.table.status')}
              </span>
            ),
            dataIndex: 'status',
            key: 'status',
            render: (value) => renderStatus(value, t),
          },
          {
            title: (
              <div className='router-table-header-with-control router-redemption-face-value-header'>
                <span
                  className='router-sortable-header'
                  onClick={() => {
                    sortRedemption('creditedYYC');
                  }}
                >
                  {t('redemption.table.face_value')}
                </span>
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
            render: (_, redemption) =>
              renderDisplayFaceValue(redemption, displayUnit, currencyIndex),
          },
          {
            title: (
              <span
                className='router-sortable-header'
                onClick={() => {
                  sortRedemption('created_time');
                }}
              >
                {t('redemption.table.created_time')}
              </span>
            ),
            key: 'created_time',
            render: (_, redemption) =>
              renderTimestamp(redemption.createdTime || redemption.created_time),
          },
          {
            title: (
              <span
                className='router-sortable-header'
                onClick={() => {
                  sortRedemption('code_expires_at');
                }}
              >
                {t('redemption.table.code_expires_at')}
              </span>
            ),
            dataIndex: 'code_expires_at',
            key: 'code_expires_at',
            render: (value) => renderExpiryTime(value, t),
          },
          {
            title: (
              <span
                className='router-sortable-header'
                onClick={() => {
                  sortRedemption('redeemed_time');
                }}
              >
                {t('redemption.table.redeemed_time')}
              </span>
            ),
            key: 'redeemed_time',
            render: (_, redemption) =>
              redemption.redeemedTime
                ? renderTimestamp(redemption.redeemedTime)
                : t('redemption.table.not_redeemed'),
          },
          {
            title: t('redemption.table.actions'),
            key: 'actions',
            className: 'router-table-action-cell router-redemption-action-cell',
            render: (_, redemption, idx) => (
              <div
                className='router-action-group-tight'
                onClick={(e) => {
                  e.stopPropagation();
                }}
              >
                <AppButton
                  className='router-inline-button'
                  color='blue'
                  onClick={async () => {
                    if (await copy(redemption.code)) {
                      showSuccess(t('token.messages.copy_success'));
                    } else {
                      showWarning(t('token.messages.copy_failed'));
                      setSearchKeyword(redemption.code);
                    }
                  }}
                >
                  {t('redemption.buttons.copy')}
                </AppButton>
                <AppPopconfirm
                  title={t('redemption.buttons.confirm_delete')}
                  onConfirm={() => {
                    manageRedemption(redemption.id, 'delete', idx);
                  }}
                >
                  <AppButton className='router-inline-button' color='red'>
                    {t('redemption.buttons.delete')}
                  </AppButton>
                </AppPopconfirm>
                <AppButton
                  className='router-inline-button'
                  disabled={redemption.status === 3}
                  onClick={() => {
                    manageRedemption(
                      redemption.id,
                      redemption.status === 1 ? 'disable' : 'enable',
                      idx,
                    );
                  }}
                >
                  {redemption.status === 1
                    ? t('redemption.buttons.disable')
                    : t('redemption.buttons.enable')}
                </AppButton>
              </div>
            ),
          },
        ]}
      />
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
