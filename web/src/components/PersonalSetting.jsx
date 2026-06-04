import React, { useContext, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { API, showError, showSuccess } from '../helpers';
import { UserContext } from '../context/User';
import {
  AppButton,
  AppField,
  AppFormRow,
  AppInput,
  AppModal,
  AppSection,
} from '../router-ui';

const defaultPasswordModal = {
  open: false,
  mode: 'set',
  currentPassword: '',
  newPassword: '',
  confirmPassword: '',
  submitting: false,
};

const normalizeUser = (user) => {
  if (!user || typeof user !== 'object') return null;
  return {
    ...user,
    display_name: user.display_name ?? user.displayName ?? '',
  };
};

const PersonalSetting = () => {
  const { t } = useTranslation();
  const [userState, userDispatch] = useContext(UserContext);
  const currentUser = useMemo(() => {
    const contextUser = normalizeUser(userState?.user);
    if (contextUser) return contextUser;
    const cached = localStorage.getItem('user');
    if (!cached) return null;
    try {
      return normalizeUser(JSON.parse(cached));
    } catch (error) {
      return null;
    }
  }, [userState?.user]);

  const [username, setUsername] = useState('');
  const [email, setEmail] = useState('');
  const [isEditingUsername, setIsEditingUsername] = useState(false);
  const [isEditingEmail, setIsEditingEmail] = useState(false);
  const [profileSubmitting, setProfileSubmitting] = useState(false);
  const [passwordModal, setPasswordModal] = useState(defaultPasswordModal);

  useEffect(() => {
    setUsername(currentUser?.username || '');
    setEmail(currentUser?.email || '');
    setIsEditingUsername(false);
    setIsEditingEmail(false);
  }, [currentUser?.email, currentUser?.username]);

  useEffect(() => {
    if (!currentUser || typeof currentUser.has_password === 'boolean') {
      return;
    }
    syncCurrentUser();
  }, [currentUser]);

  const walletAddress = currentUser?.wallet_address || '-';
  const hasPassword = currentUser?.has_password === true;

  const syncCurrentUser = async () => {
    const res = await API.get('/api/v1/public/user/self');
    const { success, message, data } = res.data || {};
    if (!success) {
      showError(message || t('user.messages.load_failed', '加载失败'));
      return false;
    }
    const nextUser = normalizeUser(data);
    userDispatch({ type: 'login', payload: nextUser });
    localStorage.setItem('user', JSON.stringify(nextUser));
    return true;
  };

  const submitUsername = async () => {
    const trimmedUsername = (username || '').trim();
    if (!trimmedUsername) {
      showError(t('user.edit.username_placeholder'));
      return;
    }
    if (!currentUser?.username) {
      showError('用户信息不存在');
      return;
    }
    setProfileSubmitting(true);
    try {
      const res = await API.put('/api/v1/public/user/self', {
        username: trimmedUsername,
        password: '',
      });
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('user.messages.update_failed', '更新失败'));
        return;
      }
      await syncCurrentUser();
      setIsEditingUsername(false);
      showSuccess(t('user.messages.update_success'));
    } finally {
      setProfileSubmitting(false);
    }
  };

  const cancelUsernameEdit = () => {
    setUsername(currentUser?.username || '');
    setIsEditingUsername(false);
  };

  const submitEmail = async () => {
    const trimmedEmail = (email || '').trim();
    if (!trimmedEmail) {
      showError('请输入邮箱地址');
      return;
    }
    setProfileSubmitting(true);
    try {
      const res = await API.put('/api/v1/public/user/self', {
        username: currentUser?.username || '',
        password: '',
        email: trimmedEmail,
      });
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('user.messages.update_failed', '更新失败'));
        return;
      }
      await syncCurrentUser();
      setIsEditingEmail(false);
      showSuccess(t('user.messages.update_success'));
    } finally {
      setProfileSubmitting(false);
    }
  };

  const cancelEmailEdit = () => {
    setEmail(currentUser?.email || '');
    setIsEditingEmail(false);
  };

  const openPasswordModal = (mode) => {
    setPasswordModal({
      ...defaultPasswordModal,
      open: true,
      mode,
    });
  };

  const closePasswordModal = () => {
    if (passwordModal.submitting) return;
    setPasswordModal(defaultPasswordModal);
  };

  const updatePasswordModalField = (name, value) => {
    setPasswordModal((prev) => ({
      ...prev,
      [name]: value,
    }));
  };

  const submitPassword = async () => {
    const isModify = passwordModal.mode === 'modify';
    const currentPassword = passwordModal.currentPassword || '';
    const newPassword = passwordModal.newPassword || '';
    const confirmPassword = passwordModal.confirmPassword || '';

    if (isModify && currentPassword.length < 8) {
      showError('请输入当前密码');
      return;
    }
    if (newPassword.length < 8) {
      showError(t('messages.error.password_length'));
      return;
    }
    if (newPassword !== confirmPassword) {
      showError(t('messages.error.password_mismatch'));
      return;
    }

    setPasswordModal((prev) => ({ ...prev, submitting: true }));
    try {
      let res;
      if (isModify) {
        res = await API.post('/api/v1/public/user/self/password', {
          current_password: currentPassword,
          new_password: newPassword,
        });
      } else {
        res = await API.put('/api/v1/public/user/self', {
          username: currentUser?.username || '',
          password: newPassword,
        });
      }
      const { success, message } = res.data || {};
      if (!success) {
        showError(message || t('user.messages.update_failed', '更新失败'));
        return;
      }
      showSuccess(isModify ? '密码修改成功' : '密码设置成功');
      setPasswordModal(defaultPasswordModal);
    } finally {
      setPasswordModal((prev) =>
        prev.open ? { ...prev, submitting: false } : defaultPasswordModal
      );
    }
  };

  return (
    <div className='router-page-stack'>
      <AppSection title='账户信息'>
        <div className='router-page-stack'>
          <AppField label='钱包地址'>
            <AppInput
              className='router-section-input'
              value={walletAddress}
              readOnly
            />
          </AppField>
          <AppField label={t('user.edit.username')}>
            <div className='router-setting-inline-row'>
              <AppInput
                className='router-section-input'
                name='username'
                placeholder={t('user.edit.username_placeholder')}
                value={username}
                readOnly={!isEditingUsername}
                onChange={(e, { value }) => setUsername(value)}
              />
              <div className='router-setting-inline-actions'>
                {isEditingUsername ? (
                  <>
                    <AppButton
                      className='router-section-button'
                      type='button'
                      onClick={cancelUsernameEdit}
                      disabled={profileSubmitting}
                    >
                      {t('common.cancel', '取消')}
                    </AppButton>
                    <AppButton
                      className='router-section-button'
                      type='button'
                      color='blue'
                      loading={profileSubmitting}
                      disabled={(username || '').trim() === (currentUser?.username || '').trim()}
                      onClick={submitUsername}
                    >
                      保存
                    </AppButton>
                  </>
                ) : (
                  <AppButton
                    className='router-section-button'
                    type='button'
                    onClick={() => setIsEditingUsername(true)}
                  >
                    编辑
                  </AppButton>
                )}
              </div>
            </div>
          </AppField>
          <AppField label='邮箱'>
            <div className='router-setting-inline-row'>
              <AppInput
                className='router-section-input'
                type='email'
                placeholder='请输入邮箱地址'
                value={email}
                readOnly={!isEditingEmail}
                onChange={(e, { value }) => setEmail(value)}
              />
              <div className='router-setting-inline-actions'>
                {isEditingEmail ? (
                  <>
                    <AppButton
                      className='router-section-button'
                      type='button'
                      onClick={cancelEmailEdit}
                      disabled={profileSubmitting}
                    >
                      {t('common.cancel', '取消')}
                    </AppButton>
                    <AppButton
                      className='router-section-button'
                      type='button'
                      color='blue'
                      loading={profileSubmitting}
                      disabled={
                        (email || '').trim() === '' ||
                        (email || '').trim() ===
                          (currentUser?.email || '').trim()
                      }
                      onClick={submitEmail}
                    >
                      保存
                    </AppButton>
                  </>
                ) : (
                  <AppButton
                    className='router-section-button'
                    type='button'
                    onClick={() => setIsEditingEmail(true)}
                  >
                    {(currentUser?.email || '').trim() ? '编辑' : '设置'}
                  </AppButton>
                )}
              </div>
            </div>
          </AppField>
          <AppField label='密码'>
            <div className='router-setting-inline-row'>
              <AppInput
                className='router-section-input'
                value={hasPassword ? '已设置' : '未设置'}
                readOnly
              />
              <div className='router-setting-inline-actions'>
                <AppButton
                  className='router-section-button'
                  type='button'
                  color='blue'
                  onClick={() => openPasswordModal(hasPassword ? 'modify' : 'set')}
                >
                  {hasPassword ? '修改密码' : '设置密码'}
                </AppButton>
              </div>
            </div>
          </AppField>
        </div>
      </AppSection>

      <AppModal
        size='tiny'
        open={passwordModal.open}
        onClose={closePasswordModal}
        title={passwordModal.mode === 'modify' ? '修改密码' : '设置密码'}
        footer={[
          <AppButton key='cancel' className='router-modal-button' onClick={closePasswordModal}>
            {t('common.cancel', '取消')}
          </AppButton>,
          <AppButton
            key='confirm'
            className='router-modal-button'
            color='blue'
            loading={passwordModal.submitting}
            onClick={submitPassword}
          >
            {passwordModal.mode === 'modify' ? '确认修改' : '确认设置'}
          </AppButton>,
        ]}
      >
        <div className='router-page-stack'>
          {passwordModal.mode === 'modify' ? (
            <AppFormRow className='router-modal-form-row'>
              <AppField label='当前密码'>
                <AppInput
                  type='password'
                  value={passwordModal.currentPassword}
                  onChange={(e, { value }) =>
                    updatePasswordModalField('currentPassword', value)
                  }
                  autoComplete='current-password'
                />
              </AppField>
            </AppFormRow>
          ) : null}
          <AppFormRow className='router-modal-form-row'>
            <AppField label='新密码'>
              <AppInput
                type='password'
                value={passwordModal.newPassword}
                onChange={(e, { value }) =>
                  updatePasswordModalField('newPassword', value)
                }
                autoComplete='new-password'
              />
            </AppField>
          </AppFormRow>
          <AppFormRow className='router-modal-form-row'>
            <AppField label='确认新密码'>
              <AppInput
                type='password'
                value={passwordModal.confirmPassword}
                onChange={(e, { value }) =>
                  updatePasswordModalField('confirmPassword', value)
                }
                autoComplete='new-password'
              />
            </AppField>
          </AppFormRow>
        </div>
      </AppModal>
    </div>
  );
};

export default PersonalSetting;
