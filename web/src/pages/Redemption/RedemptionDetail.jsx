import React, { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Card, Form, Label } from 'semantic-ui-react';
import {
  useLocation,
  useNavigate,
  useParams,
  useSearchParams,
} from 'react-router-dom';
import { API, showError, showSuccess, timestamp2string } from '../../helpers';
import {
  formatYYCValue,
  renderQuotaWithPrompt,
} from '../../helpers/render';

function renderStatus(status, t) {
  switch (status) {
    case 1:
      return (
        <Label basic color='green' className='router-tag'>
          {t('redemption.status.unused')}
        </Label>
      );
    case 2:
      return (
        <Label basic color='red' className='router-tag'>
          {t('redemption.status.disabled')}
        </Label>
      );
    case 3:
      return (
        <Label basic color='grey' className='router-tag'>
          {t('redemption.status.used')}
        </Label>
      );
    default:
      return (
        <Label basic color='black' className='router-tag'>
          {t('redemption.status.unknown')}
        </Label>
      );
  }
}

const RedemptionDetail = () => {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const { id } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [redemption, setRedemption] = useState(null);
  const [inputs, setInputs] = useState({
    name: '',
    quota: 0,
  });
  const isEditing = searchParams.get('edit') === '1';
  const returnPath = (() => {
    const from = location.state?.from;
    if (typeof from !== 'string') {
      return '';
    }
    const normalized = from.trim();
    return normalized.startsWith('/') ? normalized : '';
  })();

  const syncInputs = useCallback((data) => {
    setInputs({
      name: (data?.name || '').toString(),
      quota: Number(data?.yyc_value ?? data?.quota ?? 0) || 0,
    });
  }, []);

  const setEditMode = useCallback(
    (nextEditing) => {
      const nextSearchParams = new URLSearchParams(searchParams.toString());
      if (nextEditing) {
        nextSearchParams.set('edit', '1');
      } else {
        nextSearchParams.delete('edit');
      }
      setSearchParams(nextSearchParams, { replace: true });
    },
    [searchParams, setSearchParams]
  );

  const handleInputChange = useCallback((e, { name, value }) => {
    setInputs((prev) => ({
      ...prev,
      [name]: value,
    }));
  }, []);

  const loadRedemption = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get(`/api/v1/admin/redemption/${id}`);
      const { success, message, data } = res.data;
      if (success) {
        setRedemption(data);
        syncInputs(data);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    } finally {
      setLoading(false);
    }
  }, [id, syncInputs]);

  useEffect(() => {
    loadRedemption().then();
  }, [loadRedemption]);

  const handleCancelEdit = () => {
    syncInputs(redemption);
    setEditMode(false);
  };

  const submitEdit = async () => {
    setSaving(true);
    try {
      const res = await API.put('/api/v1/admin/redemption/', {
        id,
        name: (inputs.name || '').toString().trim(),
        quota: parseInt(inputs.quota || 0, 10) || 0,
      });
      const { success, message, data } = res.data || {};
      if (!success) {
        showError(message);
        return;
      }
      setRedemption(data);
      syncInputs(data);
      setEditMode(false);
      showSuccess(t('redemption.messages.update_success'));
    } catch (error) {
      showError(error?.message || error);
    } finally {
      setSaving(false);
    }
  };

  const redeemedByValue =
    redemption?.redeemed_by_username ||
    redemption?.redeemed_by_user_id ||
    t('redemption.table.not_redeemed');

  const handleBack = () => {
    if (returnPath !== '') {
      navigate(-1);
      return;
    }
    navigate('/redemption');
  };

  return (
    <div className='dashboard-container'>
      <Card fluid className='chart-card'>
        <Card.Content>
          <Card.Header className='header router-page-title'>
            {t('redemption.detail.title')}
          </Card.Header>
          <div className='router-toolbar router-block-gap-sm'>
            <div className='router-toolbar-start'>
              {isEditing ? (
                <>
                  <Button
                    className='router-page-button'
                    onClick={handleCancelEdit}
                    disabled={saving}
                  >
                    {t('redemption.edit.buttons.cancel')}
                  </Button>
                  <Button
                    className='router-page-button'
                    primary
                    loading={saving}
                    disabled={saving}
                    onClick={submitEdit}
                  >
                    {t('redemption.edit.buttons.submit')}
                  </Button>
                </>
              ) : (
                <>
                  <Button className='router-page-button' onClick={handleBack}>
                    {t('redemption.detail.buttons.back')}
                  </Button>
                  <Button
                    className='router-page-button'
                    primary
                    onClick={() => setEditMode(true)}
                  >
                    {t('redemption.buttons.edit')}
                  </Button>
                </>
              )}
            </div>
            <div className='router-toolbar-end'>
              <div className='router-action-group'>
                {redemption ? renderStatus(redemption.status, t) : null}
              </div>
            </div>
          </div>

          <Form loading={loading}>
            <Form.Group widths='equal'>
              {isEditing ? (
                <Form.Input
                  className='router-section-input'
                  label={t('redemption.edit.name')}
                  name='name'
                  value={inputs.name}
                  placeholder={t('redemption.edit.name_placeholder')}
                  onChange={handleInputChange}
                />
              ) : (
                <Form.Input
                  className='router-section-input'
                  label={t('redemption.table.name')}
                  value={redemption?.name || t('redemption.table.no_name')}
                  readOnly
                />
              )}
              <Form.Input
                className='router-section-input'
                label={t('redemption.detail.code')}
                value={redemption?.code || ''}
                readOnly
              />
            </Form.Group>
            <Form.Group widths='equal'>
              {isEditing ? (
                <Form.Input
                  className='router-section-input'
                  label={`${t('redemption.edit.quota')}${renderQuotaWithPrompt(inputs.quota, t)}`}
                  name='quota'
                  type='number'
                  value={inputs.quota}
                  placeholder={t('redemption.edit.quota_placeholder')}
                  onChange={handleInputChange}
                />
              ) : (
                <Form.Input
                  className='router-section-input'
                  label={t('redemption.table.quota')}
                  value={redemption ? formatYYCValue(redemption.yyc_value ?? redemption.quota) : ''}
                  readOnly
                />
              )}
              <Form.Input
                className='router-section-input'
                label={t('redemption.detail.redeemed_by')}
                value={redeemedByValue}
                readOnly
              />
            </Form.Group>
            <Form.Group widths='equal'>
              <Form.Input
                className='router-section-input'
                label={t('redemption.table.created_time')}
                value={
                  redemption?.created_time
                    ? timestamp2string(redemption.created_time)
                    : ''
                }
                readOnly
              />
              <Form.Input
                className='router-section-input'
                label={t('redemption.table.redeemed_time')}
                value={
                  redemption?.redeemed_time
                    ? timestamp2string(redemption.redeemed_time)
                    : t('redemption.table.not_redeemed')
                }
                readOnly
              />
            </Form.Group>
          </Form>
        </Card.Content>
      </Card>
    </div>
  );
};

export default RedemptionDetail;
