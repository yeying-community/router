import React, { useMemo, useState } from 'react';
import {
  Button,
  Checkbox,
  Dropdown,
  Form,
  Label,
  Message,
  Popup,
  Table,
} from 'semantic-ui-react';

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

  const displayedColumnWidths = useMemo(() => {
    if (batchSelectionMode) {
      return columnWidths;
    }
    return columnWidths.slice(1);
  }, [batchSelectionMode, columnWidths]);

  return (
    <section className='router-entity-detail-section'>
      <div className='router-entity-detail-section-header'>
        <div className='router-toolbar-start'>
          <span className='router-entity-detail-section-title'>
            {t('channel.edit.model_tester.title')}
          </span>
        </div>
      </div>
      <Form.Field>
        <Message info className='router-section-message'>
          {t('channel.edit.model_tester.hint')}
        </Message>
        <div className='router-toolbar router-block-gap-sm'>
          <div className='router-toolbar-start router-block-gap-sm'>
            <Dropdown
              selection
              className='router-section-dropdown router-detail-filter-dropdown router-dropdown-min-170'
              options={providerOptions}
              value={providerFilter || undefined}
              disabled={disabledBase || providerOptions.length === 0}
              placeholder={t('channel.edit.model_tester.filters.provider')}
              onChange={(e, { value }) =>
                {
                  const nextValue = (value || '').toString();
                  setProviderFilter(nextValue);
                  if (nextValue !== '') {
                    window.localStorage.setItem(providerStorageKey, nextValue);
                  }
                }
              }
            />
            <Dropdown
              selection
              className='router-section-dropdown router-detail-filter-dropdown router-dropdown-min-170'
              options={typeOptions}
              value={typeFilter || undefined}
              disabled={disabledBase || typeOptions.length === 0}
              placeholder={t('channel.edit.model_tester.filters.type')}
              onChange={(e, { value }) =>
                {
                  const nextValue = (value || '').toString();
                  setTypeFilter(nextValue);
                  if (nextValue !== '') {
                    window.localStorage.setItem(typeStorageKey, nextValue);
                  }
                }
              }
            />
            <Dropdown
              selection
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
            <Popup
              basic
              on='click'
              open={settingsOpen}
              onClose={() => setSettingsOpen(false)}
              position='bottom left'
              trigger={
                <Button
                  type='button'
                  className='router-page-button'
                  basic
                  disabled={disabledBase || filteredRows.length === 0}
                  onClick={() => setSettingsOpen((prev) => !prev)}
                >
                  {t('channel.edit.model_tester.settings_button')}
                </Button>
              }
              content={
                <div className='router-log-filter-editor'>
                  <div className='router-log-filter-editor-title'>
                    {t('channel.edit.model_tester.settings_title')}
                  </div>
                  {streamCapableRows.length > 0 ? (
                    <Form.Field style={{ marginBottom: 0 }}>
                      <Checkbox
                        toggle
                        label={t('channel.edit.model_tester.settings_stream')}
                        checked={batchStreamValue}
                        disabled={disabledBase}
                        onChange={(e, { checked }) =>
                          updateAllModelTestStreams(
                            !!checked,
                            streamCapableModelIDs,
                          )
                        }
                      />
                    </Form.Field>
                  ) : null}
                  <Form.Field style={{ marginBottom: 0, marginTop: 12 }}>
                    <label>
                      {t('channel.edit.model_tester.settings_audio_language')}
                    </label>
                    <Dropdown
                      selection
                      className='router-section-dropdown router-dropdown-min-170'
                      options={audioLanguageOptions}
                      value={audioTestLanguage || 'zh-CN'}
                      onChange={(e, { value }) =>
                        setAudioTestLanguage((value || 'zh-CN').toString())
                      }
                    />
                  </Form.Field>
                </div>
              }
            />
          </div>
          <div className='router-toolbar-end router-block-gap-sm'>
            <Button
              type='button'
              className='router-section-button'
              color='blue'
              loading={modelTesting && modelTestingScope === 'batch'}
              disabled={
                disabledBase ||
                modelTesting ||
                (batchSelectionMode &&
                  (filteredSelectedCount === 0 ||
                    selectedModelTestHasActiveTasks))
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
            </Button>
            {batchSelectionMode && (
              <Button
                type='button'
                className='router-page-button'
                basic
                disabled={disabledBase || modelTesting}
                onClick={() => setBatchSelectionMode(false)}
              >
                {t('common.cancel')}
              </Button>
            )}
            <Button
              type='button'
              className='router-page-button'
              basic
              onClick={() =>
                openChannelTaskView({
                  type: 'channel_model_test',
                })
              }
            >
              {t('channel.edit.model_tester.history_tasks')}
            </Button>
          </div>
        </div>
        {modelTestError && (
          <div className='router-error-text router-block-gap-sm'>
            {modelTestError}
          </div>
        )}
        <Table
          celled
          stackable
          className='router-detail-table router-model-test-table'
        >
          <colgroup>
            {displayedColumnWidths.map((width, index) => (
              <col
                key={`channel-model-test-col-${index}`}
                style={{ width }}
              />
            ))}
          </colgroup>
          <Table.Header>
            <Table.Row>
              {batchSelectionMode && (
                <Table.HeaderCell collapsing textAlign='center'>
                  <Checkbox
                    checked={filteredAllSelected}
                    indeterminate={filteredPartiallySelected}
                    disabled={disabledBase || filteredRows.length === 0}
                    onChange={(e, { checked }) =>
                      toggleFilteredTargets(!!checked)
                    }
                  />
                </Table.HeaderCell>
              )}
              <Table.HeaderCell>
                {t('channel.edit.model_tester.table.model')}
              </Table.HeaderCell>
              <Table.HeaderCell>
                {t('channel.edit.model_tester.table.endpoint')}
              </Table.HeaderCell>
              <Table.HeaderCell collapsing>
                {t('channel.edit.model_tester.table.status')}
              </Table.HeaderCell>
              <Table.HeaderCell collapsing>
                {t('channel.edit.model_tester.table.latency')}
              </Table.HeaderCell>
              <Table.HeaderCell collapsing>
                {t('channel.edit.model_tester.table.tested_at')}
              </Table.HeaderCell>
              <Table.HeaderCell collapsing>
                {t('channel.edit.model_tester.table.actions')}
              </Table.HeaderCell>
            </Table.Row>
          </Table.Header>
          <Table.Body>
            {filteredRows.length === 0 ? (
              <Table.Row>
                <Table.Cell
                  className='router-empty-cell'
                  colSpan={batchSelectionMode ? '8' : '7'}
                >
                  {t(
                    modelTestRows.length === 0
                      ? 'channel.edit.model_tester.empty'
                      : 'channel.edit.model_selector.empty_filtered',
                  )}
                </Table.Cell>
              </Table.Row>
            ) : (
              filteredRows.map((row) => {
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
                const labelColor =
                  effectiveStatus === 'running'
                    ? 'blue'
                    : effectiveStatus === 'pending'
                      ? 'orange'
                      : effectiveStatus === 'untested'
                        ? undefined
                        : effectiveStatus === 'supported'
                          ? 'green'
                          : effectiveStatus === 'skipped'
                            ? 'grey'
                            : 'red';
                return (
                  <Table.Row key={row.model}>
                    {batchSelectionMode && (
                      <Table.Cell textAlign='center'>
                        <Checkbox
                          checked={modelTestTargetModels.includes(row.model)}
                          disabled={disabledBase}
                          onChange={(e, { checked }) =>
                            toggleModelTestTarget(row.model, !!checked)
                          }
                        />
                      </Table.Cell>
                    )}
                    <Table.Cell title={row.model || '-'}>
                      <span className='router-cell-truncate'>{row.model || '-'}</span>
                    </Table.Cell>
                    <Table.Cell className='router-table-dropdown-cell'>
                      {row.type === 'text' || row.type === 'image' || row.type === 'audio' ? (
                        <Dropdown
                          selection
                          className='router-mini-dropdown router-table-dropdown-fluid'
                          options={getEndpointOptionsForModel(row)}
                          disabled={disabledBase}
                          value={normalizedEndpoint}
                          onChange={(e, { value }) =>
                            updateModelTestEndpoint(row.model, value)
                          }
                        />
                      ) : (
                        normalizedEndpoint || row.endpoint || '-'
                      )}
                    </Table.Cell>
                    <Table.Cell>
                      {activeTask ? (
                        <Label basic color={labelColor} className='router-tag'>
                          {t(`channel.edit.model_tester.status.${effectiveStatus}`)}
                        </Label>
                      ) : (
                        <span className='router-nowrap'>
                          {(endpointSummary?.successCount || 0)}/
                          {(endpointSummary?.failureCount || 0)}
                        </span>
                      )}
                    </Table.Cell>
                    <Table.Cell className='router-nowrap'>
                      {endpointSummary?.latencyCount > 0
                        ? `${endpointSummary.minLatencyMs}/` +
                          `${Math.round(
                            endpointSummary.totalLatencyMs /
                              endpointSummary.latencyCount,
                          )}/` +
                          `${endpointSummary.maxLatencyMs}`
                        : '-'}
                    </Table.Cell>
                    <Table.Cell className='router-nowrap'>
                      {item?.tested_at > 0
                        ? timestamp2string(item.tested_at)
                        : '-'}
                    </Table.Cell>
                    <Table.Cell collapsing>
                      <div className='router-inline-actions'>
                        <Button
                          type='button'
                          className='router-inline-button'
                          basic
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
                        </Button>
                        <Button
                          type='button'
                          className='router-inline-button'
                          basic
                          onClick={() =>
                            openChannelTaskView({
                              type: 'channel_model_test',
                              model: row.model,
                            })
                          }
                        >
                          {t('channel.edit.model_tester.history')}
                        </Button>
                      </div>
                    </Table.Cell>
                  </Table.Row>
                );
              })
            )}
          </Table.Body>
        </Table>
      </Form.Field>
    </section>
  );
};

export default ChannelDetailTestsTab;
