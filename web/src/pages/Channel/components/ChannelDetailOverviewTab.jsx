import React from 'react';
import {
  AppButton,
  AppDetailSection,
  AppField,
  AppFormRow,
  AppInput,
  AppSelect,
} from '../../../router-ui';

const billingModeOptions = (t) => [
  { value: 'unsupported', label: t('channel.edit.billing.modes.unsupported') },
  { value: 'manual', label: t('channel.edit.billing.modes.manual') },
  { value: 'builtin_openai', label: t('channel.edit.billing.modes.builtin_openai') },
  { value: 'builtin_closeai', label: t('channel.edit.billing.modes.builtin_closeai') },
  { value: 'builtin_openai_sb', label: t('channel.edit.billing.modes.builtin_openai_sb') },
  { value: 'builtin_aiproxy', label: t('channel.edit.billing.modes.builtin_aiproxy') },
  { value: 'builtin_api2gpt', label: t('channel.edit.billing.modes.builtin_api2gpt') },
  { value: 'builtin_aigc2d', label: t('channel.edit.billing.modes.builtin_aigc2d') },
  { value: 'builtin_siliconflow', label: t('channel.edit.billing.modes.builtin_siliconflow') },
  { value: 'builtin_deepseek', label: t('channel.edit.billing.modes.builtin_deepseek') },
  { value: 'builtin_openrouter', label: t('channel.edit.billing.modes.builtin_openrouter') },
  { value: 'builtin_cdk', label: t('channel.edit.billing.modes.builtin_cdk') },
];

