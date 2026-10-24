import React, { useMemo, useState } from 'react';
import {
  AppAlert,
  AppButton,
  AppDetailSection,
  AppDrawer,
  AppFilterHeader,
  AppInput,
  AppPagination,
  AppPopover,
  resolvePopupContainer,
  AppSelect,
  AppSwitch,
  AppTable,
  AppTableActionButton,
  AppTag,
  AppTooltip,
} from '../../../router-ui';

const normalizeRecentTestStatus = (value) => {
  const normalized = (value || '').toString().trim().toLowerCase();
  if (
    normalized === 'supported' ||
    normalized === 'success'
  ) {
    return 'success';
  }
  if (
    normalized === 'unsupported' ||
    normalized === 'skipped' ||
    normalized === 'failed'
  ) {
    return 'failed';
  }
  if (normalized === 'pending' || normalized === 'running') {
    return normalized;
  }
  return normalized || 'untested';
};

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
  modelTesting,
  modelTestingScope,
  modelTestingTargetSet,
  handleRunModelTests,
  detailTestingReadonly,
  modelTestError,
  onDownloadTestArtifact,
  initialHistoryModel,
  initialHistoryEndpoint,
  onCloseHistoryFilter,
  selectedModelTestHasActiveTasks,
  timestamp2string,
  updateAllModelTestStreams,
  resolvePreferredProviderForModel,
  normalizeChannelModelType,
  audioTestLanguage,
  setAudioTestLanguage,
  responsesTestMode,
  setResponsesTestMode,
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

  const [historyOpen, setHistoryOpen] = useState(
    Boolean(initialHistoryModel || initialHistoryEndpoint),
  );
  const [historyModel, setHistoryModel] = useState(
    (initialHistoryModel || '').toString().trim(),
  );
  const [historyEndpoint, setHistoryEndpoint] = useState(
    (initialHistoryEndpoint || '').toString().trim(),
  );
  const [historyPage, setHistoryPage] = useState(1);
  const [historyPageSize, setHistoryPageSize] = useState(10);

  if (inputs.protocol === 'proxy') {
    return null;
  }

  const getRowTestKey = (row) =>
    (row?.test_key || buildModelTestResultKey(row?.model, row?.endpoint || ''))
      .toString()
      .trim();

  const getRowEndpoint = (row) =>
    (row?.endpoint || getEffectiveModelEndpoint(row) || '').toString().trim();

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
  const responsesTestModeOptions = useMemo(
    () => [
      {
        key: 'text',
        value: 'text',
        text: t('channel.edit.model_tester.responses_test_mode_options.text'),
      },
      {
        key: 'image_generation',
        value: 'image_generation',
        text: t(
          'channel.edit.model_tester.responses_test_mode_options.image_generation',
        ),
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

  const filteredTargetKeys = useMemo(
    () => filteredRows.map((row) => getRowTestKey(row)).filter(Boolean),
    [filteredRows],
  );
  const filteredTargetSet = useMemo(
    () => new Set(modelTestTargetModels),
    [modelTestTargetModels],
  );
  const filteredAllSelected =
    filteredRows.length > 0 &&
    filteredRows.every((row) => filteredTargetSet.has(getRowTestKey(row)));
  const filteredPartiallySelected =
    !filteredAllSelected &&
    filteredRows.some((row) => filteredTargetSet.has(getRowTestKey(row)));
  const filteredSelectedCount = filteredRows.filter((row) =>
    filteredTargetSet.has(getRowTestKey(row)),
  ).length;

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
  const hasResponsesTestRows = useMemo(
    () =>
      filteredRows.some((row) => getRowEndpoint(row) === '/v1/responses'),
    [filteredRows],
  );

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

  const historyRows = useMemo(() => {
    const rows = (Array.isArray(modelTestResults) ? modelTestResults : [])
      .filter((item) => {
        const modelName = (item?.model || '').toString().trim();
        const endpoint = (item?.endpoint || '').toString().trim();
        if (historyModel !== '' && modelName !== historyModel) {
          return false;
        }
        if (historyEndpoint !== '' && endpoint !== historyEndpoint) {
          return false;
        }
        return modelName !== '' && endpoint !== '';
      })
      .slice();
    rows.sort((left, right) => {
      const testedAtDiff =
        Number(right?.tested_at || 0) - Number(left?.tested_at || 0);
      if (testedAtDiff !== 0) {
        return testedAtDiff;
      }
      const roundDiff = Number(right?.round || 0) - Number(left?.round || 0);
      if (roundDiff !== 0) {
        return roundDiff;
      }
      const modelDiff = (left?.model || '').localeCompare(right?.model || '');
      if (modelDiff !== 0) {
        return modelDiff;
      }
      return (left?.endpoint || '').localeCompare(right?.endpoint || '');
    });
    return rows;
  }, [historyEndpoint, historyModel, modelTestResults]);

  const pagedHistoryRows = useMemo(() => {
    const offset = (historyPage - 1) * historyPageSize;
    return historyRows.slice(offset, offset + historyPageSize);
  }, [historyPage, historyPageSize, historyRows]);

  React.useEffect(() => {
    const maxPage = Math.max(
      1,
      Math.ceil(historyRows.length / historyPageSize),
    );
    if (historyPage > maxPage) {
      setHistoryPage(maxPage);
    }
  }, [historyPage, historyPageSize, historyRows.length]);

  const openTestHistory = (row = null) => {
    setHistoryModel((row?.model || '').toString().trim());
    setHistoryEndpoint(row ? getRowEndpoint(row) : '');
    setHistoryPage(1);
    setHistoryOpen(true);
  };

  const closeTestHistory = () => {
    setHistoryOpen(false);
    onCloseHistoryFilter?.();
  };

  const historyTitle =
    historyModel === ''
      ? t('channel.edit.model_tester.history_title')
      : t('channel.edit.model_tester.history_title_filtered', {
          model: historyModel,
          endpoint: historyEndpoint,
        });

  const toggleFilteredTargets = (checked) => {
    const targetSet = new Set(filteredTargetSet);
    filteredTargetKeys.forEach((targetKey) => {
      if (checked) {
        targetSet.add(targetKey);
      } else {
        targetSet.delete(targetKey);
      }
    });
    const nextSelected = Array.from(targetSet);
    modelTestRows.forEach((row) => {
      const targetKey = getRowTestKey(row);
      const shouldSelect = nextSelected.includes(targetKey);
      const isSelected = filteredTargetSet.has(targetKey);
      if (shouldSelect !== isSelected) {
        toggleModelTestTarget(targetKey, shouldSelect);
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
          toggleModelTestTarget(getRowTestKey(record), selected);
        },
        onSelectAll: (selected) => {
          toggleFilteredTargets(selected);
        },
      }
    : undefined;

  return (
    <>
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
                      {t('channel.edit.model_tester.responses_test_mode')}
                    </label>
                    <AppSelect
                      className='router-section-dropdown router-dropdown-min-170'
                      disabled={!hasResponsesTestRows}
                      getPopupContainer={resolvePopupContainer}
                      options={responsesTestModeOptions}
                      value={responsesTestMode || 'text'}
                      onChange={(e, { value }) =>
                        setResponsesTestMode((value || 'text').toString())
                      }
                    />
                  </div>
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
                        'channel.edit.model_tester.image_edit_source_url_placeholder',
                      )}
                      onChange={(e, { value }) =>
                        setImageEditTestURL((value || '').toString())
                      }
                    />
                  </div>
                  <div className='router-block-gap-xs'>
                    <label htmlFor='channel-model-test-image-upload'>
                      {t('channel.edit.model_tester.image_edit_upload')}
                    </label>
                    <input
                      id='channel-model-test-image-upload'
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
              onClick={() => openTestHistory()}
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
          rowKey={(row) => getRowTestKey(row)}
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
                const normalizedEndpoint = getRowEndpoint(row);
                return (
                  <span
                    className='router-cell-truncate'
                    title={normalizedEndpoint || '-'}
                  >
                    {normalizedEndpoint || '-'}
                  </span>
                );
              },
            },
            {
              title: t('channel.edit.model_tester.table.status'),
              key: 'status',
              width: displayedColumnWidths[batchSelectionMode ? 3 : 2],
              render: (_, row) => {
                const normalizedEndpoint = getRowEndpoint(row);
                const rowTestKey = getRowTestKey(row);
                const item = modelTestResultsByKey.get(
                  buildModelTestResultKey(row.model, normalizedEndpoint),
                );
                const activeTask =
                  activeChannelTasksByModel.get(rowTestKey) || null;
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
              title: t('channel.edit.model_tester.table.recent_test'),
              key: 'recent_test',
              width: displayedColumnWidths[batchSelectionMode ? 4 : 3],
              render: (_, row) => {
                const normalizedEndpoint = getRowEndpoint(row);
                const rowTestKey = getRowTestKey(row);
                const item = modelTestResultsByKey.get(
                  buildModelTestResultKey(row.model, normalizedEndpoint),
                );
                const activeTask =
                  activeChannelTasksByModel.get(rowTestKey) || null;
                const effectiveStatus = normalizeRecentTestStatus(
                  activeTask?.status ||
                    item?.status ||
                    row?.endpoint_last_test_status ||
                    row?.last_test_status,
                );
                const tagColor =
                  effectiveStatus === 'running'
                    ? 'blue'
                    : effectiveStatus === 'pending'
                      ? 'orange'
                      : effectiveStatus === 'success'
                        ? 'green'
                        : effectiveStatus === 'untested'
                          ? undefined
                          : 'red';
                const failureReason =
                  effectiveStatus !== 'success' &&
                  effectiveStatus !== 'running' &&
                  effectiveStatus !== 'pending' &&
                  effectiveStatus !== 'untested'
                    ? (
                        item?.message ||
                        row?.endpoint_last_test_error ||
                        row?.last_test_error ||
                        ''
                      )
                        .toString()
                        .trim()
                    : '';
                const recentTestTag = (
                  <AppTag color={tagColor} className='router-tag'>
                    {t(`channel.edit.model_tester.status.${effectiveStatus}`)}
                  </AppTag>
                );
                if (failureReason) {
                  return (
                    <AppTooltip
                      title={t('channel.edit.model_tester.failure_reason', {
                        reason: failureReason,
                      })}
                    >
                      <span className='router-nowrap'>{recentTestTag}</span>
                    </AppTooltip>
                  );
                }
                return recentTestTag;
              },
            },
            {
              title: t('channel.edit.model_tester.table.latency'),
              key: 'latency',
              width: displayedColumnWidths[batchSelectionMode ? 5 : 4],
              render: (_, row) => {
                const normalizedEndpoint = getRowEndpoint(row);
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
              width: displayedColumnWidths[batchSelectionMode ? 6 : 5],
              render: (_, row) => {
                const normalizedEndpoint = getRowEndpoint(row);
                const item = modelTestResultsByKey.get(
                  buildModelTestResultKey(row.model, normalizedEndpoint),
                );
                const testedAt = item?.tested_at || row?.endpoint_last_tested_at || 0;
                return (
                  <span className='router-nowrap'>
                    {testedAt > 0 ? timestamp2string(testedAt) : '-'}
                  </span>
                );
              },
            },
              {
                title: t('channel.edit.model_tester.table.actions'),
                key: 'actions',
                width: displayedColumnWidths[batchSelectionMode ? 7 : 6],
                render: (_, row) => {
                  const rowTestKey = getRowTestKey(row);
                  const activeTask =
                    activeChannelTasksByModel.get(rowTestKey) || null;
                  return (
                  <div className='router-inline-actions router-table-actions-icon-compact'>
                    <AppTableActionButton
                      icon='exchange'
                      title={t('channel.edit.model_tester.single')}
                      loading={
                        (modelTesting &&
                          modelTestingScope === 'single' &&
                          modelTestingTargetSet.has(rowTestKey)) ||
                        !!activeTask
                      }
                      disabled={
                        disabledBase ||
                        modelTesting ||
                        batchSelectionMode ||
                        activeChannelTasksByModel.has(rowTestKey)
                      }
                      onClick={() =>
                        handleRunModelTests({
                          targetModels: [rowTestKey],
                          scope: 'single',
                        })
                      }
                    />
                    <AppTableActionButton
                      icon='history'
                      title={t('channel.edit.model_tester.history')}
                      onClick={() => openTestHistory(row)}
                    />
                  </div>
                );
              },
            },
          ]}
        />
      </div>
      </AppDetailSection>
      <AppDrawer
        open={historyOpen}
        onClose={closeTestHistory}
        placement='right'
        size='large'
        title={historyTitle}
        className='channel-model-test-history-drawer'
      >
        <div className='router-page-stack'>
          <AppAlert
            type='info'
            showIcon
            title={t('channel.edit.model_tester.history_hint')}
          />
          <AppTable
            className='router-detail-table'
            pagination={false}
            scroll={{ x: 1120 }}
            rowKey={(item) =>
              [
                item?.channel_id,
                item?.model,
                item?.round,
                item?.endpoint,
                item?.is_stream,
                item?.tested_at,
              ].join('::')
            }
            dataSource={pagedHistoryRows}
            locale={{
              emptyText: t('channel.edit.model_tester.history_empty'),
            }}
            columns={[
              {
                title: t('channel.edit.model_tester.table.model'),
                dataIndex: 'model',
                key: 'model',
                width: 190,
                render: (value) => (
                  <span className='router-monospace-value' title={value || '-'}>
                    {value || '-'}
                  </span>
                ),
              },
              {
                title: t('channel.edit.model_tester.table.endpoint'),
                dataIndex: 'endpoint',
                key: 'endpoint',
                width: 190,
                render: (value) => (
                  <span className='router-cell-truncate' title={value || '-'}>
                    {value || '-'}
                  </span>
                ),
              },
              {
                title: t('channel.edit.model_tester.history_table.result'),
                key: 'result',
                width: 100,
                render: (_, item) => {
                  const status = (item?.status || 'unsupported')
                    .toString()
                    .trim()
                    .toLowerCase();
                  const color =
                    status === 'supported'
                      ? 'green'
                      : status === 'skipped'
                      ? 'grey'
                      : 'red';
                  return (
                    <AppTag color={color} className='router-tag'>
                      {t(`channel.edit.model_tester.status.${status}`)}
                    </AppTag>
                  );
                },
              },
              {
                title: t('channel.edit.model_tester.history_table.mode'),
                key: 'mode',
                width: 100,
                render: (_, item) =>
                  item?.is_stream === true
                    ? t('channel.edit.model_tester.history_stream')
                    : item?.is_stream === false
                    ? t('channel.edit.model_tester.history_non_stream')
                    : '-',
              },
              {
                title: t('channel.edit.model_tester.history_table.source'),
                dataIndex: 'source',
                key: 'source',
                width: 120,
                render: (value) =>
                  t(
                    `channel.edit.model_tester.history_sources.${
                      value || 'manual_test'
                    }`,
                    {
                      defaultValue: value || '-',
                    },
                  ),
              },
              {
                title: t('channel.edit.model_tester.history_table.latency'),
                dataIndex: 'latency_ms',
                key: 'latency_ms',
                width: 90,
                render: (value) =>
                  Number(value || 0) > 0 ? `${value} ms` : '-',
              },
              {
                title: t('channel.edit.model_tester.history_table.message'),
                dataIndex: 'message',
                key: 'message',
                width: 220,
                render: (value) => (
                  <span className='router-cell-truncate' title={value || '-'}>
                    {value || '-'}
                  </span>
                ),
              },
              {
                title: t('channel.edit.model_tester.history_table.tested_at'),
                dataIndex: 'tested_at',
                key: 'tested_at',
                width: 170,
                render: (value) =>
                  Number(value || 0) > 0 ? timestamp2string(value) : '-',
              },
              {
                title: t('channel.edit.model_tester.table.actions'),
                key: 'actions',
                width: 100,
                fixed: 'right',
                render: (_, item) => {
                  const rowTestKey = buildModelTestResultKey(
                    item?.model,
                    item?.endpoint,
                  );
                  const canRetest = rowsWithMeta.some(
                    (row) => getRowTestKey(row) === rowTestKey,
                  );
                  return (
                    <div className='router-inline-actions router-table-actions-icon-compact'>
                      <AppTableActionButton
                        icon='exchange'
                        title={t('channel.edit.model_tester.history_retest')}
                        disabled={
                          disabledBase ||
                          modelTesting ||
                          !canRetest ||
                          activeChannelTasksByModel.has(rowTestKey)
                        }
                        onClick={() => {
                          closeTestHistory();
                          handleRunModelTests({
                            targetModels: [rowTestKey],
                            scope: 'single',
                          });
                        }}
                      />
                      <AppTableActionButton
                        icon='download'
                        title={t('channel.edit.model_tester.history_download')}
                        disabled={
                          !(item?.artifact_name || item?.artifact_path) ||
                          typeof onDownloadTestArtifact !== 'function'
                        }
                        onClick={() => onDownloadTestArtifact?.(item)}
                      />
                    </div>
                  );
                },
              },
            ]}
          />
          {historyRows.length > 0 ? (
            <AppPagination
              current={historyPage}
              total={historyRows.length}
              pageSize={historyPageSize}
              onChange={(nextPage, nextPageSize) => {
                if (nextPageSize !== historyPageSize) {
                  setHistoryPageSize(nextPageSize);
                  setHistoryPage(1);
                  return;
                }
                setHistoryPage(nextPage);
              }}
            />
          ) : null}
        </div>
      </AppDrawer>
    </>
  );
};

export default ChannelDetailTestsTab;
