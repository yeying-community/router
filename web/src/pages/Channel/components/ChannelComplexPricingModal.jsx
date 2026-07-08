import React from 'react';
import {
  AppButton,
  AppFormActions,
  AppModal,
  AppTable,
  AppTag,
  AppToolbar,
} from '../../../router-ui';

const ChannelComplexPricingModal = ({
  t,
  open,
  onClose,
  data,
  normalizeChannelModelType,
}) => {
  const buildPriceComponentRowKey = (detail, component) =>
    [
      detail?.provider || 'provider',
      detail?.model || 'model',
      component?.id || '',
      component?.component || 'component',
      component?.condition || 'condition',
      component?.currency || '',
      component?.price || component?.value || '',
    ].join('-');

  return (
    <AppModal
      size='large'
      open={open}
      onClose={onClose}
      title={t('channel.edit.model_selector.pricing_detail_title')}
      footer={
        <AppFormActions>
          <AppButton
            type='button'
            className='router-modal-button'
            onClick={onClose}
          >
            {t('channel.edit.buttons.cancel')}
          </AppButton>
        </AppFormActions>
      }
    >
      <div className='router-modal-scroll-body'>
        <div className='router-block-gap-sm'>
          <div className='router-text-meta'>
            {t('channel.edit.model_selector.pricing_detail_model', {
              model: data?.model || '-',
            })}
          </div>
        </div>
        {(data?.details || []).length === 0 ? (
          <div className='router-empty-cell'>
            {t('channel.edit.model_selector.pricing_detail_empty')}
          </div>
        ) : (
          (data?.details || []).map((detail, index) => (
            <div
              key={`${detail.provider || 'provider'}-${detail.model || 'model'}-${index}`}
              className='router-block-gap-sm router-complex-pricing-detail-block'
            >
              <AppToolbar
                className='router-block-gap-sm'
                startClassName='router-tag-group'
                start={
                  <>
                  <AppTag className='router-tag'>
                    {detail.provider || '-'}
                  </AppTag>
                  <AppTag className='router-tag'>
                    {detail.model || '-'}
                  </AppTag>
                  <AppTag className='router-tag'>
                    {t(
                      `channel.model_types.${normalizeChannelModelType(detail.type)}`,
                    )}
                  </AppTag>
                  {(detail.supported_endpoints || []).map((endpoint) => (
                    <AppTag
                      key={`${detail.provider || 'provider'}-${detail.model || 'model'}-${endpoint}`}
                      className='router-tag'
                    >
                      {endpoint}
                    </AppTag>
                  ))}
                  </>
                }
              />
              <AppTable
                className='router-detail-table'
                pagination={false}
                scroll={{ x: 980 }}
                rowKey={(component) =>
                  buildPriceComponentRowKey(detail, component)
                }
                dataSource={detail.price_components || []}
                columns={[
                  {
                    title: t('channel.edit.model_selector.pricing_detail_table.component'),
                    dataIndex: 'component',
                    key: 'component',
                    render: (value) => value || '-',
                  },
                  {
                    title: t('channel.edit.model_selector.pricing_detail_table.condition'),
                    dataIndex: 'condition',
                    key: 'condition',
                    render: (value) => value || '-',
                  },
                  {
                    title: t('channel.edit.model_selector.pricing_detail_table.input_price'),
                    dataIndex: 'input_price',
                    key: 'input_price',
                    render: (value) => value || 0,
                  },
                  {
                    title: t('channel.edit.model_selector.pricing_detail_table.output_price'),
                    dataIndex: 'output_price',
                    key: 'output_price',
                    render: (value) => value || 0,
                  },
                  {
                    title: t('channel.edit.model_selector.pricing_detail_table.price_unit'),
                    dataIndex: 'price_unit',
                    key: 'price_unit',
                    render: (value) => value || '-',
                  },
                  {
                    title: t('channel.edit.model_selector.pricing_detail_table.currency'),
                    dataIndex: 'currency',
                    key: 'currency',
                    render: (value) => value || 'USD',
                  },
                  {
                    title: t('channel.edit.model_selector.pricing_detail_table.source'),
                    dataIndex: 'source',
                    key: 'source',
                    render: (value) => value || 'manual',
                  },
                  {
                    title: t('channel.edit.model_selector.pricing_detail_table.source_url'),
                    dataIndex: 'source_url',
                    key: 'source_url',
                    render: (value) => value || '-',
                  },
                ]}
              />
            </div>
          ))
        )}
      </div>
    </AppModal>
  );
};

export default ChannelComplexPricingModal;
