import React, { useMemo, useState } from 'react';
import {
  AppAlert,
  AppButton,
  AppDetailSection,
  AppFilterHeader,
  AppInput,
  AppSelect,
  AppSwitch,
  AppTable,
  AppTag,
  AppTooltip,
} from '../../../router-ui';

const resolveEndpointTestStatusKey = (row) =>
  (row?.last_test_status || '').toString().trim() || 'untested';

const ChannelDetailEndpointsTab = ({
  t,
  columnWidths,
  endpointSummaryText,
  channelEndpoints,
  channelEndpointsLoading,
  channelEndpointsError,
  buildChannelEndpointKey,
  endpointCapabilityReadonly,
  endpointMutatingKey,
  updateChannelEndpointCapability,
  channelEndpointPoliciesLoading,
  channelEndpointPolicies,
  channelEndpointPoliciesError,
  endpointPolicyReadonly,
  openEndpointPolicyEditor,
  timestamp2string,
}) => {
  const policyByKey = new Map(
    channelEndpointPolicies.map((row) => [
      buildChannelEndpointKey(row.model, row.endpoint),
      row,
    ]),
  );
  const [testStatusFilter, setTestStatusFilter] = useState('all');
  const [baseURLDrafts, setBaseURLDrafts] = useState({});

  const testStatusOptions = useMemo(
    () => [
      {
        key: 'all',
        value: 'all',
        text: t('channel.edit.endpoint_capabilities.filters.all_test_status'),
      },
      ...[
        'success',
        'failed',
        'untested',
      ].map((status) => ({
        key: status,
        value: status,
        text: t(`channel.edit.model_tester.status.${status}`),
      })),
    ],
    [t],
  );

  const filteredRows = useMemo(
    () =>
      channelEndpoints.filter((row) => {
        if (testStatusFilter === 'all') {
          return true;
        }
        return resolveEndpointTestStatusKey(row) === testStatusFilter;
      }),
    [channelEndpoints, testStatusFilter],
  );

  const resolveBaseURLDraft = (row, endpointKey) => {
    if (Object.prototype.hasOwnProperty.call(baseURLDrafts, endpointKey)) {
      return baseURLDrafts[endpointKey];
    }
    return row.base_url || '';
  };

  return (
    <AppDetailSection
      title={t('channel.edit.endpoint_capabilities.title')}
      titleTag='span'
      headerStart={<span className='router-toolbar-meta'>({endpointSummaryText})</span>}
    >
      <div>
        <AppAlert
          type='info'
          showIcon
          className='router-section-message'
          title={t('channel.edit.endpoint_capabilities.hint')}
        />
        <AppFilterHeader
          className='router-toolbar-compact'
          startClassName='router-block-gap-sm'
          picker={
            <AppSelect
              className='router-section-dropdown router-detail-filter-dropdown router-dropdown-min-170'
              options={testStatusOptions}
              value={testStatusFilter}
              disabled={channelEndpointsLoading || channelEndpoints.length === 0}
              placeholder={t('channel.edit.endpoint_capabilities.filters.test_status')}
              onChange={(e, { value }) =>
                setTestStatusFilter((value || 'all').toString())
              }
            />
          }
        />
        <AppTable
          className='router-detail-table router-channel-endpoint-capability-table'
          pagination={false}
          scroll={{ x: 980 }}
          locale={{
            emptyText: channelEndpointsLoading
              ? t('channel.edit.endpoint_capabilities.loading')
              : channelEndpoints.length === 0
                ? t('channel.edit.endpoint_capabilities.empty')
                : t('channel.edit.endpoint_capabilities.filtered_empty'),
          }}
          rowKey={(row) => buildChannelEndpointKey(row.model, row.endpoint)}
          dataSource={filteredRows}
          columns={[
            {
              title: t('channel.edit.endpoint_capabilities.table.model'),
              dataIndex: 'model',
              key: 'model',
              width: columnWidths[0],
              render: (value) => (
                <span className='router-cell-truncate' title={value}>
                  {value}
                </span>
              ),
            },
            {
              title: t('channel.edit.endpoint_capabilities.table.endpoint'),
              dataIndex: 'endpoint',
              key: 'endpoint',
              width: columnWidths[1],
              render: (value) => (
                <span className='router-cell-truncate' title={value}>
                  {value}
                </span>
              ),
            },
            {
              title: t('channel.edit.endpoint_capabilities.table.base_url'),
              key: 'base_url',
              width: columnWidths[2],
              render: (_, row) => {
                const endpointKey = buildChannelEndpointKey(row.model, row.endpoint);
                const isMutating = endpointMutatingKey === endpointKey;
                const draftBaseURL = resolveBaseURLDraft(row, endpointKey);
                return (
                  <AppInput
                    className='router-section-input'
                    placeholder={t(
                      'channel.edit.endpoint_capabilities.table.base_url_placeholder',
                    )}
                    value={draftBaseURL}
                    readOnly={endpointCapabilityReadonly || isMutating}
                    onChange={(e, { value }) => {
                      setBaseURLDrafts((prev) => ({
                        ...prev,
                        [endpointKey]: (value || '').toString(),
                      }));
                    }}
                    onBlur={() => {
                      const normalizedCurrent = (row.base_url || '').toString().trim();
                      const normalizedNext = (draftBaseURL || '').toString().trim();
                      if (normalizedCurrent === normalizedNext) {
                        return;
                      }
                      updateChannelEndpointCapability(
                        {
                          ...row,
                          base_url: normalizedNext,
                        },
                        { base_url: normalizedNext, enabled: row.enabled === true },
                        { skipConfirm: true },
                      );
                    }}
                  />
                );
              },
            },
            {
              title: t('channel.edit.endpoint_capabilities.table.enabled'),
              key: 'enabled',
              width: columnWidths[3],
              align: 'center',
              render: (_, row) => {
                const endpointKey = buildChannelEndpointKey(row.model, row.endpoint);
                const isMutating = endpointMutatingKey === endpointKey;
                const blockedReason = (row.enable_block_reason || '').trim();
                const disabled =
                  endpointCapabilityReadonly ||
                  isMutating ||
                  (!!blockedReason && row.enabled !== true);
                return (
                  <AppSwitch
                    checked={row.enabled === true}
                    disabled={disabled}
                    title={blockedReason || undefined}
                    onChange={(_, { checked }) =>
                      updateChannelEndpointCapability(row, {
                        enabled: checked === true,
                      })
                    }
                  />
                );
              },
            },
            {
              title: t('channel.edit.endpoint_capabilities.table.test_status'),
              key: 'test_status',
              width: columnWidths[4],
              render: (_, row) => {
                const latestStatusKey = resolveEndpointTestStatusKey(row);
                const lastTestError = (row.last_test_error || '').trim();
                const statusTag = (
                  <AppTag
                    color={
                      latestStatusKey === 'success'
                        ? 'green'
                        : latestStatusKey === 'failed'
                          ? 'red'
                          : 'grey'
                    }
                  >
                    {t(`channel.edit.model_tester.status.${latestStatusKey}`)}
                  </AppTag>
                );
                if (!lastTestError) {
                  return statusTag;
                }
                return (
                  <AppTooltip
                    title={lastTestError}
                  >
                    <span>
                      {statusTag}
                    </span>
                  </AppTooltip>
                );
              },
            },
            {
              title: t('channel.edit.endpoint_policies.table.policy'),
              key: 'policy',
              width: columnWidths[5],
              render: (_, row) => {
                const endpointKey = buildChannelEndpointKey(row.model, row.endpoint);
                const policyRow = policyByKey.get(endpointKey) || null;
                if (
                  channelEndpointPoliciesLoading &&
                  channelEndpointPolicies.length === 0
                ) {
                  return (
                    <span className='router-cell-truncate'>
                      {t('channel.edit.endpoint_policies.loading')}
                    </span>
                  );
                }
                return (
                  <span className='router-cell-truncate'>
                    {policyRow?.template_key || '-'}
                  </span>
                );
              },
            },
            {
              title: t('channel.edit.endpoint_policies.table.actions'),
              key: 'actions',
              width: columnWidths[6],
              render: (_, row) => {
                const endpointKey = buildChannelEndpointKey(row.model, row.endpoint);
                const policyRow = policyByKey.get(endpointKey) || null;
                return (
                  <AppButton
                    type='button'
                    className='router-inline-button'
                    disabled={endpointPolicyReadonly}
                    onClick={() => openEndpointPolicyEditor(row)}
                    title={
                      policyRow?.updated_at > 0
                        ? timestamp2string(policyRow.updated_at)
                        : row.updated_at > 0
                          ? timestamp2string(row.updated_at)
                          : undefined
                    }
                  >
                    {t('channel.edit.endpoint_policies.action')}
                  </AppButton>
                );
              },
            },
          ]}
        />
        {channelEndpointsError && (
          <div className='router-error-text router-error-text-top'>
            {channelEndpointsError}
          </div>
        )}
        {channelEndpointPoliciesError && (
          <div className='router-error-text router-error-text-top'>
            {channelEndpointPoliciesError}
          </div>
        )}
      </div>
    </AppDetailSection>
  );
};

export default ChannelDetailEndpointsTab;