const ChannelDetailOverviewTab = ({
  t,
  inputs,
  currentProtocolOption,
  channelProtocolOptions,
  detailBasicEditing,
  detailBasicSaving,
  detailBasicEditLocked,
  detailBasicReadonly,
  channelIdentifierMaxLength,
  handleInputChange,
  cancelDetailBasicEdit,
  saveDetailBasicInfo,
  setDetailBasicEditing,
  basicConnectionFields,
  addressRoutingFields,
  protocolSelectionHintContent,
  protocolSpecificFields,
  timestamp2string,
  billingProfile,
  detailBillingEditing,
  detailBillingDraft,
  billingSubmitting,
  detailBillingEditLocked,
  setDetailBillingEditing,
  onUpdateBillingProfileDraft,
  onCancelBillingProfileEdit,
  onSaveBillingProfile,
}) => {
  const billingReadonly = !detailBillingEditing || billingSubmitting;
  const activeBillingMode = (
    detailBillingEditing
      ? detailBillingDraft?.billing_mode
      : billingProfile?.billing_mode
  ) || 'unsupported';
  const showCDKBillingConfig = activeBillingMode === 'builtin_cdk';

  return (
    <>
      <AppDetailSection
        title={t('channel.edit.detail_basic_title')}
        titleTag='span'
        headerEnd={
          detailBasicEditing ? (
            <>
              <AppButton
                type='button'
                className='router-page-button'
                onClick={cancelDetailBasicEdit}
                disabled={detailBasicSaving}
              >
                {t('channel.edit.buttons.cancel')}
              </AppButton>
              <AppButton
                type='button'
                className='router-page-button'
                color='blue'
                loading={detailBasicSaving}
                disabled={detailBasicSaving}
                onClick={saveDetailBasicInfo}
              >
                {t('channel.edit.buttons.save')}
              </AppButton>
            </>
          ) : (
            <AppButton
              type='button'
              className='router-page-button'
              color='blue'
              disabled={detailBasicEditLocked}
              onClick={() => setDetailBasicEditing(true)}
            >
              {t('common.edit')}
            </AppButton>
          )
        }
      >
        <AppFormRow>
          <AppField label={t('channel.edit.id')} readOnly>
            <AppInput
              className='router-section-input'
              value={inputs.id || '-'}
              readOnly
            />
          </AppField>
          <AppField
            label={t('channel.edit.identifier')}
            required
            readOnly={detailBasicReadonly}
          >
            <AppInput
              className='router-section-input'
              name='name'
              placeholder={t('channel.edit.identifier_placeholder')}
              onChange={handleInputChange}
              value={inputs.name}
              maxLength={channelIdentifierMaxLength}
              readOnly={detailBasicReadonly}
            />
          </AppField>
          <AppField
            label={t('channel.edit.type')}
            required={!detailBasicReadonly}
            readOnly={detailBasicReadonly}
          >
            {detailBasicReadonly ? (
              <AppInput
                className='router-section-input'
                value={currentProtocolOption?.text || inputs.protocol || '-'}
                readOnly
              />
            ) : (
              <AppSelect
                className='router-section-dropdown'
                name='protocol'
                search
                options={channelProtocolOptions}
                value={inputs.protocol}
                onChange={handleInputChange}
              />
            )}
          </AppField>
        </AppFormRow>
        {protocolSelectionHintContent}
        {basicConnectionFields}
        {addressRoutingFields}
        {protocolSpecificFields}
        <AppFormRow>
          <AppField label={t('channel.edit.created_time')} readOnly>
            <AppInput
              className='router-section-input'
              value={
                inputs.created_time ? timestamp2string(inputs.created_time) : '-'
              }
              readOnly
            />
          </AppField>
          <AppField label={t('channel.edit.updated_at')} readOnly>
            <AppInput
              className='router-section-input'
              value={
                inputs.updated_at ? timestamp2string(inputs.updated_at) : '-'
              }
              readOnly
            />
          </AppField>
        </AppFormRow>
      </AppDetailSection>
      <AppDetailSection
        title={t('channel.edit.billing.profile_title')}
        titleTag='span'
        headerEnd={
          detailBillingEditing ? (
            <>
              <AppButton
                type='button'
                className='router-page-button'
                onClick={onCancelBillingProfileEdit}
                disabled={billingSubmitting}
              >
                {t('channel.edit.buttons.cancel')}
              </AppButton>
              <AppButton
                type='button'
                className='router-page-button'
                color='blue'
                loading={billingSubmitting}
                disabled={billingSubmitting}
                onClick={onSaveBillingProfile}
              >
                {t('channel.edit.buttons.save')}
              </AppButton>
            </>
          ) : (
            <AppButton
              type='button'
              className='router-page-button'
              color='blue'
              disabled={detailBillingEditLocked}
              onClick={() => setDetailBillingEditing(true)}
            >
              {t('common.edit')}
            </AppButton>
          )
        }
      >
        <AppFormRow>
          <AppField label={t('channel.edit.billing.billing_mode')}>
            {detailBillingEditing ? (
              <AppSelect
                className='router-section-input'
                options={billingModeOptions(t)}
                value={detailBillingDraft?.billing_mode || 'unsupported'}
                onChange={(e, { value }) =>
                  onUpdateBillingProfileDraft({
                    billing_mode: (value || 'unsupported').toString(),
                  })
                }
                disabled={billingSubmitting}
              />
            ) : (
              <AppInput
                className='router-section-input'
                value={
                  billingProfile?.billing_mode
                    ? t(
                        `channel.edit.billing.modes.${billingProfile.billing_mode}`,
                        { defaultValue: billingProfile.billing_mode },
                      )
                    : '-'
                }
                readOnly
              />
            )}
          </AppField>
          <AppField label={t('channel.edit.billing.billing_api_base_url')}>
            <AppInput
              className='router-section-input'
              value={
                detailBillingEditing
                  ? detailBillingDraft?.billing_api_base_url || ''
                  : billingProfile?.billing_api_base_url || '-'
              }
              onChange={(e, { value }) =>
                onUpdateBillingProfileDraft({
                  billing_api_base_url: (value || '').toString(),
                })
              }
              readOnly={billingReadonly}
            />
          </AppField>
        </AppFormRow>
        {showCDKBillingConfig && (
          <AppFormRow>
            <AppField label={t('channel.edit.billing.cdk')}>
              <AppInput
                className='router-section-input'
                value={
                  detailBillingEditing
                    ? detailBillingDraft?.cdk || ''
                    : billingProfile?.cdk || '-'
                }
                onChange={(e, { value }) =>
                  onUpdateBillingProfileDraft({
                    cdk: (value || '').toString(),
                  })
                }
                readOnly={billingReadonly}
              />
            </AppField>
            <AppField label={t('channel.edit.billing.currency')}>
              {detailBillingEditing ? (
                <AppSelect
                  className='router-section-input'
                  options={[
                    { value: 'USD', label: 'USD' },
                    { value: 'CNY', label: 'CNY' },
                  ]}
                  value={detailBillingDraft?.currency || 'USD'}
                  onChange={(e, { value }) =>
                    onUpdateBillingProfileDraft({
                      currency: (value || 'USD').toString(),
                    })
                  }
                  disabled={billingSubmitting}
                />
              ) : (
                <AppInput
                  className='router-section-input'
                  value={billingProfile?.currency || '-'}
                  readOnly
                />
              )}
            </AppField>
          </AppFormRow>
        )}
      </AppDetailSection>
    </>
  );
};

export default ChannelDetailOverviewTab;
