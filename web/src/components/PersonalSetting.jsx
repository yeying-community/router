import React, { useContext, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Form,
  Header,
  Input,
  Modal,
  Segment,
} from 'semantic-ui-react';
import { API, showError, showSuccess } from '../helpers';
import { UserContext } from '../context/User';

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
  const [isEditingUsername, setIsEditingUsername] = useState(false);
  const [profileSubmitting, setProfileSubmitting] = useState(false);
  const [passwordModal, setPasswordModal] = useState(defaultPasswordModal);

  useEffect(() => {
    setUsername(currentUser?.username || '');
    setIsEditingUsername(false);
  }, [currentUser?.username]);

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
        display_name: currentUser?.display_name || '',
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
          display_name: currentUser?.display_name || '',
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
      <Segment>
        <Header as='h3' className='router-section-title'>
          账户信息
        </Header>
        <Form>
          <Form.Field>
            <label>钱包地址</label>
            <Input
              className='router-section-input'
              value={walletAddress}
              readOnly
            />
          </Form.Field>
          <Form.Field>
            <label>{t('user.edit.username')}</label>
            <div className='router-setting-inline-row'>
              <Input
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
                    <Button
                      className='router-section-button'
                      type='button'
                      onClick={cancelUsernameEdit}
                      disabled={profileSubmitting}
                    >
                      {t('common.cancel', '取消')}
                    </Button>
                    <Button
                      className='router-section-button'
                      type='button'
                      primary
                      loading={profileSubmitting}
                      disabled={(username || '').trim() === (currentUser?.username || '').trim()}
                      onClick={submitUsername}
                    >
                      保存
                    </Button>
                  </>
                ) : (
                  <Button
                    className='router-section-button'
                    type='button'
                    onClick={() => setIsEditingUsername(true)}
                  >
                    编辑
                  </Button>
                )}
              </div>
            </div>
          </Form.Field>
          <Form.Field>
            <label>密码</label>
            <div className='router-setting-inline-row'>
              <Input
                className='router-section-input'
                value={hasPassword ? '已设置' : '未设置'}
                readOnly
              />
              <div className='router-setting-inline-actions'>
                <Button
                  className='router-section-button'
                  type='button'
                  primary
                  onClick={() => openPasswordModal(hasPassword ? 'modify' : 'set')}
                >
                  {hasPassword ? '修改密码' : '设置密码'}
                </Button>
              </div>
            </div>
          </Form.Field>
        </Form>
      </Segment>

      <Modal size='tiny' open={passwordModal.open} onClose={closePasswordModal}>
        <Modal.Header>
          {passwordModal.mode === 'modify' ? '修改密码' : '设置密码'}
        </Modal.Header>
        <Modal.Content>
          <Form>
            {passwordModal.mode === 'modify' ? (
              <Form.Field>
                <label>当前密码</label>
                <Input
                  type='password'
                  value={passwordModal.currentPassword}
                  onChange={(e, { value }) =>
                    updatePasswordModalField('currentPassword', value)
                  }
                  autoComplete='current-password'
                />
              </Form.Field>
            ) : null}
            <Form.Field>
              <label>新密码</label>
              <Input
                type='password'
                value={passwordModal.newPassword}
                onChange={(e, { value }) =>
                  updatePasswordModalField('newPassword', value)
                }
                autoComplete='new-password'
              />
            </Form.Field>
            <Form.Field>
              <label>确认新密码</label>
              <Input
                type='password'
                value={passwordModal.confirmPassword}
                onChange={(e, { value }) =>
                  updatePasswordModalField('confirmPassword', value)
                }
                autoComplete='new-password'
              />
            </Form.Field>
          </Form>
        </Modal.Content>
        <Modal.Actions>
          <Button className='router-modal-button' onClick={closePasswordModal}>
            {t('common.cancel', '取消')}
          </Button>
          <Button
            className='router-modal-button'
            primary
            loading={passwordModal.submitting}
            onClick={submitPassword}
          >
            {passwordModal.mode === 'modify' ? '确认修改' : '确认设置'}
          </Button>
        </Modal.Actions>
      </Modal>
    </div>
  );
};

export default PersonalSetting;
