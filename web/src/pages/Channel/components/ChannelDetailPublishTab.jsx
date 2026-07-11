import React, { useMemo } from 'react';
import {
  AppAlert,
  AppButton,
  AppDetailSection,
  AppEmpty,
  AppInput,
  AppPopconfirm,
  AppTable,
  AppTag,
} from '../../../router-ui';

const normalizePublishStatus = (row) => {
  const explicitStatus = (row?.publish_status || '').toString().trim();
  if (explicitStatus) {
    return explicitStatus;
  }
  if (!row?.selected) {
    return 'selectable';
  }
  return 'pending_config';
};

const publishStatusColor = (status) => {
  switch (status) {
    case 'published':
      return 'green';
    case 'pending_publish':
      return 'blue';
    case 'pending_test':
      return 'orange';
    case 'pending_config':
      return 'yellow';
    case 'selectable':
      return 'blue';
    case 'disabled':
      return 'grey';
    default:
      return 'grey';
  }
};

const publishCheckColor = (status) => {
  switch (status) {
    case 'published':
    case 'pending_publish':
      return 'green';
    case 'pending_test':
    case 'pending_config':
      return 'orange';
    default:
      return 'grey';
  }
};

const ChannelDetailPublishTab = ({
  t,
  channelModels,
  getComplexPricingDetailsForModel,
  openComplexPricingModal,
  normalizeChannelModelType,
  onUpdatePublishedModelName,
  onUpdatePublish,
  publishMutatingModel,
  publishReadonly,
}) => {
  const publishRows = useMemo(
    () =>
      (Array.isArray(channelModels) ? channelModels : [])
        .filter((row) => row?.selected === true)
        .sort((left, right) =>
          (left?.model || left?.upstream_model || '').localeCompare(
            right?.model || right?.upstream_model || '',
          ),
        ),
    [channelModels],
  );

  const renderPrice = (row, field) => {
    const complexPricingDetails = getComplexPricingDetailsForModel(row);
    const hasComplexPricing = complexPricingDetails.some((detail) =>
      (detail.price_components || []).some(
        (component) =>
          Number(component[field] || 0) > 0,
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
    const price = row?.[field];
    const hasPrice =
      price !== null &&
      price !== undefined &&
      price !== '';
    if (!hasPrice) {
      return <span className='router-nowrap'>-</span>;
    }
    return <span className='router-nowrap'>{price}</span>;
  };

  const renderPublishCheck = (row) => {
    const status = normalizePublishStatus(row);
    return (
      <AppTag color={publishCheckColor(status)} className='router-tag'>
        {t(`channel.edit.publish.check_status.${status}`)}
      </AppTag>
    );
  };

  const renderPublishAction = (row) => {
    const status = normalizePublishStatus(row);
    const modelName = (row?.model || row?.upstream_model || '').toString().trim();
    const isMutating = publishMutatingModel === modelName;
    if (status === 'published') {
      const currentPublishedName = (row?.published_model || row?.model || row?.upstream_model || '')
        .toString()
        .trim();
      const originalPublishedName = (row?.published_model_original || row?.published_model || row?.model || row?.upstream_model || '')
        .toString()
        .trim();
      const hasNameChange = currentPublishedName !== originalPublishedName;
      return (
        <div className='router-inline-actions'>
          {hasNameChange && (
            <AppButton
              type='button'
              className='router-inline-button'
              loading={isMutating}
              disabled={publishReadonly || isMutating}
              onClick={() => onUpdatePublish?.(row, true)}
            >
              {t('channel.edit.publish.action_save_name')}
            </AppButton>
          )}
          <AppPopconfirm
            title={t('channel.edit.publish.unpublish_confirm')}
            okText={t('common.confirm')}
            cancelText={t('common.cancel')}
            disabled={publishReadonly || isMutating}
            onConfirm={() => onUpdatePublish?.(row, false)}
          >
            <span>
              <AppButton
                type='button'
                className='router-inline-button'
                loading={isMutating}
                disabled={publishReadonly || isMutating}
              >
                {t('channel.edit.publish.action_unpublish')}
              </AppButton>
            </span>
          </AppPopconfirm>
        </div>
      );
    }
    const publishDisabled = publishReadonly || isMutating || status !== 'pending_publish';
    return (
      <AppButton
        type='button'
        className='router-inline-button'
        loading={isMutating}
        disabled={publishDisabled}
        title={
          publishDisabled && status !== 'pending_publish'
            ? t(`channel.edit.publish.check_status.${status}`)
            : undefined
        }
        onClick={() => onUpdatePublish?.(row, true)}
      >
        {t('channel.edit.publish.action_publish')}
      </AppButton>
    );
  };

  return (
    <AppDetailSection
      title={t('channel.edit.publish.title')}
      titleTag='span'
      headerStart={
        <span className='router-toolbar-meta'>
          ({t('channel.edit.publish.summary', { count: publishRows.length })})
        </span>
      }
    >
      <div>
        <AppAlert
          type='info'
          showIcon
          className='router-section-message'
          title={t('channel.edit.publish.hint')}
        />
        <AppTable
          className='router-detail-table router-table-fit-page'
          pagination={false}
          locale={{
            emptyText: (
              <AppEmpty>{t('channel.edit.publish.empty')}</AppEmpty>
            ),
          }}
          rowKey={(row) => row.model || row.upstream_model}
          dataSource={publishRows}
          columns={[
            {
              title: t('channel.edit.model_selector.table.name'),
              dataIndex: 'upstream_model',
              key: 'upstream_model',
              width: 180,
              ellipsis: true,
              render: (value) => (
                <span className='router-cell-truncate router-monospace-value' title={value || '-'}>
                  {value || '-'}
                </span>
              ),
            },
            {
              title: t('channel.edit.publish.table.published_model'),
              key: 'published_model',
              width: 190,
              render: (_, row) => {
                const modelName = (row?.model || row?.upstream_model || '').toString().trim();
                return (
                  <AppInput
                    className='router-table-input router-monospace-value'
                    value={
                      Object.prototype.hasOwnProperty.call(row || {}, 'published_model')
                        ? row.published_model
                        : modelName
                    }
                    disabled={publishReadonly}
                    onChange={(event, data) =>
                      onUpdatePublishedModelName?.(
                        row,
                        data?.value ?? event?.target?.value ?? '',
                      )
                    }
                  />
                );
              },
            },
            {
              title: t('channel.edit.model_selector.table.type'),
              key: 'type',
              width: 72,
              render: (_, row) =>
                t(`channel.model_types.${normalizeChannelModelType(row.type)}`),
            },
            {
              title: t('channel.edit.model_selector.table.publish_status'),
              key: 'publish_status',
              width: 96,
              render: (_, row) => {
                const status = normalizePublishStatus(row);
                return (
                  <AppTag color={publishStatusColor(status)} className='router-tag'>
                    {t(`channel.edit.model_selector.publish_status.${status}`)}
                  </AppTag>
                );
              },
            },
            {
              title: t('channel.edit.model_selector.table.input_price'),
              key: 'input_price',
              width: 112,
              render: (_, row) => renderPrice(row, 'input_price'),
            },
            {
              title: t('channel.edit.model_selector.table.output_price'),
              key: 'output_price',
              width: 112,
              render: (_, row) => renderPrice(row, 'output_price'),
            },
            {
              title: t('channel.edit.publish.table.check'),
              key: 'check',
              width: 128,
              render: (_, row) => renderPublishCheck(row),
            },
            {
              title: t('channel.edit.publish.table.actions'),
              key: 'actions',
              width: 112,
              render: (_, row) => renderPublishAction(row),
            },
          ]}
        />
      </div>
    </AppDetailSection>
  );
};

export default ChannelDetailPublishTab;
