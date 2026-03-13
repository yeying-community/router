import React, { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Card, Form, Label } from 'semantic-ui-react';
import { useNavigate, useParams } from 'react-router-dom';
import { API, showError, timestamp2string } from '../../helpers';
import { renderQuota } from '../../helpers/render';

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
  const navigate = useNavigate();
  const { id } = useParams();
  const [loading, setLoading] = useState(true);
  const [redemption, setRedemption] = useState(null);

  const loadRedemption = useCallback(async () => {
    setLoading(true);
    try {
      const res = await API.get(`/api/v1/admin/redemption/${id}`);
      const { success, message, data } = res.data;
      if (success) {
        setRedemption(data);
      } else {
        showError(message);
      }
    } catch (error) {
      showError(error.message);
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    loadRedemption().then();
  }, [loadRedemption]);

  const redeemedByValue =
    redemption?.redeemed_by_username ||
    redemption?.redeemed_by_user_id ||
    t('redemption.table.not_redeemed');

  return (
    <div className='dashboard-container'>
      <Card fluid className='chart-card'>
        <Card.Content>
          <div className='router-toolbar-start router-block-gap-md'>
            <Button className='router-page-button' onClick={() => navigate('/redemption')}>
              {t('redemption.detail.buttons.back')}
            </Button>
            <Button
              className='router-page-button'
              primary
              onClick={() => navigate(`/redemption/edit/${id}`)}
            >
              {t('redemption.buttons.edit')}
            </Button>
          </div>

          <Form loading={loading}>
            <Form.Group widths='equal'>
              <Form.Input
                className='router-section-input'
                label={t('redemption.table.name')}
                value={redemption?.name || t('redemption.table.no_name')}
                readOnly
              />
              <Form.Input
                className='router-section-input'
                label={t('redemption.detail.code')}
                value={redemption?.code || ''}
                readOnly
              />
            </Form.Group>
            <Form.Group widths='equal'>
              <Form.Field>
                <label>{t('redemption.table.status')}</label>
                <div className='router-field-display'>
                  {redemption ? renderStatus(redemption.status, t) : null}
                </div>
              </Form.Field>
              <Form.Input
                className='router-section-input'
                label={t('redemption.table.quota')}
                value={redemption ? renderQuota(redemption.quota, t) : ''}
                readOnly
              />
            </Form.Group>
            <Form.Group widths='equal'>
              <Form.Input
                className='router-section-input'
                label={t('redemption.table.created_time')}
                value={redemption?.created_time ? timestamp2string(redemption.created_time) : ''}
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
            <Form.Group widths='equal'>
              <Form.Input
                className='router-section-input'
                label={t('redemption.detail.redeemed_by')}
                value={redeemedByValue}
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
