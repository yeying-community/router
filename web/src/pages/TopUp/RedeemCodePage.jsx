import React, { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Card, Form, Header, Table } from 'semantic-ui-react';
import { showError, showInfo, timestamp2string } from '../../helpers';
import { formatAmountWithUnit } from '../../helpers/render';
import { useTopUpWorkspace } from './shared.jsx';

const RedeemCodePage = () => {
  const { t } = useTranslation();
  const { renderDisplayAmount, submitRedemption } = useTopUpWorkspace();
  const [redemptionCode, setRedemptionCode] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [recentResult, setRecentResult] = useState(null);

  const handleSubmit = async () => {
    if ((redemptionCode || '').trim() === '') {
      showInfo(t('topup.redeem.empty_code'));
      return;
    }
    setSubmitting(true);
    try {
      const result = await submitRedemption(redemptionCode.trim());
      if (!result) {
        return;
      }
      setRecentResult(result);
      setRedemptionCode('');
    } catch (error) {
      showError(error?.message || t('topup.redeem.request_failed'));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <Card fluid className='router-soft-card router-soft-card-fill'>
        <Card.Content className='router-card-fill'>
          <Card.Header className='router-card-header'>
            <Header as='h3' className='router-section-title router-title-accent-positive'>
              <i className='ticket alternate icon' />
              {t('topup.redeem.title')}
            </Header>
          </Card.Header>
          <Card.Description className='router-card-fill'>
            <div className='router-card-body-spread'>
              <div className='router-text-muted'>{t('topup.redeem.description')}</div>

              <Form.Input
                className='router-section-input'
                fluid
                icon='key'
                iconPosition='left'
                placeholder={t('topup.redeem.placeholder')}
                value={redemptionCode}
                onChange={(event) => setRedemptionCode(event.target.value)}
                onPaste={(event) => {
                  event.preventDefault();
                  const pastedText = event.clipboardData.getData('text');
                  setRedemptionCode(pastedText.trim());
                }}
                action={
                  <Button
                    className='router-section-button'
                    onClick={async () => {
                      try {
                        const text = await navigator.clipboard.readText();
                        setRedemptionCode(text.trim());
                      } catch (error) {
                        showError(t('topup.redeem.paste_error'));
                      }
                    }}
                  >
                    {t('topup.redeem.paste')}
                  </Button>
                }
              />

              <div className='router-action-footer'>
                <Button
                  className='router-section-button'
                  color='green'
                  fluid
                  onClick={handleSubmit}
                  loading={submitting}
                  disabled={submitting}
                >
                  {submitting ? t('topup.redeem.submitting') : t('topup.redeem.submit')}
                </Button>
              </div>
            </div>
          </Card.Description>
        </Card.Content>
      </Card>

      {recentResult ? (
        <Card fluid className='router-soft-card' style={{ marginTop: '1rem' }}>
          <Card.Content>
            <Card.Header className='router-card-header'>
              <div className='router-toolbar'>
                <Header as='h3' className='router-section-title router-title-accent-warning'>
                  <i className='check circle icon' />
                  {t('topup.redemption_result.title')}
                </Header>
                <Button
                  className='router-section-button'
                  basic
                  size='small'
                  onClick={() => setRecentResult(null)}
                >
                  {t('topup.redemption_result.close')}
                </Button>
              </div>
            </Card.Header>
            <Table basic='very' compact='very' className='router-list-table'>
              <Table.Body>
                <Table.Row>
                  <Table.Cell width={4}>{t('topup.redemption_result.fields.redeemed_amount')}</Table.Cell>
                  <Table.Cell>{renderDisplayAmount(recentResult.redeemed_yyc)}</Table.Cell>
                  <Table.Cell width={4}>{t('topup.redemption_result.fields.redeemed_at')}</Table.Cell>
                  <Table.Cell>
                    {recentResult.redeemed_at ? timestamp2string(recentResult.redeemed_at) : '-'}
                  </Table.Cell>
                </Table.Row>
                <Table.Row>
                  <Table.Cell>{t('topup.redemption_result.fields.before_balance')}</Table.Cell>
                  <Table.Cell>{renderDisplayAmount(recentResult.before_yyc_balance)}</Table.Cell>
                  <Table.Cell>{t('topup.redemption_result.fields.after_balance')}</Table.Cell>
                  <Table.Cell>{renderDisplayAmount(recentResult.after_yyc_balance)}</Table.Cell>
                </Table.Row>
                <Table.Row>
                  <Table.Cell>{t('topup.redemption_result.fields.redemption_name')}</Table.Cell>
                  <Table.Cell>{recentResult.redemption_name || '-'}</Table.Cell>
                  <Table.Cell>{t('topup.redemption_result.fields.redemption_id')}</Table.Cell>
                  <Table.Cell>{recentResult.redemption_id || '-'}</Table.Cell>
                </Table.Row>
                <Table.Row>
                  <Table.Cell>{t('topup.redemption_result.fields.group')}</Table.Cell>
                  <Table.Cell>{recentResult.group_name || recentResult.group_id || '-'}</Table.Cell>
                  <Table.Cell>{t('topup.redemption_result.fields.face_value')}</Table.Cell>
                  <Table.Cell>
                    {recentResult.face_value_amount > 0
                      ? formatAmountWithUnit(
                          recentResult.face_value_amount,
                          recentResult.face_value_unit || 'YYC',
                        )
                      : '-'}
                  </Table.Cell>
                </Table.Row>
              </Table.Body>
            </Table>
          </Card.Content>
        </Card>
      ) : null}
    </>
  );
};

export default RedeemCodePage;
