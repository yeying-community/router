import React from 'react';
import {
  AppAlert,
  AppButton,
  AppField,
  AppFormActions,
  AppFormRow,
  AppInput,
  AppModal,
  AppSelect,
  AppSwitch,
  AppTextarea,
} from '../../../router-ui';

const ChannelEndpointPolicyEditorModal = ({
  t,
  open,
  onClose,
  policyEditorSaving,
  endpointPolicyTemplates,
  selectedPolicyTemplate,
  setSelectedPolicyTemplate,
  applyEndpointPolicyTemplate,
  policyDraft,
  setPolicyDraft,
  saveEndpointPolicy,
}) => {
  return (
    <AppModal
      size='large'
      open={open}
      onClose={onClose}
      closeOnDimmerClick={!policyEditorSaving}
      title={t('channel.edit.endpoint_policies.editor.title')}
      footer={
        <AppFormActions>
          <AppButton
            type='button'
            className='router-modal-button'
            onClick={onClose}
            disabled={policyEditorSaving}
          >
            {t('channel.edit.buttons.cancel')}
          </AppButton>
          <AppButton
            type='button'
            className='router-modal-button'
            color='blue'
            loading={policyEditorSaving}
            disabled={policyEditorSaving}
            onClick={saveEndpointPolicy}
          >
            {t('channel.edit.buttons.save')}
          </AppButton>
        </AppFormActions>
      }
    >
      <div className='router-modal-scroll-body'>
        <div className='router-block-gap'>
          <AppAlert
            type='info'
            showIcon
            className='router-section-message'
            title={t('channel.edit.endpoint_policies.editor.hint')}
          />
          <AppFormRow>
            <AppField label={t('channel.edit.endpoint_policies.editor.template')}>
              <AppSelect
                clearable
                className='router-modal-dropdown'
                fluid
                options={endpointPolicyTemplates}
                value={selectedPolicyTemplate}
                placeholder={t(
                  'channel.edit.endpoint_policies.editor.template_placeholder',
                )}
                onChange={(e, { value }) => {
                  const nextValue = (value || '').toString();
                  if (nextValue === '') {
                    setSelectedPolicyTemplate('');
                    setPolicyDraft((prev) => ({
                      ...prev,
                      template_key: '',
                    }));
                    return;
                  }
                  applyEndpointPolicyTemplate(nextValue);
                }}
              />
            </AppField>
          </AppFormRow>
          {(policyDraft.template_key || '') === 'IMAGE_URL_TO_BASE64' ? (
            <AppAlert
              type='warning'
              showIcon
              className='router-section-message'
              title={t('channel.edit.endpoint_policies.editor.image_url_to_base64_hint')}
            />
          ) : null}
          <AppFormRow>
            <AppField
              label={t('channel.edit.endpoint_policies.table.model')}
              readOnly
            >
              <AppInput
                className='router-modal-input'
                value={policyDraft.model}
                readOnly
              />
            </AppField>
            <AppField
              label={t('channel.edit.endpoint_policies.table.endpoint')}
              readOnly
            >
              <AppInput
                className='router-modal-input'
                value={policyDraft.endpoint}
                readOnly
              />
            </AppField>
          </AppFormRow>
          <AppFormRow>
            <AppField
              label={t('channel.edit.endpoint_policies.editor.template_key')}
              readOnly
            >
              <AppInput
                className='router-modal-input'
                value={policyDraft.template_key || ''}
                readOnly
                placeholder={t(
                  'channel.edit.endpoint_policies.editor.template_key_placeholder',
                )}
              />
            </AppField>
          </AppFormRow>
          <AppFormRow>
            <AppField label={t('channel.edit.endpoint_policies.table.status')}>
              <AppSwitch
                checked={policyDraft.enabled === true}
                onChange={(_, { checked }) =>
                  setPolicyDraft((prev) => ({
                    ...prev,
                    enabled: checked === true,
                  }))
                }
              />
            </AppField>
          </AppFormRow>
          <AppFormRow>
            <AppField label={t('channel.edit.endpoint_policies.table.reason')}>
              <AppTextarea
                className='router-section-textarea router-code-textarea router-code-textarea-sm'
                value={policyDraft.reason}
                onChange={(e, { value }) =>
                  setPolicyDraft((prev) => ({
                    ...prev,
                    reason: value || '',
                  }))
                }
              />
            </AppField>
          </AppFormRow>
          <AppFormRow>
            <AppField
              label={t('channel.edit.endpoint_policies.editor.capabilities')}
            >
              <AppTextarea
                className='router-section-textarea router-code-textarea router-code-textarea-md'
                placeholder='{"input_image_url": false}'
                value={policyDraft.capabilities}
                onChange={(e, { value }) =>
                  setPolicyDraft((prev) => ({
                    ...prev,
                    capabilities: value || '',
                  }))
                }
              />
            </AppField>
          </AppFormRow>
          <AppFormRow>
            <AppField
              label={t('channel.edit.endpoint_policies.editor.request_policy')}
            >
              <AppTextarea
                className='router-section-textarea router-code-textarea router-code-textarea-md'
                placeholder='{"actions":[{"type":"image_url_to_base64","input_types":["anthropic.image_url","openai.image_url","responses.input_image_url"]}]}'
                value={policyDraft.request_policy}
                onChange={(e, { value }) =>
                  setPolicyDraft((prev) => ({
                    ...prev,
                    request_policy: value || '',
                  }))
                }
              />
            </AppField>
          </AppFormRow>
          <AppFormRow>
            <AppField
              label={t('channel.edit.endpoint_policies.editor.response_policy')}
            >
              <AppTextarea
                className='router-section-textarea router-code-textarea router-code-textarea-md'
                placeholder='{}'
                value={policyDraft.response_policy}
                onChange={(e, { value }) =>
                  setPolicyDraft((prev) => ({
                    ...prev,
                    response_policy: value || '',
                  }))
                }
              />
            </AppField>
          </AppFormRow>
        </div>
      </div>
    </AppModal>
  );
};

export default ChannelEndpointPolicyEditorModal;
