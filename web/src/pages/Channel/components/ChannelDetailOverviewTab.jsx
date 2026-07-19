import React from 'react';
import {
  AppButton,
  AppDetailSection,
  AppField,
  AppFormRow,
  AppInput,
  AppSelect,
} from '../../../router-ui';

const normalizeBillingSourceValue = (source) => {
  const normalizedSource = (source || '').toString().trim().toLowerCase();
  return normalizedSource === '' ||
    normalizedSource === 'unsupported' ||
    normalizedSource.startsWith('builtin_')
    ? 'manual'
    : normalizedSource;
};

const adapterNameSet = (adapters = []) =>
  new Set(
    (Array.isArray(adapters) ? adapters : [])
      .map((adapter) => (adapter?.name || '').toString().trim().toLowerCase())
      .filter(Boolean)
  );

const resolveBillingSourceSelectValue = (source, adapters = []) => {
  const normalizedSource = normalizeBillingSourceValue(source);
  if (normalizedSource === 'manual') {
    return normalizedSource;
  }
  return adapterNameSet(adapters).has(normalizedSource)
    ? normalizedSource
    : 'manual';
};

const buildBillingSourceOptions = (t, adapters = []) => {
  const options = [
    { value: 'manual', label: t('channel.edit.billing.sources.manual') },
  ];
  const seen = new Set(options.map((item) => item.value));
  (Array.isArray(adapters) ? adapters : []).forEach((adapter) => {
    const name = (adapter?.name || '').toString().trim().toLowerCase();
    if (name === '' || seen.has(name)) {
      return;
    }
    seen.add(name);
    options.push({ value: name, label: name });
  });
  return options;
};

const formatBillingSourceLabel = (t, source, adapters = []) => {
  const normalizedSource = resolveBillingSourceSelectValue(source, adapters);
  if (normalizedSource === 'manual') {
    return t(`channel.edit.billing.sources.${normalizedSource}`);
  }
  return normalizedSource;
};

const normalizeBillingCredentialFieldName = (name) =>
  (name || '').toString().trim().toLowerCase();

const normalizeBillingCredentials = (credentials) => {
  if (!credentials || typeof credentials !== 'object') {
    return {};
  }
  const result = {};
  Object.entries(credentials).forEach(([key, value]) => {
    const normalizedKey = normalizeBillingCredentialFieldName(key);
    const normalizedValue = (value || '').toString().trim();
    if (normalizedKey === '' || normalizedValue === '') {
      return;
    }
    result[normalizedKey] = normalizedValue;
  });
  return result;
};

const normalizeBillingCredentialFields = (fields) => {
  if (!Array.isArray(fields)) {
    return [];
  }
  const seen = new Set();
  return fields
    .filter((field) => field && typeof field === 'object')
    .map((field) => ({
      name: normalizeBillingCredentialFieldName(field.name),
      label: (field.label || '').toString().trim(),
      required: field.required === true,
      secret: field.secret !== false,
    }))
    .filter((field) => {
      if (field.name === '' || seen.has(field.name)) {
        return false;
      }
      seen.add(field.name);
      return true;
    });
};

const resolveBillingAdapterCredentialFields = (source, adapters = []) => {
  const normalizedSource = resolveBillingSourceSelectValue(source, adapters);
  if (normalizedSource === 'manual') {
    return [];
  }
  const adapter = (Array.isArray(adapters) ? adapters : []).find(
    (item) =>
      (item?.name || '').toString().trim().toLowerCase() === normalizedSource
  );
  return normalizeBillingCredentialFields(adapter?.credential_fields);
};

const billingCredentialLabel = (field) =>
  field.label || field.name || 'Credential';

const maskSecret = (value) => {
  const normalizedValue = typeof value === 'string' ? value.trim() : '';
  if (!normalizedValue) {
    return '-';
  }
  if (normalizedValue.length <= 6) {
    return '*'.repeat(normalizedValue.length);
  }
  return `${normalizedValue.slice(0, 3)}${'*'.repeat(
    normalizedValue.length - 6,
  )}${normalizedValue.slice(-3)}`;
};

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
  billingAdapters,
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
  const selectedBillingSource = detailBillingEditing
    ? detailBillingDraft?.billing_source
    : billingProfile?.billing_source;
  const billingCredentialFields = resolveBillingAdapterCredentialFields(
    selectedBillingSource,
    billingAdapters
  );
  const activeBillingCredentials = normalizeBillingCredentials(
    detailBillingEditing
      ? detailBillingDraft?.billing_credentials
      : billingProfile?.billing_credentials
  );

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
          <AppField label={t('channel.edit.billing.billing_source')}>
            {detailBillingEditing ? (
              <AppSelect
                className='router-section-input'
                options={buildBillingSourceOptions(t, billingAdapters)}
                value={resolveBillingSourceSelectValue(
                  detailBillingDraft?.billing_source,
                  billingAdapters
                )}
                onChange={(e, { value }) =>
                  onUpdateBillingProfileDraft({
                    billing_source: (value || 'manual').toString().trim(),
                    billing_credentials:
                      normalizeBillingSourceValue(value) === 'manual'
                        ? {}
                        : detailBillingDraft?.billing_credentials || {},
                  })
                }
                disabled={billingSubmitting}
              />
            ) : (
              <AppInput
                className='router-section-input'
                value={
                  formatBillingSourceLabel(
                    t,
                    billingProfile?.billing_source,
                    billingAdapters
                  )
                }
                readOnly
              />
            )}
          </AppField>
        </AppFormRow>
        {billingCredentialFields.length > 0 ? (
          <AppFormRow>
            {billingCredentialFields.map((field) => {
              const credentialValue = activeBillingCredentials[field.name] || '';
              return (
                <AppField
                  key={field.name}
                  label={billingCredentialLabel(field)}
                  required={detailBillingEditing && field.required}
                >
                  <AppInput
                    className='router-section-input'
                    type={
                      detailBillingEditing && field.secret
                        ? 'password'
                        : undefined
                    }
                    value={
                      detailBillingEditing
                        ? credentialValue
                        : field.secret
                          ? maskSecret(credentialValue)
                          : credentialValue || '-'
                    }
                    onChange={(e, { value }) =>
                      onUpdateBillingProfileDraft({
                        billing_credentials: {
                          ...normalizeBillingCredentials(
                            detailBillingDraft?.billing_credentials
                          ),
                          [field.name]: (value || '').toString(),
                        },
                      })
                    }
                    readOnly={billingReadonly}
                    autoComplete='new-password'
                  />
                </AppField>
              );
            })}
          </AppFormRow>
        ) : null}
        <div className='router-form-hint router-form-hint-section'>
          {t('channel.edit.billing.credential_hint')}
        </div>
      </AppDetailSection>
    </>
  );
};

export default ChannelDetailOverviewTab;
