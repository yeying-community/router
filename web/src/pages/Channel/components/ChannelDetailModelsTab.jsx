import React from 'react';
import {
  AppAlert,
  AppButton,
  AppDetailSection,
  AppEmpty,
  AppInput,
  AppPagination,
  AppPopconfirm,
  AppSelect,
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
  getComplexPricingDetailsForModel,
  openComplexPricingModal,
  detailModelsEditLocked,
  providerDataLoading,
  toggleModelSelection,
  canSelectChannelModel,
  detailCurrentPageAllSelected,
  detailCurrentPagePartiallySelected,
  detailCurrentPageSelectableCount,
  toggleDetailCurrentPageSelections,
  normalizeChannelModelType,
  startDetailModelEdit,
  handleDeleteDetailModel,
  detailModelTotalPages,
  detailModelPage,
  setDetailModelPage,
  modelsSyncError,
}) => {
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

  const resolveInactiveLabel = (row) => {
    const disabledBy = (row?.disabled_by || '').toString().trim();
    if (disabledBy) {
      return t('channel.edit.model_selector.auto_paused');
    }
    return t('channel.edit.model_selector.inactive');
  };

  const renderMergedPrice = (row) => {
    const complexPricingDetails = getComplexPricingDetailsForModel(row);
    const hasComplexPricing = complexPricingDetails.some((detail) =>
      (detail.price_components || []).some(
        (component) =>
          Number(component.input_price || 0) > 0 ||
          Number(component.output_price || 0) > 0,
      ),
    );
    if (hasComplexPricing) {
      return (
        <AppButton
          type='button'
          className='router-inline-button'
          onClick={() => openComplexPricingModal(row)}
        >
          {t('channel.edit.model_selector.pricing_detail_button')}
        </AppButton>
      );
    }
    const hasInputPrice =
      row.input_price !== null && row.input_price !== undefined && row.input_price !== '';
    const hasOutputPrice =
      row.output_price !== null && row.output_price !== undefined && row.output_price !== '';
    if (!hasInputPrice && !hasOutputPrice) {
      return <span className='router-nowrap'>-</span>;
    }
    return (
      <span className='router-nowrap'>
        {hasInputPrice ? row.input_price : '-'}｜{hasOutputPrice ? row.output_price : '-'}
      </span>
    );
  };

  const tableRowSelection = {
    columnWidth: columnWidths.selection,
    selectedRowKeys: renderedChannelModels
      .filter((row) => row?.selected)
      .map((row) => `${row.upstream_model}-${row.model}`),
    getTitleCheckboxProps: () => ({
      checked: detailCurrentPageAllSelected,
      indeterminate: detailCurrentPagePartiallySelected,
      disabled:
        detailModelsEditing ||
        detailModelMutating ||
        providerDataLoading ||
        detailCurrentPageSelectableCount === 0,
    }),
    getCheckboxProps: (row) => {
      const canSelect = canSelectChannelModel(row);
      const isUnavailable = !canSelect && !row.selected;
      const disabledReason = isUnavailable
        ? row.enable_block_reason ||
          t('channel.edit.model_selector.selection_disabled_unassigned')
        : '';
      return {
        className: isUnavailable ? 'router-model-toggle-disabled' : undefined,
        disabled:
          detailModelMutating ||
          detailModelsEditing ||
          providerDataLoading ||
          isUnavailable,
        title: disabledReason || undefined,
      };
    },
    renderCell: (_, row, __, originNode) => {
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
          {originNode}
        </span>
      );
      if (disabledReason === '') {
        return checkboxNode;
      }
      return <AppTooltip title={disabledReason}>{checkboxNode}</AppTooltip>;
    },
    onSelect: (record, selected) => {
      toggleModelSelection(record.upstream_model, selected);
    },
    onSelectAll: (selected) => {
      toggleDetailCurrentPageSelections(selected);
    },
  };

  return (
    <AppDetailSection
      title={t('channel.edit.detail_models_title')}
      titleTag='span'
      headerStart={<span className='router-toolbar-meta'>({modelSectionMetaText})</span>}
      headerEnd={
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
        </>
      }
    >
      <div>
        <AppAlert
          type='info'
          showIcon
          className='router-section-message'
          title={t('channel.edit.model_selector.enable_hint')}
        />
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
              title: t('channel.edit.model_selector.table.name'),
              dataIndex: 'upstream_model',
              key: 'upstream_model',
              width: columnWidths.name,
              ellipsis: true,
              render: (value, row) => {
                const disableInfo = buildDisableInfo(row);
                const inactiveTag = row.inactive ? (
                  <AppTag color='grey' className='router-tag'>
                    {resolveInactiveLabel(row)}
                  </AppTag>
                ) : null;
                return (
                  <div className='router-cell-truncate' title={value}>
                    <span className='router-nowrap router-monospace-value'>{value}</span>
                    {disableInfo && inactiveTag ? (
                      <AppTooltip title={disableInfo}>{inactiveTag}</AppTooltip>
                    ) : (
                      inactiveTag
                    )}
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
              title: t('channel.edit.model_selector.table.alias'),
              dataIndex: 'model',
              key: 'model',
              width: columnWidths.alias,
              ellipsis: true,
              render: (value) => (
                <span
                  className='router-cell-truncate router-monospace-value'
                  title={value}
                >
                  {value}
                </span>
              ),
            },
            {
              title: t('channel.edit.model_selector.table.price_unit'),
              dataIndex: 'price_unit',
              key: 'price_unit',
              className: 'router-table-col-price-unit',
              width: columnWidths.priceUnit,
              ellipsis: true,
              render: (value) => <span className='router-nowrap'>{value}</span>,
            },
            {
              title: t('channel.edit.model_selector.table.price'),
              key: 'price',
              width: columnWidths.price,
              render: (_, row) => renderMergedPrice(row),
            },
            {
              title: t('channel.edit.model_selector.table.status'),
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
