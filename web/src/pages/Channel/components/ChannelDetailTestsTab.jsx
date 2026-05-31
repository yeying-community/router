import React, { useMemo, useState } from 'react';
import {
  AppAlert,
  AppButton,
  AppDetailSection,
  AppFilterHeader,
  AppInput,
  AppPopover,
  resolvePopupContainer,
  AppSelect,
  AppSwitch,
  AppTable,
  AppTag,
} from '../../../router-ui';

const ChannelDetailTestsTab = ({
  t,
  channelId,
  inputs,
  columnWidths,
  modelTestResults,
  modelTestRows,
  modelTestTargetModels,
  detailModelMutating,
  toggleModelTestTarget,
  getEffectiveModelEndpoint,
  modelTestResultsByKey,
  buildModelTestResultKey,
  activeChannelTasksByModel,
  getEndpointOptionsForModel,
  updateModelTestEndpoint,
  modelTesting,
  modelTestingScope,
  modelTestingTargetSet,
  handleRunModelTests,
  detailTestingReadonly,
  modelTestError,
  openChannelTaskView,
  selectedModelTestHasActiveTasks,
  timestamp2string,
  updateAllModelTestEndpoints,
  updateAllModelTestStreams,
  resolvePreferredProviderForModel,
  normalizeChannelModelType,
  audioTestLanguage,
  setAudioTestLanguage,
  imageEditTestURL,
  setImageEditTestURL,
  imageEditTestFileName,
  imageEditTestData,
  setImageEditTestData,
  setImageEditTestFileName,
  handleImageEditTestFileChange,
}) => {
  const [providerFilter, setProviderFilter] = useState('');
  const [typeFilter, setTypeFilter] = useState('');
  const [batchSelectionMode, setBatchSelectionMode] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);

  if (inputs.protocol === 'proxy') {
    return null;
  }

  const rowsWithMeta = useMemo(
    () =>
      modelTestRows.map((row) => ({
        ...row,
        providerKey: resolvePreferredProviderForModel(row) || '',
        typeKey: normalizeChannelModelType(row.type),
      })),
    [modelTestRows, normalizeChannelModelType, resolvePreferredProviderForModel],
  );

  const providerOptions = useMemo(() => {
    const values = Array.from(
      new Set(rowsWithMeta.map((row) => row.providerKey).filter(Boolean)),
    ).sort((a, b) => a.localeCompare(b));
    return values.map((value) => ({
      key: value,
      value,
      text: value,
    }));
  }, [rowsWithMeta]);

  const typeOptions = useMemo(() => {
    const values = Array.from(
      new Set(rowsWithMeta.map((row) => row.typeKey).filter(Boolean)),
    ).sort((a, b) => a.localeCompare(b));
    return values.map((value) => ({
      key: value,
      value,
      text: t(`channel.model_types.${value}`),
    }));
  }, [rowsWithMeta, t]);

  const audioLanguageOptions = useMemo(
    () => [
      {
        key: 'zh-CN',
        value: 'zh-CN',
        text: t('channel.edit.model_tester.audio_language_options.zh-CN'),
      },
      {
        key: 'en-US',
        value: 'en-US',
        text: t('channel.edit.model_tester.audio_language_options.en-US'),
      },
    ],
    [t],
  );

  const providerStorageKey = useMemo(
    () =>
      `channel-test-filter-provider:${(channelId || '').toString().trim() || 'create'}`,
    [channelId],
  );
  const typeStorageKey = useMemo(
    () =>
      `channel-test-filter-type:${(channelId || '').toString().trim() || 'create'}`,
    [channelId],
  );

  React.useEffect(() => {
    if (providerOptions.length === 0) {
      if (providerFilter !== '') {
        setProviderFilter('');
      }
      return;
    }
    const validValues = new Set(providerOptions.map((item) => item.value));
    if (providerFilter !== '' && validValues.has(providerFilter)) {
      return;
    }
    const storedValue = window.localStorage.getItem(providerStorageKey) || '';
    const nextValue = validValues.has(storedValue)
      ? storedValue
      : providerOptions[0]?.value || '';
    if (nextValue !== providerFilter) {
      setProviderFilter(nextValue);
    }
  }, [providerFilter, providerOptions, providerStorageKey]);

  React.useEffect(() => {
    if (typeOptions.length === 0) {
      if (typeFilter !== '') {
        setTypeFilter('');
      }
      return;
    }
    const validValues = new Set(typeOptions.map((item) => item.value));
    if (typeFilter !== '' && validValues.has(typeFilter)) {
      return;
    }
    const storedValue = window.localStorage.getItem(typeStorageKey) || '';
    const nextValue = validValues.has(storedValue)
      ? storedValue
      : typeOptions[0]?.value || '';
    if (nextValue !== typeFilter) {
      setTypeFilter(nextValue);
    }
  }, [typeFilter, typeOptions, typeStorageKey]);

  const filteredRows = useMemo(
    () =>
      rowsWithMeta.filter((row) => {
        if (providerFilter !== '' && row.providerKey !== providerFilter) {
          return false;
        }
        if (typeFilter !== '' && row.typeKey !== typeFilter) {
          return false;
        }
        return true;
      }),
    [providerFilter, rowsWithMeta, typeFilter],
  );

  const filteredModelIDs = useMemo(
    () => filteredRows.map((row) => row.model),
    [filteredRows],
  );
  const filteredTargetSet = useMemo(
    () => new Set(modelTestTargetModels),
    [modelTestTargetModels],
  );
  const filteredAllSelected =
    filteredRows.length > 0 &&
    filteredRows.every((row) => filteredTargetSet.has(row.model));
  const filteredPartiallySelected =
    !filteredAllSelected &&
    filteredRows.some((row) => filteredTargetSet.has(row.model));
  const filteredSelectedCount = filteredRows.filter((row) =>
    filteredTargetSet.has(row.model),
  ).length;

  const batchEndpointOptions = useMemo(() => {
    const map = new Map();
    filteredRows.forEach((row) => {
      getEndpointOptionsForModel(row).forEach((option) => {
        if (!map.has(option.value)) {
          map.set(option.value, option);
        }
      });
    });
    return Array.from(map.values());
  }, [filteredRows, getEndpointOptionsForModel]);

  const batchEndpointValue = useMemo(() => {
    const endpointSet = new Set(
      filteredRows.map((row) => getEffectiveModelEndpoint(row)).filter(Boolean),
    );
    return endpointSet.size === 1 ? Array.from(endpointSet)[0] || '' : '';
  }, [filteredRows, getEffectiveModelEndpoint]);

  const disabledBase = detailTestingReadonly || detailModelMutating;
  const streamCapableRows = useMemo(
    () => filteredRows.filter((row) => row?.type === 'text'),
    [filteredRows],
  );
  const streamCapableModelIDs = useMemo(
    () => streamCapableRows.map((row) => row.model),
    [streamCapableRows],
  );
  const batchStreamValue = useMemo(() => {
    if (streamCapableRows.length === 0) {
      return true;
    }
    return streamCapableRows.every((row) => row?.is_stream !== false);
  }, [streamCapableRows]);

  const resultSummaryByKey = useMemo(() => {
    const summaryMap = new Map();
    (Array.isArray(modelTestResults) ? modelTestResults : []).forEach((item) => {
      const modelName = (item?.model || '').toString().trim();
      const endpoint = (item?.endpoint || '').toString().trim();
      if (modelName === '' || endpoint === '') {
        return;
      }
      const key = buildModelTestResultKey(modelName, endpoint);
      const current = summaryMap.get(key) || {
        successCount: 0,
        failureCount: 0,
        minLatencyMs: 0,
        maxLatencyMs: 0,
        totalLatencyMs: 0,
        latencyCount: 0,
      };
      if (item?.status === 'supported' && item?.supported === true) {
        current.successCount += 1;
      } else if (
        ['unsupported', 'skipped'].includes(
          (item?.status || '').toString().trim().toLowerCase(),
        )
      ) {
        current.failureCount += 1;
      }
      const latencyMs = Number(item?.latency_ms || 0);
      if (latencyMs > 0) {
        current.minLatencyMs =
          current.minLatencyMs > 0
            ? Math.min(current.minLatencyMs, latencyMs)
            : latencyMs;
        current.maxLatencyMs = Math.max(current.maxLatencyMs, latencyMs);
        current.totalLatencyMs += latencyMs;
        current.latencyCount += 1;
      }
      summaryMap.set(key, current);
    });
    return summaryMap;
  }, [buildModelTestResultKey, modelTestResults]);

  const toggleFilteredTargets = (checked) => {
    const targetSet = new Set(filteredTargetSet);
    filteredModelIDs.forEach((model) => {
      if (checked) {
        targetSet.add(model);
      } else {
        targetSet.delete(model);
      }
    });
    const nextSelected = Array.from(targetSet);
    modelTestRows.forEach((row) => {
      const shouldSelect = nextSelected.includes(row.model);
      const isSelected = filteredTargetSet.has(row.model);
      if (shouldSelect !== isSelected) {
        toggleModelTestTarget(row.model, shouldSelect);
      }
    });
  };

  const displayedColumnWidths = useMemo(
    () => (batchSelectionMode ? columnWidths : columnWidths.slice(1)),
    [batchSelectionMode, columnWidths],
  );
  const tableRowSelection = batchSelectionMode
    ? {
        columnWidth: displayedColumnWidths[0],
        selectedRowKeys: modelTestTargetModels,
        getTitleCheckboxProps: () => ({
          checked: filteredAllSelected,
          indeterminate: filteredPartiallySelected,
          disabled: disabledBase || filteredRows.length === 0,
        }),
        getCheckboxProps: () => ({
          disabled: disabledBase,
        }),
        onSelect: (record, selected) => {
          toggleModelTestTarget(record.model, selected);
        },
        onSelectAll: (selected) => {
          toggleFilteredTargets(selected);
        },
      }
    : undefined;

  return (
    <AppDetailSection title={t('channel.edit.model_tester.title')} titleTag='span'>
      <div>
        <AppAlert
          type='info'
          showIcon
          className='router-section-message'
          title={t('channel.edit.model_tester.hint')}
        />
        <AppFilterHeader
          className='router-toolbar-compact'
          picker={
            <>
            <AppSelect
              className='router-section-dropdown router-detail-filter-dropdown router-dropdown-min-170'
              options={providerOptions}
              value={providerFilter || undefined}
              disabled={disabledBase || providerOptions.length === 0}
              placeholder={t('channel.edit.model_tester.filters.provider')}
              onChange={(e, { value }) => {
                const nextValue = (value || '').toString();
                setProviderFilter(nextValue);
                if (nextValue !== '') {
                  window.localStorage.setItem(providerStorageKey, nextValue);
                }
              }}
            />
            <AppSelect
              className='router-section-dropdown router-detail-filter-dropdown router-dropdown-min-170'
              options={typeOptions}
              value={typeFilter || undefined}
              disabled={disabledBase || typeOptions.length === 0}
              placeholder={t('channel.edit.model_tester.filters.type')}
              onChange={(e, { value }) => {
                const nextValue = (value || '').toString();
                setTypeFilter(nextValue);
                if (nextValue !== '') {
                  window.localStorage.setItem(typeStorageKey, nextValue);
                }
              }}
            />
            <AppSelect
              clearable
              className='router-section-dropdown router-detail-filter-dropdown router-dropdown-min-170'
              options={batchEndpointOptions}
              value={batchEndpointValue || undefined}
              disabled={disabledBase || batchEndpointOptions.length === 0}
              placeholder={t('channel.edit.model_tester.table.batch_set')}
              onChange={(e, { value }) => {
                if ((value || '').toString().trim() === '') {
                  return;
                }
                updateAllModelTestEndpoints(value, filteredModelIDs);
              }}
            />
            <AppPopover
              trigger='click'
              open={settingsOpen}
              onOpenChange={setSettingsOpen}
              placement='bottomLeft'
              content={
                <div className='router-log-filter-editor'>
                  <div className='router-log-filter-editor-title'>
                    {t('channel.edit.model_tester.settings_title')}
                  </div>
                  {streamCapableRows.length > 0 ? (
                    <div className='router-checkbox-block router-inline-field'>
                      <AppSwitch
                        checked={batchStreamValue}
                        disabled={disabledBase}
                        onChange={(_, { checked }) =>
                          updateAllModelTestStreams(
                            checked === true,
                            streamCapableModelIDs,
                          )
                        }
                      />
                      <span>
                        {t('channel.edit.model_tester.settings_stream')}
                      </span>
                    </div>
                  ) : null}
                  <div className='router-block-gap-xs'>
                    <label>
                      {t('channel.edit.model_tester.settings_audio_language')}
                    </label>
                    <AppSelect
                      className='router-section-dropdown router-dropdown-min-170'
                      getPopupContainer={resolvePopupContainer}
                      options={audioLanguageOptions}
                      value={audioTestLanguage || 'zh-CN'}
                      onChange={(e, { value }) =>
                        setAudioTestLanguage((value || 'zh-CN').toString())
                      }
                    />
                  </div>
                  <div className='router-block-gap-xs'>
                    <label>
                      {t('channel.edit.model_tester.image_edit_source_url')}
                    </label>
                    <AppInput
                      fluid
                      value={imageEditTestURL || ''}
                      placeholder={t(
                        'channel.edit.model_tester.image_edit_source_url',
                      )}
                      onChange={(e, { value }) =>
                        setImageEditTestURL((value || '').toString())
                      }
                    />
                  </div>
                  <div className='router-block-gap-xs'>
                    <label>
                      {t('channel.edit.model_tester.image_edit_upload')}
                    </label>
                    <input
                      type='file'
                      accept='image/*'
                      onChange={handleImageEditTestFileChange}
                    />
                    {imageEditTestData ? (
                      <div className='router-muted-text'>
                        {imageEditTestFileName ||
                          t('channel.edit.model_tester.image_edit_uploaded')}
                        <AppButton
                          type='button'
                          size='small'
                          className='router-inline-button'
                          onClick={() => {
                            setImageEditTestData('');
                            setImageEditTestFileName('');
                          }}
                        >
                          {t('common.clear')}
                        </AppButton>
                      </div>
                    ) : null}
                  </div>
                </div>
              }
            >
              <AppButton
                type='button'
                className='router-page-button'
                disabled={disabledBase || filteredRows.length === 0}
              >
                {t('channel.edit.model_tester.settings_button')}
              </AppButton>
            </AppPopover>
            </>
          }
          actions={
            <>
            <AppButton
              type='button'
              color='blue'
              className='router-section-button'
              loading={modelTesting && modelTestingScope === 'batch'}
              disabled={
                disabledBase ||
                modelTesting ||
                (batchSelectionMode &&
                  (filteredSelectedCount === 0 || selectedModelTestHasActiveTasks))
              }
              onClick={() => {
                if (!batchSelectionMode) {
                  setBatchSelectionMode(true);
                  return;
                }
                handleRunModelTests({
                  targetModels: modelTestTargetModels,
                  scope: 'batch',
                });
              }}
            >
              {t(
                batchSelectionMode
                  ? 'channel.edit.model_tester.button_run_batch'
                  : 'channel.edit.model_tester.button_enter_batch',
              )}
            </AppButton>
            {batchSelectionMode && (
              <AppButton
                type='button'
                className='router-page-button'
                disabled={disabledBase || modelTesting}
                onClick={() => setBatchSelectionMode(false)}
              >
                {t('common.cancel')}
              </AppButton>
            )}
            <AppButton
              type='button'
              className='router-page-button'
              onClick={() =>
                openChannelTaskView({
                  type: 'channel_model_test',
                })
              }
            >
              {t('channel.edit.model_tester.history_tasks')}
            </AppButton>
            </>
          }
        />
        {modelTestError && (
          <div className='router-error-text router-block-gap-sm'>
            {modelTestError}
          </div>
        )}
        <AppTable
          className='router-detail-table router-model-test-table'
          pagination={false}
          scroll={{ x: 1080 }}
          rowSelection={tableRowSelection}
          locale={{
            emptyText: t(
              modelTestRows.length === 0
                ? 'channel.edit.model_tester.empty'
                : 'channel.edit.model_selector.empty_filtered',
            ),
          }}
          rowKey={(row) => row.model}
          dataSource={filteredRows}
          columns={[
            {
              title: t('channel.edit.model_tester.table.model'),
              dataIndex: 'model',
              key: 'model',
              width: displayedColumnWidths[batchSelectionMode ? 1 : 0],
              render: (value) => (
                <span
                  className='router-cell-truncate router-monospace-value'
                  title={value || '-'}
                >
                  {value || '-'}
                </span>
              ),
            },
            {
              title: t('channel.edit.model_tester.table.endpoint'),
              key: 'endpoint',
              width: displayedColumnWidths[batchSelectionMode ? 2 : 1],
              render: (_, row) => {
                const normalizedEndpoint = getEffectiveModelEndpoint(row);
                if (
                  row.type === 'text' ||
                  row.type === 'image' ||
                  row.type === 'audio'
                ) {
                  return (
                    <AppSelect
                      className='router-mini-dropdown router-table-dropdown-fluid'
                      options={getEndpointOptionsForModel(row)}
                      disabled={disabledBase}
                      value={normalizedEndpoint}
                      onChange={(e, { value }) =>
                        updateModelTestEndpoint(row.model, value)
                      }
                    />
                  );
                }
                return normalizedEndpoint || row.endpoint || '-';
              },
            },
            {
              title: t('channel.edit.model_tester.table.status'),
              key: 'status',
              width: displayedColumnWidths[batchSelectionMode ? 3 : 2],
              render: (_, row) => {
                const normalizedEndpoint = getEffectiveModelEndpoint(row);
                const item = modelTestResultsByKey.get(
                  buildModelTestResultKey(row.model, normalizedEndpoint),
                );
                const activeTask = activeChannelTasksByModel.get(row.model) || null;
                const endpointSummary =
                  resultSummaryByKey.get(
                    buildModelTestResultKey(row.model, normalizedEndpoint),
                  ) || null;
                const effectiveStatus =
                  activeTask?.status || item?.status || 'untested';
                const tagColor =
                  effectiveStatus === 'running'
                    ? 'blue'
                    : effectiveStatus === 'pending'
                      ? 'orange'
                      : effectiveStatus === 'supported'
                        ? 'green'
                        : effectiveStatus === 'skipped'
                          ? 'grey'
                          : effectiveStatus === 'untested'
                            ? undefined
                            : 'red';
                if (activeTask) {
                  return (
                    <AppTag color={tagColor} className='router-tag'>
                      {t(`channel.edit.model_tester.status.${effectiveStatus}`)}
                    </AppTag>
                  );
                }
                return (
                  <span className='router-nowrap'>
                    {(endpointSummary?.successCount || 0)}/
                    {(endpointSummary?.failureCount || 0)}
                  </span>
                );
              },
            },
            {
              title: t('channel.edit.model_tester.table.latency'),
              key: 'latency',
              width: displayedColumnWidths[batchSelectionMode ? 4 : 3],
              render: (_, row) => {
                const normalizedEndpoint = getEffectiveModelEndpoint(row);
                const endpointSummary =
                  resultSummaryByKey.get(
                    buildModelTestResultKey(row.model, normalizedEndpoint),
                  ) || null;
                return (
                  <span className='router-nowrap'>
                    {endpointSummary?.latencyCount > 0
                      ? `${endpointSummary.minLatencyMs}/` +
                        `${Math.round(
                          endpointSummary.totalLatencyMs /
                            endpointSummary.latencyCount,
                        )}/` +
                        `${endpointSummary.maxLatencyMs}`
                      : '-'}
                  </span>
                );
              },
            },
            {
              title: t('channel.edit.model_tester.table.tested_at'),
              key: 'tested_at',
              width: displayedColumnWidths[batchSelectionMode ? 5 : 4],
              render: (_, row) => {
                const normalizedEndpoint = getEffectiveModelEndpoint(row);
                const item = modelTestResultsByKey.get(
                  buildModelTestResultKey(row.model, normalizedEndpoint),
                );
                return (
                  <span className='router-nowrap'>
                    {item?.tested_at > 0 ? timestamp2string(item.tested_at) : '-'}
                  </span>
                );
              },
            },
            {
              title: t('channel.edit.model_tester.table.actions'),
              key: 'actions',
              width: displayedColumnWidths[batchSelectionMode ? 6 : 5],
              render: (_, row) => {
                const activeTask = activeChannelTasksByModel.get(row.model) || null;
                return (
                  <div className='router-inline-actions router-table-actions-compact'>
                    <AppButton
                      type='button'
                      className='router-inline-button'
                      loading={
                        (modelTesting &&
                          modelTestingScope === 'single' &&
                          modelTestingTargetSet.has(row.model)) ||
                        !!activeTask
                      }
                      disabled={
                        disabledBase ||
                        modelTesting ||
                        batchSelectionMode ||
                        activeChannelTasksByModel.has(row.model)
                      }
                      onClick={() =>
                        handleRunModelTests({
                          targetModels: [row.model],
                          scope: 'single',
                        })
                      }
                    >
                      {t('channel.edit.model_tester.single')}
                    </AppButton>
                    <AppButton
                      type='button'
                      className='router-inline-button'
                      onClick={() =>
                        openChannelTaskView({
                          type: 'channel_model_test',
                          model: row.model,
                        })
                      }
                    >
                      {t('channel.edit.model_tester.history')}
                    </AppButton>
                  </div>
                );
              },
            },
          ]}
        />
      </div>
    </AppDetailSection>
  );
};

export default ChannelDetailTestsTab;
