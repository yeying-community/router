import React from 'react';
import {
  AppButton,
  AppField,
  AppFormActions,
  AppFormRow,
  AppInput,
  AppInputNumber,
  AppModal,
  AppSelect,
  AppSwitch,
  AppTable,
} from '../../../router-ui';

const priceUnitOptions = [
  { key: 'per_1k_tokens', value: 'per_1k_tokens', text: 'per_1k_tokens' },
  { key: 'per_1k_chars', value: 'per_1k_chars', text: 'per_1k_chars' },
  { key: 'per_image', value: 'per_image', text: 'per_image' },
  { key: 'per_video', value: 'per_video', text: 'per_video' },
  { key: 'per_minute', value: 'per_minute', text: 'per_minute' },
  { key: 'per_second', value: 'per_second', text: 'per_second' },
  { key: 'per_request', value: 'per_request', text: 'per_request' },
  { key: 'per_task', value: 'per_task', text: 'per_task' },
];

const CHANNEL_MODEL_EDITOR_PRICING_COLUMN_WIDTHS = {
  component: 140,
  condition: 132,
  inputPrice: 120,
  outputPrice: 120,
  priceUnit: 220,
  currency: 120,
};

const CHANNEL_MODEL_EDITOR_PRICING_TABLE_MIN_WIDTH =
  CHANNEL_MODEL_EDITOR_PRICING_COLUMN_WIDTHS.component +
  CHANNEL_MODEL_EDITOR_PRICING_COLUMN_WIDTHS.condition +
  CHANNEL_MODEL_EDITOR_PRICING_COLUMN_WIDTHS.inputPrice +
  CHANNEL_MODEL_EDITOR_PRICING_COLUMN_WIDTHS.outputPrice +
  CHANNEL_MODEL_EDITOR_PRICING_COLUMN_WIDTHS.priceUnit +
  CHANNEL_MODEL_EDITOR_PRICING_COLUMN_WIDTHS.currency;

