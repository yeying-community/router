import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../helpers';
import {
  AppAlert,
  AppButton,
  AppDivider,
  AppField,
  AppFilterHeader,
  AppFormActions,
  AppFormRow,
  AppInput,
  AppSwitch,
  AppSpin,
} from '../router-ui';

const SystemSetting = ({ section = '' }) => {
  const { t } = useTranslation();
  const [inputs, setInputs] = useState({
    PasswordLoginEnabled: 'true',
    PasswordRegisterEnabled: 'true',
    RegisterEnabled: 'true',
    SMTPServer: '',
    SMTPPort: '',
    SMTPAccount: '',
    SMTPFrom: '',
    SMTPToken: '',
    SystemName: '',
    Logo: '',
  });
  const [loading, setLoading] = useState(false);
  const normalizedSection = (section || '').trim().toLowerCase();
  const showAllSections =
    normalizedSection === '' || normalizedSection === 'all';
  const sectionVisible = {
    general: showAllSections || normalizedSection === 'general',
    smtp: showAllSections || normalizedSection === 'smtp',
    login: showAllSections || normalizedSection === 'login',
  };
  const sectionOrder = ['general', 'smtp', 'login'];
  const shouldRenderDividerAfter = (key) => {
    if (!showAllSections) {
      return false;
    }
    const index = sectionOrder.indexOf(key);
    if (index < 0) {
      return false;
    }
    return sectionOrder
      .slice(index + 1)
      .some((nextKey) => Boolean(sectionVisible[nextKey]));
  };

  const loadOptions = async () => {
    const res = await API.get('/api/v1/admin/option/');
    const { success, message, data } = res.data;
    if (!success) {
      showError(message);
      return;
    }
    const map = {};
    data.forEach((item) => {
      map[item.key] = item.value;
    });
    setInputs((prev) => ({ ...prev, ...map }));
  };

  useEffect(() => {
    loadOptions();
  }, []);

  const updateOption = async (key, value) => {
    const res = await API.put('/api/v1/admin/option/', { key, value });
    const { success, message } = res.data;
    if (!success) {
      showError(message);
      return false;
    }
    setInputs((prev) => ({ ...prev, [key]: value }));
    return true;
  };

  const submitAuth = async () => {
    setLoading(true);
    await updateOption('PasswordLoginEnabled', inputs.PasswordLoginEnabled);
    await updateOption('PasswordRegisterEnabled', inputs.PasswordRegisterEnabled);
    await updateOption('RegisterEnabled', inputs.RegisterEnabled);
    setLoading(false);
    showSuccess(t('setting.system.saved', '已保存'));
  };

  const submitSMTP = async () => {
    setLoading(true);
    await updateOption('SMTPServer', inputs.SMTPServer);
    await updateOption('SMTPPort', inputs.SMTPPort);
    await updateOption('SMTPAccount', inputs.SMTPAccount);
    await updateOption('SMTPFrom', inputs.SMTPFrom);
    await updateOption('SMTPToken', inputs.SMTPToken);
    setLoading(false);
    showSuccess(t('setting.system.saved', '已保存'));
  };

  const submitGeneral = async () => {
    setLoading(true);
    await updateOption('SystemName', inputs.SystemName);
    await updateOption('Logo', inputs.Logo);
    setLoading(false);
    showSuccess(t('setting.system.saved', '已保存'));
  };

  const handleChange = (_, { name, value }) => {
    setInputs((prev) => ({ ...prev, [name]: value }));
  };

  return (
    <AppSpin spinning={loading}>
      <div className='router-settings-system-block'>
      {sectionVisible.general ? (
        <>
          <AppFilterHeader
            title={t('setting.system.general.title')}
            titleClassName='router-ui-section-title'
            className='router-toolbar-compact'
          />
          <div className='router-settings-section-body'>
            <AppFormRow>
              <AppField label={t('setting.system.general.system_name')}>
                <AppInput
                  className='router-section-input'
                  name='SystemName'
                  value={inputs.SystemName}
                  onChange={handleChange}
                />
              </AppField>
              <AppField label={t('setting.system.general.logo')}>
                <AppInput
                  className='router-section-input'
                  name='Logo'
                  value={inputs.Logo}
                  onChange={handleChange}
                />
              </AppField>
            </AppFormRow>
            <AppFormActions align='start'>
              <AppButton className='router-section-button' color='blue' onClick={submitGeneral}>
                {t('setting.system.buttons.save')}
              </AppButton>
            </AppFormActions>
          </div>
          {shouldRenderDividerAfter('general') ? <AppDivider /> : null}
        </>
      ) : null}

      {sectionVisible.smtp ? (
        <>
          <AppFilterHeader
            title={t('setting.system.smtp.title')}
            titleClassName='router-ui-section-title'
            className='router-toolbar-compact'
          />
          <AppAlert
            className='router-section-message router-settings-inline-message'
            type='info'
            showIcon
            title={t('setting.system.smtp.subtitle')}
          />
          <div className='router-settings-section-body'>
            <AppFormRow>
              <AppField label={t('setting.system.smtp.server')}>
                <AppInput
                  className='router-section-input'
                  placeholder={t('setting.system.smtp.server_placeholder')}
                  name='SMTPServer'
                  onChange={handleChange}
                  value={inputs.SMTPServer}
                />
              </AppField>
              <AppField label={t('setting.system.smtp.port')}>
                <AppInput
                  className='router-section-input'
                  placeholder={t('setting.system.smtp.port_placeholder')}
                  name='SMTPPort'
                  onChange={handleChange}
                  value={inputs.SMTPPort}
                />
              </AppField>
              <AppField label={t('setting.system.smtp.account')}>
                <AppInput
                  className='router-section-input'
                  placeholder={t('setting.system.smtp.account_placeholder')}
                  name='SMTPAccount'
                  onChange={handleChange}
                  value={inputs.SMTPAccount}
                />
              </AppField>
            </AppFormRow>
            <AppFormRow>
              <AppField label={t('setting.system.smtp.from')}>
                <AppInput
                  className='router-section-input'
                  placeholder={t('setting.system.smtp.from_placeholder')}
                  name='SMTPFrom'
                  onChange={handleChange}
                  value={inputs.SMTPFrom}
                />
              </AppField>
              <AppField label={t('setting.system.smtp.token')}>
                <AppInput
                  className='router-section-input'
                  placeholder={t('setting.system.smtp.token_placeholder')}
                  name='SMTPToken'
                  onChange={handleChange}
                  value={inputs.SMTPToken}
                />
              </AppField>
            </AppFormRow>
            <AppFormActions align='start'>
              <AppButton className='router-section-button' color='blue' onClick={submitSMTP}>
                {t('setting.system.buttons.save')}
              </AppButton>
            </AppFormActions>
          </div>
          {shouldRenderDividerAfter('smtp') ? <AppDivider /> : null}
        </>
      ) : null}

      {sectionVisible.login ? (
        <>
          <AppFilterHeader
            title={t('setting.system.login.title')}
            titleClassName='router-ui-section-title'
            className='router-toolbar-compact'
          />
          <div className='router-settings-section-body router-page-stack'>
            <AppFormRow>
              <AppField label={t('setting.system.login.password_login')}>
                <AppSwitch
                  checked={
                    inputs.PasswordLoginEnabled === 'true' ||
                    inputs.PasswordLoginEnabled === true
                  }
                  onChange={(_, { checked }) =>
                    handleChange(null, {
                      name: 'PasswordLoginEnabled',
                      value: checked ? 'true' : 'false',
                    })
                  }
                />
              </AppField>
              <AppField label={t('setting.system.login.password_register')}>
                <AppSwitch
                  checked={
                    inputs.PasswordRegisterEnabled === 'true' ||
                    inputs.PasswordRegisterEnabled === true
                  }
                  onChange={(_, { checked }) =>
                    handleChange(null, {
                      name: 'PasswordRegisterEnabled',
                      value: checked ? 'true' : 'false',
                    })
                  }
                />
              </AppField>
              <AppField label={t('setting.system.login.registration')}>
                <AppSwitch
                  checked={
                    inputs.RegisterEnabled === 'true' ||
                    inputs.RegisterEnabled === true
                  }
                  onChange={(_, { checked }) =>
                    handleChange(null, {
                      name: 'RegisterEnabled',
                      value: checked ? 'true' : 'false',
                    })
                  }
                />
              </AppField>
            </AppFormRow>
            <AppFormActions align='start'>
              <AppButton className='router-section-button' onClick={submitAuth}>
                {t('setting.system.buttons.save')}
              </AppButton>
            </AppFormActions>
          </div>
        </>
      ) : null}
      </div>
    </AppSpin>
  );
};

export default SystemSetting;
