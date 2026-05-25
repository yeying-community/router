import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { API, showError, showSuccess } from '../../helpers';
import {
  AppButton,
  AppField,
  AppFilterHeader,
  AppFormActions,
  AppFormRow,
  AppInput,
  AppSection,
} from '../../router-ui';

const AddUser = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const originInputs = {
    username: '',
    password: '',
  };
  const [inputs, setInputs] = useState(originInputs);
  const { username, password } = inputs;

  const handleInputChange = (e, { name, value }) => {
    setInputs((inputs) => ({ ...inputs, [name]: value }));
  };

  const submit = async () => {
    if (inputs.username === '' || inputs.password === '') return;
    if (password.length < 8 || password.length > 20) {
      showError(t('messages.error.password_length_range'));
      return;
    }
    const res = await API.post(`/api/v1/admin/user/`, inputs);
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('user.messages.create_success'));
      setInputs(originInputs);
      navigate('/admin/user');
    } else {
      showError(message);
    }
  };

  return (
    <div className='dashboard-container'>
      <AppFilterHeader
        breadcrumbs={[
          { key: 'workspace', label: t('header.admin_workspace') },
          { key: 'business', label: t('header.business_operation') },
          { key: 'user', label: t('header.user') },
          { key: 'user-add', label: t('user.add.title'), active: true },
        ]}
        title={t('user.add.title')}
      />
      <AppSection title={t('user.add.title')}>
        <AppFormRow>
          <AppField label={t('user.edit.username')} required>
            <AppInput
              className='router-section-input'
              name='username'
              placeholder={t('user.edit.username_placeholder')}
              onChange={handleInputChange}
              value={username}
              autoComplete='off'
              required
            />
          </AppField>
        </AppFormRow>
        <AppFormRow>
          <AppField label={t('user.edit.password')} required>
            <AppInput
              className='router-section-input'
              name='password'
              type='password'
              placeholder={t('user.edit.password_placeholder')}
              onChange={handleInputChange}
              value={password}
              autoComplete='off'
              required
            />
          </AppField>
        </AppFormRow>
        <AppFormActions align='start' className='router-block-gap-sm'>
          <AppButton className='router-page-button' color='blue' type='submit' onClick={submit}>
            {t('user.edit.buttons.submit')}
          </AppButton>
        </AppFormActions>
      </AppSection>
    </div>
  );
};

export default AddUser;
