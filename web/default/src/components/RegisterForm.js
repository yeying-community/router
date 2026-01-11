import React, { useState } from 'react';
import { Button, Form, Grid, Header, Image, Message, Card } from 'semantic-ui-react';
import { Link, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { API, getLogo, showError, showInfo, showSuccess } from '../helpers';

const RegisterForm = () => {
  const { t } = useTranslation();
  const [inputs, setInputs] = useState({
    username: '',
    password: '',
    password2: '',
    email: '',
  });
  const { username, password, password2 } = inputs;
  const [loading, setLoading] = useState(false);
  const logo = getLogo();
  let affCode = new URLSearchParams(window.location.search).get('aff');
  if (affCode) {
    localStorage.setItem('aff', affCode);
  }

  const navigate = useNavigate();

  function handleChange(e) {
    const { name, value } = e.target;
    setInputs((prev) => ({ ...prev, [name]: value }));
  }

  async function handleSubmit() {
    if (password.length < 8) {
      showInfo(t('messages.error.password_length'));
      return;
    }
    if (password !== password2) {
      showInfo(t('messages.error.password_mismatch'));
      return;
    }
    if (username && password) {
      setLoading(true);
      if (!affCode) {
        affCode = localStorage.getItem('aff');
      }
      const payload = { ...inputs, aff_code: affCode };
      const res = await API.post('/api/user/register', payload);
      const { success, message } = res.data;
      setLoading(false);
      if (success) {
        navigate('/login');
        showSuccess(t('messages.success.register'));
      } else {
        showError(message);
      }
    }
  }

  return (
    <Grid textAlign='center' style={{ marginTop: '48px' }}>
      <Grid.Column style={{ maxWidth: 450 }}>
        <Card
          fluid
          className='chart-card'
          style={{ boxShadow: '0 1px 3px rgba(0,0,0,0.12)' }}
        >
          <Card.Content>
            <Card.Header>
              <Header as='h2' textAlign='center' style={{ marginBottom: '1.5em' }}>
                <Image src={logo} style={{ marginBottom: '10px' }} />
                <Header.Content>{t('auth.register.title')}</Header.Content>
              </Header>
            </Card.Header>
            <Form size='large'>
              <Form.Input
                fluid
                icon='user'
                iconPosition='left'
                placeholder={t('auth.register.username')}
                onChange={handleChange}
                name='username'
                style={{ marginBottom: '1em' }}
              />
              <Form.Input
                fluid
                icon='lock'
                iconPosition='left'
                placeholder={t('auth.register.password')}
                onChange={handleChange}
                name='password'
                type='password'
                style={{ marginBottom: '1em' }}
              />
              <Form.Input
                fluid
                icon='lock'
                iconPosition='left'
                placeholder={t('auth.register.confirm_password')}
                onChange={handleChange}
                name='password2'
                type='password'
                style={{ marginBottom: '1.5em' }}
              />
              <Button
                fluid
                size='large'
                onClick={handleSubmit}
                style={{ background: '#2F73FF', color: 'white', marginBottom: '1.5em' }}
                loading={loading}
              >
                {t('auth.register.button')}
              </Button>
            </Form>

            <Message style={{ background: 'transparent', boxShadow: 'none' }}>
              <div style={{ textAlign: 'center', fontSize: '0.9em', color: '#666' }}>
                {t('auth.register.has_account')}
                <Link to='/login' style={{ color: '#2185d0', marginLeft: '2px' }}>
                  {t('auth.register.login')}
                </Link>
              </div>
            </Message>
          </Card.Content>
        </Card>
      </Grid.Column>
    </Grid>
  );
};

export default RegisterForm;
