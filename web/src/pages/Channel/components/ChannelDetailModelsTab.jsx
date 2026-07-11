import React, { useMemo, useState } from 'react';
import {
  AppAlert,
  AppButton,
  AppDetailSection,
  AppEmpty,
  AppFilterHeader,
  AppInput,
  AppPagination,
  AppPopconfirm,
  AppSelect,
  AppSwitch,
  AppTable,
  AppTableActionButton,
  AppTag,
  AppTooltip,
} from '../../../router-ui';

const formatDisableTime = (value) => {
  const timestamp = Number(value || 0);
  if (timestamp <= 0) {
    return '';
  }
  return new Date(timestamp * 1000).toLocaleString();
};

const ChannelDetailModelsTab = ({
  t,
  columnWidths,
  modelSectionMetaText,
  detailModelFilter,
  setDetailModelFilter,
  detailUpstreamStatusFilter,
  setDetailUpstreamStatusFilter,
  detailProviderFilter,
  setDetailProviderFilter,
  detailProviderFilterOptions,
  detailModelsEditing,
  modelSearchKeyword,
  setModelSearchKeyword,
  fetchModelsLoading,
  activeRefreshModelsTask,
  detailModelMutating,
  handleFetchModels,
  searchedChannelModels,
  visibleChannelModels,
  renderedChannelModels,
  detailModelsEditLocked,
  providerDataLoading,
  toggleModelSelection,
  canSelectChannelModel,
  normalizeChannelModelType,
  startDetailModelEdit,
  handleDeleteDetailModel,
  handleBatchSelectDetailModels,
  handleBatchDeleteDetailModels,
  detailModelTotalPages,
  detailModelPage,
  setDetailModelPage,
  modelsSyncError,
}) => {
  const [batchDeleteMode, setBatchDeleteMode] = useState(false);
  const [batchDeleteRowKeys, setBatchDeleteRowKeys] = useState([]);
  const [batchSelectMode, setBatchSelectMode] = useState(false);
  const [batchSelectRowKeys, setBatchSelectRowKeys] = useState([]);

  const buildRowKey = (row) => `${row.upstream_model}-${row.model}`;

  const batchDeleteRows = useMemo(() => {
    const selectedKeySet = new Set(batchDeleteRowKeys);
    return visibleChannelModels.filter((row) => selectedKeySet.has(buildRowKey(row)));
  }, [batchDeleteRowKeys, visibleChannelModels]);

  const batchSelectRows = useMemo(() => {
    const selectedKeySet = new Set(batchSelectRowKeys);
    return visibleChannelModels.filter((row) => selectedKeySet.has(buildRowKey(row)));
  }, [batchSelectRowKeys, visibleChannelModels]);

  const batchSelectableRows = useMemo(
    () => renderedChannelModels.filter((row) => canSelectChannelModel(row)),
    [canSelectChannelModel, renderedChannelModels]
  );

  const batchMode = batchSelectMode || batchDeleteMode;
  const batchRowKeys = batchSelectMode ? batchSelectRowKeys : batchDeleteRowKeys;
  const setBatchRowKeys = batchSelectMode
    ? setBatchSelectRowKeys
    : setBatchDeleteRowKeys;

  const buildDisableInfo = (row) => {
    const parts = [];
    const disabledBy = (row?.disabled_by || '').toString().trim();
    const disabledAt = formatDisableTime(row?.disabled_at);
    const disabledReason = (row?.disabled_reason || '').toString().trim();
    if (disabledBy) {
      parts.push(t('channel.edit.capability_disable.by', { value: disabledBy }));
    }
    if (disabledAt) {
      parts.push(t('channel.edit.capability_disable.at', { value: disabledAt }));
    }
    if (disabledReason) {
      parts.push(t('channel.edit.capability_disable.reason', { value: disabledReason }));
    }
    return parts.join('\n');
  };

  const tableRowSelection = batchMode
    ? {
        columnWidth: columnWidths.selection,
        selectedRowKeys: batchRowKeys,
        getTitleCheckboxProps: () => ({
          disabled:
            detailModelsEditing ||
            detailModelMutating ||
            providerDataLoading ||
            (batchSelectMode
              ? batchSelectableRows.length === 0
              : renderedChannelModels.length === 0),
        }),
        getCheckboxProps: (row) => ({
          disabled:
            detailModelsEditing ||
            detailModelMutating ||
            providerDataLoading ||
            (batchSelectMode && !canSelectChannelModel(row)),
        }),
        onSelect: (record, selected) => {
          const rowKey = buildRowKey(record);
          setBatchRowKeys((prev) => {
            const next = new Set(prev);
            if (selected) {
              next.add(rowKey);
            } else {
              next.delete(rowKey);
            }
            return Array.from(next);
          });
        },
        onSelectAll: (selected, selectedRows, changeRows) => {
          const changedRows = batchSelectMode
            ? changeRows.filter((row) => canSelectChannelModel(row))
            : changeRows;
          const changedKeys = changedRows.map(buildRowKey);
          setBatchRowKeys((prev) => {
            const next = new Set(prev);
            changedKeys.forEach((rowKey) => {
              if (selected) {
                next.add(rowKey);
              } else {
                next.delete(rowKey);
              }
            });
            return Array.from(next);
          });
        },
      }
    : undefined;

  const renderSelectionStatus = (row) => {
    const canSelect = canSelectChannelModel(row);
    const isUnavailable = !canSelect && !row.selected;
    const disabledReason = isUnavailable
      ? row.enable_block_reason ||
        t('channel.edit.model_selector.selection_disabled_unassigned')
      : '';
    const checkboxNode = (
      <span
        className={[
          'router-inline-block',
          'router-model-toggle-wrap',
          isUnavailable ? 'router-model-toggle-wrap-disabled' : '',
        ]
          .filter(Boolean)
          .join(' ')}
        aria-label={disabledReason || undefined}
      >
        <AppSwitch
          checked={row.selected === true}
          disabled={
            detailModelMutating ||
            detailModelsEditing ||
            providerDataLoading ||
            isUnavailable
          }
          onChange={(e, { checked }) =>
            toggleModelSelection(row.upstream_model, checked === true)
          }
        />
      </span>
    );
    if (disabledReason === '') {
      return checkboxNode;
    }
    return <AppTooltip title={disabledReason}>{checkboxNode}</AppTooltip>;
  };

  const handleConfirmBatchSelect = async () => {
    const ok = await handleBatchSelectDetailModels(batchSelectRows);
    if (ok) {
      setBatchSelectRowKeys([]);
      setBatchSelectMode(false);
    }
  };

  const handleConfirmBatchDelete = async () => {
    const ok = await handleBatchDeleteDetailModels(batchDeleteRows);
    if (ok) {
      setBatchDeleteRowKeys([]);
      setBatchDeleteMode(false);
    }
  };

  const renderToolbar = () => (
    <AppFilterHeader
      className='router-toolbar-compact'
      picker={
        <>
        <AppSelect
          className='router-section-dropdown router-dropdown-min-170 router-detail-filter-dropdown'
          disabled={detailModelsEditing}
          options={[
            {
              key: 'all',
              value: 'all',
              text: t('channel.edit.model_selector.filters.all'),
            },
            {
              key: 'enabled',
              value: 'enabled',
              text: t('channel.edit.model_selector.filters.enabled'),
            },
            {
              key: 'disabled',
              value: 'disabled',
              text: t('channel.edit.model_selector.filters.disabled'),
            },
          ]}
          value={detailModelFilter}
          onChange={(e, { value }) =>
            setDetailModelFilter((value || 'all').toString())
          }
        />
        <AppSelect
          className='router-section-dropdown router-dropdown-min-170 router-detail-filter-dropdown'
          disabled={detailModelsEditing}
          options={[
            {
              key: 'all',
              value: 'all',
              text: t('channel.edit.model_selector.filters.upstream_status_all'),
            },
            {
              key: 'returned',
              value: 'returned',
              text: t('channel.edit.model_selector.upstream_return_status.returned'),
            },
            {
              key: 'not_returned',
              value: 'not_returned',
              text: t(
                'channel.edit.model_selector.upstream_return_status.not_returned'
              ),
            },
          ]}
          value={detailUpstreamStatusFilter}
          onChange={(e, { value }) =>
            setDetailUpstreamStatusFilter((value || 'all').toString())
          }
        />
        <AppSelect
          className='router-section-dropdown router-dropdown-min-170 router-detail-filter-dropdown'
          disabled={detailModelsEditing || providerDataLoading}
          options={detailProviderFilterOptions}
          value={detailProviderFilter}
          onChange={(e, { value }) =>
            setDetailProviderFilter((value || 'all').toString())
          }
        />
        <AppInput
          className='router-section-input router-search-form-sm'
          icon='search'
          iconPosition='left'
          disabled={detailModelsEditing}
          placeholder={t('channel.edit.model_selector.search_placeholder')}
          value={modelSearchKeyword}
          onChange={(e, { value }) => setModelSearchKeyword(value || '')}
        />
        </>
      }
      actions={
        <>
        <AppButton
          type='button'
          className='router-page-button'
          loading={fetchModelsLoading || !!activeRefreshModelsTask}
          disabled={
            detailModelsEditing ||
            fetchModelsLoading ||
            !!activeRefreshModelsTask ||
            detailModelMutating
          }
          onClick={() => handleFetchModels({ silent: false })}
        >
          {t('channel.edit.buttons.sync_models')}
        </AppButton>
        {batchSelectMode ? (
          <>
            <AppButton
              type='button'
              color='blue'
              className='router-page-button'
              loading={detailModelMutating}
              disabled={batchSelectRows.length === 0 || detailModelMutating}
              onClick={handleConfirmBatchSelect}
            >
              {t('channel.edit.model_selector.batch_select_selected', {
                count: batchSelectRows.length,
              })}
            </AppButton>
            <AppButton
              type='button'
              className='router-page-button'
              disabled={detailModelMutating}
              onClick={() => {
                setBatchSelectMode(false);
                setBatchSelectRowKeys([]);
              }}
            >
              {t('common.cancel')}
            </AppButton>
          </>
        ) : (
          <AppButton
            type='button'
            className='router-page-button'
            disabled={
              detailModelsEditing ||
              detailModelMutating ||
              providerDataLoading ||
              batchDeleteMode ||
              searchedChannelModels.length === 0
            }
            onClick={() => {
              setBatchDeleteMode(false);
              setBatchDeleteRowKeys([]);
              setBatchSelectMode(true);
            }}
          >
            {t('channel.edit.model_selector.batch_select')}
          </AppButton>
        )}
        {batchDeleteMode ? (
          <>
            <AppPopconfirm
              title={t('channel.edit.model_selector.batch_delete_confirm', {
                count: batchDeleteRows.length,
              })}
              onConfirm={handleConfirmBatchDelete}
              okText={t('common.confirm')}
              cancelText={t('common.cancel')}
              disabled={batchDeleteRows.length === 0 || detailModelMutating}
            >
              <span>
                <AppButton
                  type='button'
                  color='red'
                  className='router-page-button'
                  loading={detailModelMutating}
                  disabled={batchDeleteRows.length === 0 || detailModelMutating}
                >
                  {t('channel.edit.model_selector.batch_delete_selected', {
                    count: batchDeleteRows.length,
                  })}
                </AppButton>
              </span>
            </AppPopconfirm>
            <AppButton
              type='button'
              className='router-page-button'
              disabled={detailModelMutating}
              onClick={() => {
                setBatchDeleteMode(false);
                setBatchDeleteRowKeys([]);
              }}
            >
              {t('common.cancel')}
            </AppButton>
          </>
        ) : (
          <AppButton
            type='button'
            className='router-page-button'
            disabled={
              detailModelsEditing ||
              detailModelMutating ||
              providerDataLoading ||
              batchSelectMode ||
              searchedChannelModels.length === 0
            }
            onClick={() => {
              setBatchSelectMode(false);
              setBatchSelectRowKeys([]);
              setBatchDeleteMode(true);
            }}
          >
            {t('channel.edit.model_selector.batch_delete')}
          </AppButton>
        )}
        </>
      }
    />
  );

  return (
    <AppDetailSection
      title={t('channel.edit.detail_models_title')}
      titleTag='span'
      headerStart={<span className='router-toolbar-meta'>({modelSectionMetaText})</span>}
    >
      <div>
        <AppAlert
          type='info'
          showIcon
          className='router-section-message'
          title={t('channel.edit.model_selector.enable_hint')}
        />
        {renderToolbar()}
        <AppTable
          className='router-detail-table router-table-fit-page router-channel-detail-model-table'
          pagination={false}
          rowSelection={tableRowSelection}
          locale={{
            emptyText: (
              <AppEmpty>
                {modelSearchKeyword.trim() !== ''
                  ? t('channel.edit.model_selector.empty_search')
                  : visibleChannelModels.length > 0
                    ? t('channel.edit.model_selector.empty_filtered')
                    : t('channel.edit.model_selector.empty')}
              </AppEmpty>
            ),
          }}
          rowKey={(row) => `${row.upstream_model}-${row.model}`}
          dataSource={searchedChannelModels.length === 0 ? [] : renderedChannelModels}
          columns={[
            {
              title: t('channel.edit.model_selector.table.selection_status'),
              key: 'selection_status',
              className: 'router-table-col-status-compact',
              width: columnWidths.selection,
              render: (_, row) => renderSelectionStatus(row),
            },
            {
              title: t('channel.edit.model_selector.table.name'),
              dataIndex: 'upstream_model',
              key: 'upstream_model',
              width: columnWidths.name,
              ellipsis: true,
              render: (value, row) => {
                const disableInfo = buildDisableInfo(row);
                return (
                  <div className='router-cell-truncate' title={value}>
                    <span className='router-nowrap router-monospace-value'>{value}</span>
                    {disableInfo ? (
                      <AppTooltip title={disableInfo}>
                        <AppTag color='grey' className='router-tag'>
                          {t('channel.edit.model_selector.runtime_disabled')}
                        </AppTag>
                      </AppTooltip>
                    ) : null}
                  </div>
                );
              },
            },
            {
              title: t('channel.edit.model_selector.table.type'),
              key: 'type',
              className: 'router-table-col-type-tight',
              width: columnWidths.type,
              render: (_, row) =>
                t(`channel.model_types.${normalizeChannelModelType(row.type)}`),
            },
            {
              title: t('channel.edit.model_selector.table.upstream_status'),
              key: 'status',
              className: 'router-table-col-status-compact',
              width: columnWidths.status,
              render: (_, row) => {
                const statusKey = row.sync_status || 'unknown';
                const color =
                  statusKey === 'returned'
                    ? 'green'
                    : statusKey === 'not_returned'
                      ? 'orange'
                      : 'grey';
                return (
                  <AppTag color={color} className='router-tag'>
                    {t(`channel.edit.model_selector.upstream_return_status.${statusKey}`)}
                  </AppTag>
                );
              },
            },
            {
              title: t('channel.edit.model_selector.table.upstream_return'),
              key: 'last_synced_at',
              className: 'router-table-col-datetime',
              width: columnWidths.upstreamReturn,
              render: (_, row) => {
                const syncedAtText =
                  Number(row.last_synced_at || 0) > 0
                    ? new Date(row.last_synced_at * 1000).toLocaleString()
                    : '-';
                return (
                  <span className='router-nowrap' title={syncedAtText}>
                    {syncedAtText}
                  </span>
                );
              },
            },
            {
              title: t('channel.table.actions'),
              key: 'actions',
              className: 'router-table-col-actions-icon',
              width: 84,
              render: (_, row) => {
                const rowEditActionDisabled =
                  detailModelsEditLocked || detailModelMutating || detailModelsEditing;
                const rowDeleteDisabled =
                  detailModelMutating || detailModelsEditing;
                return (
                  <div className='router-inline-actions router-table-actions-icon-compact'>
                    <AppTableActionButton
                      icon='edit'
                      title={t('common.edit')}
                      disabled={rowEditActionDisabled}
                      onClick={() => startDetailModelEdit(row.upstream_model)}
                    />
                    <AppPopconfirm
                      title={t('channel.edit.model_selector.delete_confirm')}
                      onConfirm={() => handleDeleteDetailModel(row)}
                      okText={t('common.confirm')}
                      cancelText={t('common.cancel')}
                      disabled={rowDeleteDisabled}
                    >
                      <span>
                        <AppTableActionButton
                          icon='trash'
                          title={t('common.delete')}
                          color='red'
                          disabled={rowDeleteDisabled}
                        />
                      </span>
                    </AppPopconfirm>
                  </div>
                );
              },
            },
          ]}
        />
        {detailModelTotalPages > 1 && (
          <div className='router-pagination-wrap'>
            <AppPagination
              className='router-section-pagination'
              activePage={detailModelPage}
              totalPages={detailModelTotalPages}
              onPageChange={(e, { activePage }) =>
                setDetailModelPage(Number(activePage) || 1)
              }
            />
          </div>
        )}
        {modelsSyncError && (
          <div className='router-error-text router-error-text-top'>
            {modelsSyncError}
          </div>
        )}
      </div>
    </AppDetailSection>
  );
};

export default ChannelDetailModelsTab;
