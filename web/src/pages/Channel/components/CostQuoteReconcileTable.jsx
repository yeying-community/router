import React from 'react';
import { AppAlert, AppButton, AppDetailSection, AppTag } from '../../../router-ui';
import { formatCreditAmount } from '../../../helpers/render';

// Read-only reconciliation view: billing service's normalized unit cost (in YYC)
// per model vs the channel's local procurement readiness. Never affects online
// charging. Pure presentational — parent owns the data shape and refresh.
const CostQuoteReconcileTable = ({ t, costQuotes, onSyncCostQuotes, syncSubmitting = false }) => {
  const rows = Array.isArray(costQuotes?.rows) ? costQuotes.rows : [];
  const available = costQuotes?.service_available === true;

  return (
    <AppDetailSection
      className='router-billing-management-section'
      title={t('channel.edit.billing.cost_reconcile_title')}
      titleTag='span'
      headerEnd={
        available ? (
          <AppButton
            type='button'
            className='router-page-button'
            color='blue'
            loading={syncSubmitting}
            disabled={syncSubmitting}
            onClick={onSyncCostQuotes}
          >
            {t('channel.edit.billing.cost_reconcile_sync')}
          </AppButton>
        ) : null
      }
    >
      <div className='router-billing-subsection-header'>
        <div>
          <div className='router-billing-subsection-description'>
            {t('channel.edit.billing.cost_reconcile_hint')}
          </div>
        </div>
      </div>
      {!available ? (
        <AppAlert
          type='info'
          showIcon
          className='router-section-message'
          title={t('channel.edit.billing.cost_reconcile_unavailable_title')}
          description={costQuotes?.reason || t('channel.edit.billing.cost_reconcile_unavailable_default')}
        />
      ) : rows.length === 0 ? (
        <AppAlert
          type='info'
          showIcon
          className='router-section-message'
          title={t('channel.edit.billing.cost_reconcile_empty_title')}
        />
      ) : (
        <table className='router-cost-reconcile-table'>
          <thead>
            <tr>
              <th>{t('channel.edit.billing.cost_reconcile_col.model')}</th>
              <th>{t('channel.edit.billing.cost_reconcile_col.capacity_unit')}</th>
              <th className='router-cost-reconcile-num'>
                {t('channel.edit.billing.cost_reconcile_col.service_unit_cost')}
              </th>
              <th className='router-cost-reconcile-num'>
                {t('channel.edit.billing.cost_reconcile_col.service_unit_cost_yyc')}
              </th>
              <th>{t('channel.edit.billing.cost_reconcile_col.service_confidence')}</th>
              <th>{t('channel.edit.billing.cost_reconcile_col.local_readiness')}</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.model}>
                <td className='router-monospace-value'>{row.model || '-'}</td>
                <td>{row.capacity_unit || '-'}</td>
                <td className='router-cost-reconcile-num'>
                  {row.service_unit_cost ? Number(row.service_unit_cost).toFixed(4) : '-'}
                  {row.service_currency ? ` ${row.service_currency}` : ''}
                </td>
                <td className='router-cost-reconcile-num'>
                  {formatCreditAmount(row.service_unit_cost_yyc || 0, true)}
                </td>
                <td>
                  <AppTag
                    color={
                      row.service_confidence === 'actual'
                        ? 'green'
                        : row.service_confidence === 'estimated'
                          ? 'orange'
                          : 'default'
                    }
                  >
                    {row.service_confidence || '-'}
                  </AppTag>
                </td>
                <td>
                  <AppTag
                    color={
                      row.local_readiness === 'ready'
                        ? 'green'
                        : row.local_readiness === 'untracked'
                          ? 'default'
                          : 'orange'
                    }
                  >
                    {row.local_readiness || '-'}
                  </AppTag>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </AppDetailSection>
  );
};

export default CostQuoteReconcileTable;
