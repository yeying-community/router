import React, { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { showError, showInfo, timestamp2string } from '../../helpers';
import { formatAmountWithUnit } from '../../helpers/render';
import {
  TOPUP_RESULT_COLUMN_WIDTHS,
  TOPUP_RESULT_TABLE_MIN_WIDTH,
} from '../../constants/tableWidthPresets';
import { useTopUpWorkspace } from './shared.jsx';
import { AppButton, AppInput, AppModal, AppTable } from '../../router-ui';

const RedeemCodePage = ({ open, onClose, onRedeemed }) => {
  const { t } = useTranslation();
  const { renderDisplayAmount, submitRedemption } = useTopUpWorkspace();
  const [redemptionCode, setRedemptionCode] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [recentResult, setRecentResult] = useState(null);

  useEffect(() => {
    if (!open) {
      setRedemptionCode('');
      setRecentResult(null);
      setSubmitting(false);
    }
  }, [open]);

  const recentResultRows = recentResult
    ? [
        {
          key: 'row-1',
          leftLabel: t('topup.redemption_result.fields.redeemed_amount'),
          leftValue: renderDisplayAmount(recentResult.redeemed_amount),
          rightLabel: t('topup.redemption_result.fields.redeemed_at'),
          rightValue: recentResult.redeemed_at
            ? timestamp2string(recentResult.redeemed_at)
            : '-',
        },
        {
          key: 'row-2',
          leftLabel: t('topup.redemption_result.fields.before_balance'),
          leftValue: renderDisplayAmount(recentResult.before_balance_amount),
          rightLabel: t('topup.redemption_result.fields.after_balance'),
          rightValue: renderDisplayAmount(recentResult.after_balance_amount),
        },
        {
          key: 'row-3',
          leftLabel: t('topup.redemption_result.fields.redemption_name'),
          leftValue: recentResult.redemption_name || '-',
          rightLabel: t('topup.redemption_result.fields.redemption_id'),
          rightValue: recentResult.redemption_id ? (
            <span className='router-monospace-value'>
              {recentResult.redemption_id}
            </span>
          ) : (
            '-'
          ),
        },
        {
          key: 'row-4',
          leftLabel: t('topup.redemption_result.fields.group'),
          leftValue: recentResult.group_name || recentResult.group_id || '-',
          rightLabel: t('topup.redemption_result.fields.face_value'),
          rightValue:
            recentResult.face_value_amount > 0
              ? formatAmountWithUnit(
                  recentResult.face_value_amount,
                  recentResult.face_value_unit || 'YYC',
                )
              : '-',
        },
        {
          key: 'row-5',
          leftLabel: t('topup.redemption_result.fields.credit_expires_at'),
          leftValue: recentResult.credit_expires_at
            ? timestamp2string(recentResult.credit_expires_at)
            : t('common.never'),
          rightLabel: '',
          rightValue: '',
        },
      ]
    : [];

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
      if (onRedeemed) {
        onRedeemed(result);
      }
    } catch (error) {
      showError(error?.message || t('topup.redeem.request_failed'));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <AppModal
      size='small'
      open={open}
      onClose={onClose}
      closeOnDimmerClick={!submitting}
      title={t('topup.redeem.title')}
      footer={[
        <AppButton
          key='cancel'
          className='router-section-button'
          onClick={onClose}
          disabled={submitting}
        >
          {t('common.cancel')}
        </AppButton>,
        <AppButton
          key='submit'
          className='router-section-button'
          color='blue'
          onClick={handleSubmit}
          loading={submitting}
          disabled={submitting}
        >
          {submitting ? t('topup.redeem.submitting') : t('common.confirm')}
        </AppButton>,
      ]}
    >
      <div className='router-section-stack'>
        <div className='router-text-muted'>{t('topup.redeem.description')}</div>

        <AppInput
          className='router-modal-input router-machine-input'
          fluid
          placeholder={t('topup.redeem.placeholder')}
          value={redemptionCode}
          onChange={(event) => setRedemptionCode(event.target.value)}
          onPaste={(event) => {
            event.preventDefault();
            const pastedText = event.clipboardData.getData('text');
            setRedemptionCode(pastedText.trim());
          }}
        />
      </div>

      {recentResult ? (
        <div className='router-topup-result-section'>
          <div className='router-toolbar router-toolbar-compact'>
            <div className='router-title-accent-warning'>
              {t('topup.redemption_result.title')}
            </div>
            <AppButton
              className='router-inline-button'
              basic
              size='small'
              onClick={() => setRecentResult(null)}
            >
              {t('topup.redemption_result.close')}
            </AppButton>
          </div>
          <div className='router-table-scroll-x'>
            <AppTable
              className='router-list-table router-table-fit-page'
              rowKey='key'
              pagination={false}
              scroll={{ x: TOPUP_RESULT_TABLE_MIN_WIDTH }}
              dataSource={recentResultRows}
              columns={[
                {
                  title: '',
                  dataIndex: 'leftLabel',
                  width: TOPUP_RESULT_COLUMN_WIDTHS.label,
                  render: (value) => <span className='router-text-muted'>{value}</span>,
                },
                {
                  title: '',
                  dataIndex: 'leftValue',
                  width: TOPUP_RESULT_COLUMN_WIDTHS.value,
                },
                {
                  title: '',
                  dataIndex: 'rightLabel',
                  width: TOPUP_RESULT_COLUMN_WIDTHS.label,
                  render: (value) =>
                    value ? <span className='router-text-muted'>{value}</span> : null,
                },
                {
                  title: '',
                  dataIndex: 'rightValue',
                  width: TOPUP_RESULT_COLUMN_WIDTHS.value,
                },
              ]}
            />
          </div>
        </div>
      ) : null}
    </AppModal>
  );
};

export default RedeemCodePage;