const ChannelModelEditorModal = ({
  t,
  open,
  onClose,
  detailModelMutating,
  detailEditingModelRow,
  normalizeChannelModelType,
  updateModelConfigField,
  providerDataLoading,
  getProviderSelectOptionsForModel,
  resolvePreferredProviderForModel,
  openAppendProviderModal,
  canSelectChannelModel,
  toggleModelSelection,
  getComplexPricingDetailsForModel,
  saveDetailModelsConfig,
}) => {
  const providerComponentDefaults =
    (getComplexPricingDetailsForModel(detailEditingModelRow || {})[0]
      ?.price_components || []);
  const effectivePriceComponents =
    (detailEditingModelRow?.price_components || []).length > 0
      ? detailEditingModelRow.price_components
      : providerComponentDefaults;
  const hasComponentPricing = effectivePriceComponents.length > 0;
  const canToggleModelEnabled = canSelectChannelModel({
    ...(detailEditingModelRow || {}),
    inactive: false,
  });

  const updatePriceComponentField = (index, field, value) => {
    const nextComponents = effectivePriceComponents.map((component, itemIndex) => {
      if (itemIndex !== index) {
        return component;
      }
      if (field === 'input_price' || field === 'output_price') {
        const price = Number(value);
        return {
          ...component,
          [field]: Number.isFinite(price) && price >= 0 ? price : 0,
          source: 'channel_override',
        };
      }
      return {
        ...component,
        [field]: value || '',
        source: 'channel_override',
      };
    });
    updateModelConfigField(
      detailEditingModelRow.upstream_model,
      'price_components',
      nextComponents,
    );
  };

  return (
    <AppModal
      size='small'
      open={open}
      onClose={onClose}
      closeOnDimmerClick={!detailModelMutating}
      closeOnEscape={!detailModelMutating}
      className='router-channel-model-editor-modal'
      title={`${t('common.edit')} · ${detailEditingModelRow?.upstream_model || '-'}`}
      footer={
        <AppFormActions>
          <AppButton
            type='button'
            className='router-modal-button'
            onClick={onClose}
            disabled={detailModelMutating}
          >
            {t('channel.edit.buttons.cancel')}
          </AppButton>
          <AppButton
            type='button'
            className='router-modal-button'
            color='blue'
            loading={detailModelMutating}
            disabled={detailModelMutating}
            onClick={saveDetailModelsConfig}
          >
            {t('channel.edit.buttons.save')}
          </AppButton>
        </AppFormActions>
      }
    >
      {detailEditingModelRow ? (
        <div className='router-channel-model-editor-form'>
          <div className='router-channel-model-editor-card'>
            <div className='router-channel-model-editor-section-title'>
              {t('channel.edit.model_selector.editor.info_title')}
            </div>
            <AppFormRow>
              <AppField label={t('channel.edit.model_selector.table.name')} readOnly>
                <AppInput
                  className='router-modal-input'
                  value={detailEditingModelRow.upstream_model || '-'}
                  readOnly
                />
              </AppField>
              <AppField label={t('channel.edit.model_selector.table.type')} readOnly>
                <AppInput
                  className='router-modal-input'
                  value={t(
                    `channel.model_types.${normalizeChannelModelType(detailEditingModelRow.type)}`,
                  )}
                  readOnly
                />
              </AppField>
            </AppFormRow>
            <AppFormRow>
              <AppField label={t('channel.edit.model_selector.table.alias')}>
                <AppInput
                  className='router-modal-input'
                  value={detailEditingModelRow.model || ''}
                  onChange={(e, { value }) =>
                    updateModelConfigField(
                      detailEditingModelRow.upstream_model,
                      'model',
                      value || detailEditingModelRow.upstream_model,
                    )
                  }
                />
              </AppField>
            </AppFormRow>
            <AppField label={t('channel.edit.model_selector.table.providers')}>
              <div className='router-channel-model-editor-provider-row'>
                <AppSelect
                  fluid
                  className='router-modal-dropdown'
                  placeholder={t(
                    'channel.edit.model_selector.editor.provider_placeholder',
                  )}
                  options={getProviderSelectOptionsForModel(
                    detailEditingModelRow,
                  )}
                  value={resolvePreferredProviderForModel(
                    detailEditingModelRow,
                  )}
                  disabled={
                    providerDataLoading ||
                    getProviderSelectOptionsForModel(detailEditingModelRow)
                      .length === 0
                  }
                  onChange={(e, { value }) =>
                    updateModelConfigField(
                      detailEditingModelRow.upstream_model,
                      'provider',
                      value || '',
                    )
                  }
                />
                {getProviderSelectOptionsForModel(detailEditingModelRow)
                  .length === 0 ? (
                  <>
                    <span className='router-text-meta'>
                      {t('channel.edit.model_selector.editor.provider_empty')}
                    </span>
                    <AppButton
                      type='button'
                      className='router-inline-button'
                      basic
                      onClick={() => openAppendProviderModal(detailEditingModelRow)}
                    >
                      {t('channel.edit.model_selector.provider_add')}
                    </AppButton>
                  </>
                ) : null}
              </div>
            </AppField>
          </div>

          <div className='router-channel-model-editor-card'>
            <div className='router-channel-model-editor-section-title'>
              {t('channel.edit.model_selector.editor.status_title')}
            </div>
            <AppFormRow>
              <AppField
                label={t('channel.edit.model_selector.editor.upstream_return_title')}
                readOnly
              >
                <AppInput
                  className='router-modal-input'
                  value={[
                    t(
                      `channel.edit.model_selector.upstream_return_status.${
                        detailEditingModelRow.sync_status || 'unknown'
                      }`,
                    ),
                    Number(detailEditingModelRow.last_synced_at || 0) > 0
                      ? new Date(
                          detailEditingModelRow.last_synced_at * 1000,
                        ).toLocaleString()
                      : '',
                  ]
                    .filter(Boolean)
                    .join(' · ')}
                  readOnly
                />
              </AppField>
            </AppFormRow>
            <AppFormRow>
              <AppField
                label={t('channel.edit.model_selector.editor.enable_block_reason_title')}
                readOnly
              >
                <AppInput
                  className='router-modal-input'
                  value={detailEditingModelRow.enable_block_reason || '-'}
                  readOnly
                />
              </AppField>
            </AppFormRow>
            <div className='router-channel-model-editor-toggle-row'>
              <div className='router-channel-model-editor-toggle-copy'>
                <div className='router-channel-model-editor-toggle-label'>
                  {t('channel.edit.model_selector.table.selected')}
                </div>
                <div className='router-channel-model-editor-toggle-hint'>
                  {t('channel.edit.model_selector.editor.status_hint')}
                </div>
              </div>
              <AppSwitch
                checked={!!detailEditingModelRow.selected}
                disabled={
                  detailModelMutating ||
                  providerDataLoading ||
                  (!canToggleModelEnabled && !detailEditingModelRow.selected)
                }
                onChange={(_, { checked }) =>
                  toggleModelSelection(
                    detailEditingModelRow.upstream_model,
                    checked === true,
                  )
                }
              />
            </div>
          </div>

          <div className='router-channel-model-editor-card'>
            <div className='router-channel-model-editor-section-title'>
              {t('channel.edit.model_selector.editor.pricing_title')}
            </div>
            {hasComponentPricing ? (
              <div className='router-channel-model-editor-table-wrap'>
                <AppTable
                  className='router-detail-table router-channel-model-editor-pricing-table'
                  pagination={false}
                  scroll={{ x: CHANNEL_MODEL_EDITOR_PRICING_TABLE_MIN_WIDTH }}
                  rowKey={(component) =>
                    [
                      component?.component || 'component',
                      component?.condition || 'default',
                      component?.price_unit || 'unit',
                      component?.source || 'source',
                    ].join('-')
                  }
                  dataSource={effectivePriceComponents}
                  columns={[
                    {
                      title: t('channel.edit.model_selector.pricing_detail_table.component'),
                      dataIndex: 'component',
                      key: 'component',
                      width: CHANNEL_MODEL_EDITOR_PRICING_COLUMN_WIDTHS.component,
                      render: (value) => value || '-',
                    },
                    {
                      title: t('channel.edit.model_selector.pricing_detail_table.condition'),
                      dataIndex: 'condition',
                      key: 'condition',
                      width: CHANNEL_MODEL_EDITOR_PRICING_COLUMN_WIDTHS.condition,
                      render: (value) => value || '-',
                    },
                    {
                      title: t('channel.edit.model_selector.table.input_price'),
                      dataIndex: 'input_price',
                      key: 'input_price',
                      width: CHANNEL_MODEL_EDITOR_PRICING_COLUMN_WIDTHS.inputPrice,
                      render: (value, _record, index) => (
                        <AppInputNumber
                          className='router-modal-input'
                          min={0}
                          step={0.000001}
                          precision={6}
                          fluid
                          value={value ?? 0}
                          onChange={(e, { value: nextValue }) =>
                            updatePriceComponentField(index, 'input_price', nextValue)
                          }
                        />
                      ),
                    },
                    {
                      title: t('channel.edit.model_selector.table.output_price'),
                      dataIndex: 'output_price',
                      key: 'output_price',
                      width: CHANNEL_MODEL_EDITOR_PRICING_COLUMN_WIDTHS.outputPrice,
                      render: (value, _record, index) => (
                        <AppInputNumber
                          className='router-modal-input'
                          min={0}
                          step={0.000001}
                          precision={6}
                          fluid
                          value={value ?? 0}
                          onChange={(e, { value: nextValue }) =>
                            updatePriceComponentField(index, 'output_price', nextValue)
                          }
                        />
                      ),
                    },
                    {
                      title: t('channel.edit.model_selector.table.price_unit'),
                      dataIndex: 'price_unit',
                      key: 'price_unit',
                      width: CHANNEL_MODEL_EDITOR_PRICING_COLUMN_WIDTHS.priceUnit,
                      render: (value, _record, index) => (
                        <AppSelect
                          className='router-modal-dropdown'
                          options={priceUnitOptions}
                          value={value || 'per_1k_tokens'}
                          onChange={(e, { value: nextValue }) =>
                            updatePriceComponentField(
                              index,
                              'price_unit',
                              nextValue || 'per_1k_tokens',
                            )
                          }
                        />
                      ),
                    },
                    {
                      title: t('channel.edit.model_selector.pricing_detail_table.currency'),
                      dataIndex: 'currency',
                      key: 'currency',
                      width: CHANNEL_MODEL_EDITOR_PRICING_COLUMN_WIDTHS.currency,
                      render: (value, _record, index) => (
                        <AppInput
                          className='router-modal-input'
                          value={value || 'USD'}
                          onChange={(e, { value: nextValue }) =>
                            updatePriceComponentField(index, 'currency', nextValue || 'USD')
                          }
                        />
                      ),
                    },
                  ]}
                />
              </div>
            ) : (
              <AppFormRow>
                <AppField label={t('channel.edit.model_selector.table.price_unit')}>
                  <AppSelect
                    className='router-modal-dropdown'
                    options={priceUnitOptions}
                    value={detailEditingModelRow.price_unit || 'per_1k_tokens'}
                    onChange={(e, { value }) =>
                      updateModelConfigField(
                        detailEditingModelRow.upstream_model,
                        'price_unit',
                        value || 'per_1k_tokens',
                      )
                    }
                  />
                </AppField>
                <AppField label={t('channel.edit.model_selector.table.input_price')}>
                  <AppInputNumber
                    className='router-modal-input'
                    min={0}
                    step={0.000001}
                    precision={6}
                    fluid
                    placeholder='-'
                    value={detailEditingModelRow.input_price ?? ''}
                    onChange={(e, { value }) =>
                      updateModelConfigField(
                        detailEditingModelRow.upstream_model,
                        'input_price',
                        value,
                      )
                    }
                  />
                </AppField>
                <AppField label={t('channel.edit.model_selector.table.output_price')}>
                  <AppInputNumber
                    className='router-modal-input'
                    min={0}
                    step={0.000001}
                    precision={6}
                    fluid
                    placeholder='-'
                    value={detailEditingModelRow.output_price ?? ''}
                    onChange={(e, { value }) =>
                      updateModelConfigField(
                        detailEditingModelRow.upstream_model,
                        'output_price',
                        value,
                      )
                    }
                  />
                </AppField>
              </AppFormRow>
            )}
          </div>
        </div>
      ) : null}
    </AppModal>
  );
};

export default ChannelModelEditorModal;
