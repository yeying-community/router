import React, { useState } from 'react';
import { Button, Form, Grid, Header, Image, Message, Card } from 'semantic-ui-react';
import { Link, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { API, getLogo, showError, showInfo, showSuccess } from '../helpers';

const PasswordResetForm = () => {
  const { t } = useTranslation();
  const [email, setEmail] = useState('');
  const [loading, setLoading] = useState(false);
  const logo = getLogo();
  const navigate = useNavigate();

  const sendResetEmail = async () => {
    if (email === '') {
      showInfo(t('messages.error.empty_email', '请输入邮箱地址')); 
      return;
    }
    setLoading(true);
    const res = await API.get(`/api/reset_password?email=${email}`);
    const { success, message } = res.data;
    setLoading(false);
    if (success) {
      showSuccess(t('messages.success.password_reset'));
      navigate('/login');
    } else {
      showError(message);
    }
  };

  return (
    <Grid textAlign='center' style={{ marginTop: '48px' }}>
      <Grid.Column style={{ maxWidth: 450 }}>
        <Card fluid className='chart-card' style={{ boxShadow: '0 1px 3px rgba(0,0,0,0.12)' }}>
          <Card.Content>
            <Card.Header>
              <Header as='h2' textAlign='center' style={{ marginBottom: '1.5em' }}>
                <Image src={logo} style={{ marginBottom: '10px' }} />
                <Header.Content>{t('auth.reset.title')}</Header.Content>
              </Header>
            </Card.Header>
            <Form size='large'>
              <Form.Input
                fluid
                icon='mail'
                iconPosition='left'
                placeholder={t('auth.reset.email')}
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                style={{ marginBottom: '1em' }}
              />
              <Button
                fluid
                size='large'
                onClick={sendResetEmail}
                loading={loading}
                style={{ background: '#2F73FF', color: 'white', marginBottom: '1.5em' }}
              >
                {t('auth.reset.button')}
              </Button>
            </Form>

            <Message style={{ background: 'transparent', boxShadow: 'none' }}>
              <div style={{ textAlign: 'center', fontSize: '0.9em', color: '#666' }}>
                {t('auth.reset.remember_password')}
                <Link to='/login' style={{ color: '#2185d0', marginLeft: '2px' }}>
                  {t('auth.login.login')}
                </Link>
              </div>
            </Message>
          </Card.Content>
        </Card>
      </Grid.Column>
    </Grid>
  );
};

export default PasswordResetForm;
