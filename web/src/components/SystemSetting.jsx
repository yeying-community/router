import React, { useEffect, useState } from 'react';
import { Button, Divider, Form, Header, Message, Segment } from 'semantic-ui-react';
import { useTranslation } from 'react-i18next';
import { API, removeTrailingSlash, showError, showSuccess } from '../helpers';

const SystemSetting = () => {
  const { t } = useTranslation();
  const [inputs, setInputs] = useState({
    PasswordLoginEnabled: 'true',
    PasswordRegisterEnabled: 'true',
    RegisterEnabled: 'true',
    Notice: '',
    SMTPServer: '',
    SMTPPort: '',
    SMTPAccount: '',
    SMTPFrom: '',
    SMTPToken: '',
    ServerAddress: '',
    Footer: '',
    SystemName: '',
    Logo: '',
    TopUpLink: '',
    ChatLink: '',
  });
  const [loading, setLoading] = useState(false);

  const loadOptions = async () => {
    const res = await API.get('/api/option/');
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
    const res = await API.put('/api/option/', { key, value });
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
    await updateOption('Footer', inputs.Footer);
    await updateOption('ServerAddress', removeTrailingSlash(inputs.ServerAddress));
    await updateOption('TopUpLink', inputs.TopUpLink);
    await updateOption('ChatLink', inputs.ChatLink);
    setLoading(false);
    showSuccess(t('setting.system.saved', '已保存'));
  };

  const submitNotice = async () => {
    setLoading(true);
    await updateOption('Notice', inputs.Notice);
    setLoading(false);
    showSuccess(t('setting.system.saved', '已保存'));
  };

  const handleChange = (_, { name, value }) => {
    setInputs((prev) => ({ ...prev, [name]: value }));
  };

  return (
    <Segment loading={loading} basic>
      <Header as='h3'>{t('setting.system.general.title')}</Header>
      <Form>
        <Form.Input
          label={t('setting.system.general.server_address')}
          placeholder={t('setting.system.general.server_address_placeholder')}
          name='ServerAddress'
          value={inputs.ServerAddress}
          onChange={handleChange}
        />
        <Form.Input
          label={t('setting.system.general.system_name')}
          name='SystemName'
          value={inputs.SystemName}
          onChange={handleChange}
        />
        <Form.Input
          label={t('setting.system.general.logo')}
          name='Logo'
          value={inputs.Logo}
          onChange={handleChange}
        />
        <Form.TextArea
          label='Footer HTML'
          name='Footer'
          value={inputs.Footer}
          onChange={handleChange}
        />
        <Form.Input
          label={t('setting.system.top_up_link', '充值链接')}
          name='TopUpLink'
          value={inputs.TopUpLink}
          onChange={handleChange}
        />
        <Form.Input
          label={t('setting.system.chat_link', '聊天链接')}
          name='ChatLink'
          value={inputs.ChatLink}
          onChange={handleChange}
        />
        <Button primary onClick={submitGeneral}>
          {t('setting.system.buttons.save')}
        </Button>
      </Form>

      <Divider />
      <Header as='h3'>{t('setting.system.smtp.title')}</Header>
      <Message>{t('setting.system.smtp.subtitle')}</Message>
      <Form>
        <Form.Group widths={3}>
          <Form.Input
            label={t('setting.system.smtp.server')}
            placeholder={t('setting.system.smtp.server_placeholder')}
            name='SMTPServer'
            onChange={handleChange}
            value={inputs.SMTPServer}
          />
          <Form.Input
            label={t('setting.system.smtp.port')}
            placeholder={t('setting.system.smtp.port_placeholder')}
            name='SMTPPort'
            onChange={handleChange}
            value={inputs.SMTPPort}
          />
          <Form.Input
            label={t('setting.system.smtp.account')}
            placeholder={t('setting.system.smtp.account_placeholder')}
            name='SMTPAccount'
            onChange={handleChange}
            value={inputs.SMTPAccount}
          />
        </Form.Group>
        <Form.Group widths={2}>
          <Form.Input
            label={t('setting.system.smtp.from')}
            placeholder={t('setting.system.smtp.from_placeholder')}
            name='SMTPFrom'
            onChange={handleChange}
            value={inputs.SMTPFrom}
          />
          <Form.Input
            label={t('setting.system.smtp.token')}
            placeholder={t('setting.system.smtp.token_placeholder')}
            name='SMTPToken'
            onChange={handleChange}
            value={inputs.SMTPToken}
          />
        </Form.Group>
        <Button primary onClick={submitSMTP}>
          {t('setting.system.buttons.save')}
        </Button>
      </Form>

      <Divider />
      <Header as='h3'>{t('setting.system.login.title')}</Header>
      <Form>
        <Form.Checkbox
          label={t('setting.system.login.password_login')}
          name='PasswordLoginEnabled'
          checked={inputs.PasswordLoginEnabled === 'true' || inputs.PasswordLoginEnabled === true}
          onChange={(_, data) => handleChange(_, { ...data, value: data.checked ? 'true' : 'false' })}
        />
        <Form.Checkbox
          label={t('setting.system.login.password_register')}
          name='PasswordRegisterEnabled'
          checked={inputs.PasswordRegisterEnabled === 'true' || inputs.PasswordRegisterEnabled === true}
          onChange={(_, data) => handleChange(_, { ...data, value: data.checked ? 'true' : 'false' })}
        />
        <Form.Checkbox
          label={t('setting.system.login.registration')}
          name='RegisterEnabled'
          checked={inputs.RegisterEnabled === 'true' || inputs.RegisterEnabled === true}
          onChange={(_, data) => handleChange(_, { ...data, value: data.checked ? 'true' : 'false' })}
        />
        <Button onClick={submitAuth}>{t('setting.system.buttons.save')}</Button>
      </Form>

      <Divider />
      <Header as='h3'>{t('setting.system.notice', '站点公告')}</Header>
      <Form>
        <Form.TextArea
          name='Notice'
          value={inputs.Notice}
          onChange={handleChange}
          placeholder={t('setting.system.notice_placeholder', '支持 Markdown')}
        />
        <Button onClick={submitNotice}>{t('setting.system.buttons.save')}</Button>
      </Form>
    </Segment>
  );
};

export default SystemSetting;
