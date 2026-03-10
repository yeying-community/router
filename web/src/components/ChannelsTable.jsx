import React, { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Button,
  Form,
  Icon,
  Input,
  Label,
  Pagination,
  Popup,
  Table,
} from 'semantic-ui-react';
import { Link, useNavigate } from 'react-router-dom';
import {
  API,
  showError,
  showInfo,
  showSuccess,
  timestamp2string,
} from '../helpers';

import { ITEMS_PER_PAGE } from '../constants';
import {
  getChannelProtocolOptions,
  loadChannelProtocolOptions,
} from '../helpers/helper';
import { renderNumber } from '../helpers/render';

function renderTimestamp(timestamp) {
  return <>{timestamp2string(timestamp)}</>;
}

async function fetchSelectedChannelModels(channelId) {
  const normalizedChannelId = (channelId || '').toString().trim();
  if (normalizedChannelId === '') {
    return [];
  }
  const items = [];
  let page = 0;
  while (page < 50) {
    const res = await API.get(`/api/v1/admin/channel/${normalizedChannelId}/models`, {
      params: {
        p: page,
        page_size: 100,
      },
    });
    const { success, data } = res.data || {};
    if (!success) {
      return [];
    }
    const pageItems = Array.isArray(data?.items) ? data.items : [];
    items.push(...pageItems);
    const total = Number(data?.total || pageItems.length || 0);
    if (
      pageItems.length === 0 ||
      items.length >= total ||
      pageItems.length < 100
    ) {
      break;
    }
    page += 1;
  }
  return items
    .filter((item) => item && item.selected === true && item.inactive !== true)
    .map((item) => (item.model || '').toString().trim())
    .filter((item) => item !== '');
}

async function fetchChannelLatestTests(channelId) {
  const normalizedChannelId = (channelId || '').toString().trim();
  if (normalizedChannelId === '') {
    return {
      last_tested_at: 0,
      items: [],
    };
  }
  const res = await API.get(`/api/v1/admin/channel/${normalizedChannelId}/tests`);
  const { success, data } = res.data || {};
  if (!success) {
    return {
      last_tested_at: 0,
      items: [],
    };
  }
  return {
    last_tested_at: Number(data?.last_tested_at || 0),
    items: Array.isArray(data?.items) ? data.items : [],
  };
}

function buildProtocolMap(options, t) {
  const protocolMap = {};
  if (Array.isArray(options)) {
    options.forEach((option) => {
      if (
        option &&
        typeof option.value === 'string' &&
        option.value.trim() !== ''
      ) {
        protocolMap[option.value] = option;
      }
    });
  }
  protocolMap.unknown = {
    value: 'unknown',
    text: t('channel.table.status_unknown'),
    color: 'grey',
  };
  return protocolMap;
}

function renderProtocol(protocol, protocolMap) {
  const normalized = (protocol || '').toString().trim().toLowerCase();
  const option = protocolMap[normalized] || protocolMap.unknown;
  const colorClassMap = {
    grey: 'router-text-muted',
    green: 'router-text-success',
    red: 'router-text-danger',
    yellow: 'router-text-warning',
    olive: 'router-text-olive',
    blue: 'router-text-info',
    orange: 'router-text-warning',
  };
  return (
    <span className={colorClassMap[option?.color] || undefined}>
      {option ? option.text : normalized || 'unknown'}
    </span>
  );
}

function getChannelDisplayName(channel) {
  const name = (channel?.name || '').toString().trim();
  if (name !== '') {
    return name;
  }
  return '-';
}

function renderChannelName(channel, t) {
  const displayName = getChannelDisplayName(channel);
  return <span>{displayName || t('channel.table.no_name')}</span>;
}

function renderBalance(protocol, balance, t) {
  const normalized = (protocol || '').toString().trim().toLowerCase();
  switch (normalized) {
    case 'openai':
      if (balance === 0) {
        return <span>{t('channel.table.balance_not_supported')}</span>;
      }
      return <span>${balance.toFixed(2)}</span>;
    case 'closeai':
      return <span>¥{balance.toFixed(2)}</span>;
    case 'custom':
      return <span>${balance.toFixed(2)}</span>;
    case 'openai-sb':
      return <span>¥{(balance / 10000).toFixed(2)}</span>;
    case 'aiproxy':
      return <span>{renderNumber(balance)}</span>;
    case 'api2gpt':
      return <span>¥{balance.toFixed(2)}</span>;
    case 'aigc2d':
      return <span>{renderNumber(balance)}</span>;
    case 'openrouter':
      return <span>${balance.toFixed(2)}</span>;
    case 'deepseek':
      return <span>¥{balance.toFixed(2)}</span>;
    case 'siliconflow':
      return <span>¥{balance.toFixed(2)}</span>;
    default:
      return <span>{t('channel.table.balance_not_supported')}</span>;
  }
}

const selectionModeNone = '';
const selectionModeDelete = 'delete';
const selectionModeDisable = 'disable';
const channelStatusCreating = 4;
const CHANNEL_CREATE_CACHE_KEY = 'router.channel.create.v3';
const CREATE_CHANNEL_STEP_MIN = 1;
const CREATE_CHANNEL_STEP_MAX = 4;

function parseCreateStep(rawStep) {
  const step = Number(rawStep);
  if (!Number.isInteger(step)) {
    return CREATE_CHANNEL_STEP_MIN;
  }
  if (step < CREATE_CHANNEL_STEP_MIN) {
    return CREATE_CHANNEL_STEP_MIN;
  }
  if (step > CREATE_CHANNEL_STEP_MAX) {
    return CREATE_CHANNEL_STEP_MAX;
  }
  return step;
}

function readStoredCreatingStep(channelId) {
  if (typeof window === 'undefined') {
    return 0;
  }
  const targetChannelId = (channelId || '').toString().trim();
  if (targetChannelId === '') {
    return 0;
  }
  try {
    const raw = localStorage.getItem(CHANNEL_CREATE_CACHE_KEY);
    if (!raw) {
      return 0;
    }
    const cachedState = JSON.parse(raw);
    if (!cachedState || typeof cachedState !== 'object') {
      return 0;
    }
    if ((cachedState.channel_id || '').toString().trim() !== targetChannelId) {
      return 0;
    }
    return parseCreateStep(cachedState.step);
  } catch {
    return 0;
  }
}

const ChannelsTable = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [channels, setChannels] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activePage, setActivePage] = useState(1);
  const [totalChannels, setTotalChannels] = useState(0);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [searching, setSearching] = useState(false);
  const [selectionMode, setSelectionMode] = useState(selectionModeNone);
  const [batchDeleting, setBatchDeleting] = useState(false);
  const [batchDisabling, setBatchDisabling] = useState(false);
  const [selectedChannelIds, setSelectedChannelIds] = useState([]);
  const [protocolMap, setProtocolMap] = useState(() =>
    buildProtocolMap(getChannelProtocolOptions(), t)
  );

  const processChannelData = useCallback((channel) => {
    const next = { ...channel };
    next.id = (next.id || '').toString().trim();
    next.protocol = (next.protocol || '').toString().trim().toLowerCase();
    if (next.protocol === '') {
      next.protocol = 'openai';
    }
    return next;
  }, []);

  const loadChannels = useCallback(
    async ({ page = 1, keyword = '' } = {}) => {
      const normalizedPage = Number(page) > 0 ? Number(page) : 1;
      const normalizedKeyword = (keyword || '').toString().trim();
      const res = await API.get('/api/v1/admin/channels/', {
        params: {
          page: normalizedPage,
          page_size: ITEMS_PER_PAGE,
          keyword: normalizedKeyword,
        },
      });
      const { success, message, data } = res.data;
      if (success) {
        const items = Array.isArray(data?.items) ? data.items : [];
        setChannels(items.map(processChannelData));
        const total = Number(data?.total || 0);
        setTotalChannels(Number.isFinite(total) && total >= 0 ? total : 0);
      } else {
        showError(message);
      }
      setLoading(false);
    },
    [processChannelData]
  );

  const onPaginationChange = (e, { activePage }) => {
    (async () => {
      const nextPage = Number(activePage) > 0 ? Number(activePage) : 1;
      setLoading(true);
      await loadChannels({ page: nextPage, keyword: searchKeyword });
      setActivePage(nextPage);
    })();
  };

  const refresh = async () => {
    setLoading(true);
    await loadChannels({ page: activePage, keyword: searchKeyword });
  };

  useEffect(() => {
    loadChannels({ page: 1, keyword: '' })
      .then()
      .catch((reason) => {
        showError(reason);
      });
  }, [loadChannels]);

  useEffect(() => {
    let disposed = false;
    setProtocolMap(buildProtocolMap(getChannelProtocolOptions(), t));
    loadChannelProtocolOptions().then((options) => {
      if (disposed) {
        return;
      }
      setProtocolMap(buildProtocolMap(options, t));
    });
    return () => {
      disposed = true;
    };
  }, [t]);

  useEffect(() => {
    if (selectionMode === selectionModeNone) {
      return;
    }
    const validIds = new Set(channels.map((channel) => channel.id));
    setSelectedChannelIds((prev) => prev.filter((id) => validIds.has(id)));
  }, [selectionMode, channels]);

  const manageChannel = async (id, action, idx, value) => {
    const normalizedID = (id || '').toString().trim();
    if (normalizedID === '') {
      showError('渠道 ID 无效');
      return;
    }
    let data = { id: normalizedID };
    let res;
    switch (action) {
      case 'delete':
        res = await API.delete(
          `/api/v1/admin/channel/${encodeURIComponent(normalizedID)}/`
        );
        break;
      case 'enable':
        data.status = 1;
        res = await API.put('/api/v1/admin/channel/', data);
        break;
      case 'disable':
        data.status = 2;
        res = await API.put('/api/v1/admin/channel/', data);
        break;
      case 'priority':
        if (value === '') {
          return;
        }
        data.priority = parseInt(value);
        res = await API.put('/api/v1/admin/channel/', data);
        break;
      case 'weight':
        if (value === '') {
          return;
        }
        data.weight = parseInt(value);
        if (data.weight < 0) {
          data.weight = 0;
        }
        res = await API.put('/api/v1/admin/channel/', data);
        break;
      default:
        return;
    }
    const { success, message } = res.data;
    if (success) {
      showSuccess(t('channel.messages.operation_success'));
      setLoading(true);
      await loadChannels({ page: activePage, keyword: searchKeyword });
    } else {
      showError(message);
    }
  };

  const renderStatus = (status, t) => {
    const plainStatusText = (text, className) => (
      <span className={className}>{text}</span>
    );
    switch (status) {
      case 1:
        return plainStatusText(
          t('channel.table.status_enabled'),
          'router-text-success'
        );
      case 2:
        return (
          <Popup
            trigger={
              <span className='router-text-danger'>
                {t('channel.table.status_disabled')}
              </span>
            }
            content={t('channel.table.status_disabled_tip')}
            basic
          />
        );
      case 3:
        return (
          <Popup
            trigger={
              <span className='router-text-warning'>
                {t('channel.table.status_auto_disabled')}
              </span>
            }
            content={t('channel.table.status_auto_disabled_tip')}
            basic
          />
        );
      case channelStatusCreating:
        return plainStatusText(
          t('channel.table.status_creating'),
          'router-text-info'
        );
      default:
        return plainStatusText(
          t('channel.table.status_unknown'),
          'router-text-muted'
        );
    }
  };

  const renderCapabilities = (capabilities, t) => {
    const normalized = Array.isArray(capabilities)
      ? capabilities.filter(Boolean).map((item) => item.toString().toLowerCase())
      : [];
    if (normalized.length === 0) {
      return <span className='router-text-muted'>-</span>;
    }
    const order = ['text', 'image', 'audio', 'video'];
    const capabilitySet = new Set(normalized);
    const ordered = order.filter((item) => capabilitySet.has(item));
    return ordered.map((capability, index) => (
      <span key={`${capability}-${index}`}>
        {t(`channel.model_types.${capability}`, capability)}
        {index === ordered.length - 1 ? '' : ' / '}
      </span>
    ));
  };

  const searchChannels = async () => {
    setSearching(true);
    setLoading(true);
    await loadChannels({ page: 1, keyword: searchKeyword });
    setActivePage(1);
    setSearching(false);
  };

  const updateChannelBalance = async (id, name, idx) => {
    const res = await API.get(`/api/v1/admin/channel/update_balance/${id}/`);
    const { success, message, balance } = res.data;
    if (success) {
      let newChannels = [...channels];
      let realIdx = idx;
      newChannels[realIdx].balance = balance;
      newChannels[realIdx].balance_updated_time = Date.now() / 1000;
      setChannels(newChannels);
      showSuccess(t('channel.messages.balance_update_success', { name }));
    } else {
      showError(message);
    }
  };

  const handleKeywordChange = async (e, { value }) => {
    setSearchKeyword(value.trim());
  };

  const sortChannel = (key) => {
    if (channels.length === 0) return;
    setLoading(true);
    let sortedChannels = [...channels];
    sortedChannels.sort((a, b) => {
      const leftValue = Array.isArray(a[key]) ? a[key].join(',') : a[key];
      const rightValue = Array.isArray(b[key]) ? b[key].join(',') : b[key];
      if (!isNaN(leftValue)) {
        // If the value is numeric, subtract to sort
        return leftValue - rightValue;
      } else {
        // If the value is not numeric, sort as strings
        return ('' + leftValue).localeCompare(String(rightValue));
      }
    });
    if (sortedChannels[0].id === channels[0].id) {
      sortedChannels.reverse();
    }
    setChannels(sortedChannels);
    setLoading(false);
  };

  const pagedChannels = channels;
  const pagedChannelIds = pagedChannels
    .filter((channel) => !channel.deleted)
    .map((channel) => channel.id);
  const allPagedSelected =
    pagedChannelIds.length > 0 &&
    pagedChannelIds.every((id) => selectedChannelIds.includes(id));
  const inBatchSelectMode = selectionMode !== selectionModeNone;
  const footerColSpan = 7 + (inBatchSelectMode ? 1 : 0);
  const actionBusy = batchDeleting || batchDisabling;

  const toggleChannelSelection = (channelId, checked) => {
    setSelectedChannelIds((prev) => {
      const next = new Set(prev);
      if (checked) {
        next.add(channelId);
      } else {
        next.delete(channelId);
      }
      return Array.from(next);
    });
  };

  const togglePagedSelection = (checked) => {
    setSelectedChannelIds((prev) => {
      const next = new Set(prev);
      pagedChannelIds.forEach((id) => {
        if (checked) {
          next.add(id);
        } else {
          next.delete(id);
        }
      });
      return Array.from(next);
    });
  };

  const cancelBatchSelection = () => {
    setSelectionMode(selectionModeNone);
    setSelectedChannelIds([]);
  };

  const inferCreatingStepFromState = (channel, selectedModels, testData) => {
    if (!channel || typeof channel !== 'object') {
      return 1;
    }
    if (channel.protocol === 'proxy') {
      return 1;
    }
    const models = Array.isArray(selectedModels) ? selectedModels : [];
    if (models.length === 0) {
      return 2;
    }
    const testedAt = Number(testData?.last_tested_at || 0);
    const testResults = Array.isArray(testData?.items) ? testData.items : [];
    if (testedAt > 0 || testResults.length > 0) {
      return 4;
    }
    if (models.length > 0) {
      return 3;
    }
    return 2;
  };

  const resolveCreatingStep = async (channel) => {
    if (!channel) {
      return 1;
    }
    const storedStep = readStoredCreatingStep(channel.id);
    if (storedStep > 0) {
      return storedStep;
    }
    try {
      const [selectedModels, testData] = await Promise.all([
        fetchSelectedChannelModels(channel.id),
        fetchChannelLatestTests(channel.id),
      ]);
      return inferCreatingStepFromState(channel, selectedModels, testData);
    } catch {
      // Fallback to step 1 when details cannot be loaded.
    }
    return 1;
  };

  const openChannelByStatus = async (channel) => {
    if (!channel || !channel.id || inBatchSelectMode) {
      return;
    }
    if (channel.status === channelStatusCreating) {
      const step = await resolveCreatingStep(channel);
      navigate(
        `/channel/add?channel_id=${encodeURIComponent(channel.id)}&step=${step}`
      );
      return;
    }
    navigate(`/channel/detail/${channel.id}`);
  };

  const stopRowClick = (event) => {
    event.stopPropagation();
  };

  const collectSelectedTargets = () => {
    return selectedChannelIds
      .map((id) => {
        const absoluteIndex = channels.findIndex(
          (channel) => channel.id === id
        );
        if (absoluteIndex < 0) return null;
        return {
          id,
          absoluteIndex,
          channel: channels[absoluteIndex],
        };
      })
      .filter(Boolean);
  };

  const confirmBatchDelete = async () => {
    if (selectedChannelIds.length === 0) {
      showInfo(t('channel.messages.batch_delete_select_required'));
      return;
    }
    const targets = collectSelectedTargets();
    if (targets.length === 0) {
      showInfo(t('channel.messages.batch_delete_select_required'));
      return;
    }

    setSelectionMode(selectionModeNone);
    setSelectedChannelIds([]);
    setBatchDeleting(true);

    const results = await Promise.allSettled(
      targets.map(async (target) => {
        const res = await API.delete(`/api/v1/admin/channel/${target.id}/`);
        const { success, message } = res.data || {};
        return {
          id: target.id,
          success: !!success,
          message: message || '',
        };
      })
    );

    const succeededIds = [];
    let firstFailedMessage = '';
    results.forEach((result) => {
      if (result.status === 'fulfilled' && result.value.success) {
        succeededIds.push(result.value.id);
      } else if (!firstFailedMessage) {
        if (result.status === 'fulfilled') {
          firstFailedMessage = result.value.message || 'Delete failed';
        } else {
          firstFailedMessage = result.reason?.message || `${result.reason}`;
        }
      }
    });

    if (succeededIds.length > 0) {
      const succeededSet = new Set(succeededIds);
      setChannels((prev) =>
        prev.map((channel) =>
          succeededSet.has(channel.id) ? { ...channel, deleted: true } : channel
        )
      );
    }

    const failedCount = results.length - succeededIds.length;
    showInfo(
      t('channel.messages.batch_delete_done', {
        success: succeededIds.length,
        failed: failedCount,
      })
    );
    if (firstFailedMessage) {
      showError(firstFailedMessage);
    }
    setLoading(true);
    await loadChannels({ page: activePage, keyword: searchKeyword });
    setBatchDeleting(false);
  };

  const confirmBatchDisable = async () => {
    if (selectedChannelIds.length === 0) {
      showInfo(t('channel.messages.batch_disable_select_required'));
      return;
    }
    const targets = collectSelectedTargets();
    if (targets.length === 0) {
      showInfo(t('channel.messages.batch_disable_select_required'));
      return;
    }

    setSelectionMode(selectionModeNone);
    setSelectedChannelIds([]);
    setBatchDisabling(true);

    const results = await Promise.allSettled(
      targets.map(async (target) => {
        const res = await API.put('/api/v1/admin/channel/', {
          id: target.id,
          status: 2,
        });
        const { success, message } = res.data || {};
        return {
          id: target.id,
          success: !!success,
          message: message || '',
        };
      })
    );

    const succeededIds = [];
    let firstFailedMessage = '';
    results.forEach((result) => {
      if (result.status === 'fulfilled' && result.value.success) {
        succeededIds.push(result.value.id);
      } else if (!firstFailedMessage) {
        if (result.status === 'fulfilled') {
          firstFailedMessage = result.value.message || 'Disable failed';
        } else {
          firstFailedMessage = result.reason?.message || `${result.reason}`;
        }
      }
    });

    if (succeededIds.length > 0) {
      const succeededSet = new Set(succeededIds);
      setChannels((prev) =>
        prev.map((channel) =>
          succeededSet.has(channel.id) ? { ...channel, status: 2 } : channel
        )
      );
    }

    const failedCount = results.length - succeededIds.length;
    showInfo(
      t('channel.messages.batch_disable_done', {
        success: succeededIds.length,
        failed: failedCount,
      })
    );
    if (firstFailedMessage) {
      showError(firstFailedMessage);
    }
    setLoading(true);
    await loadChannels({ page: activePage, keyword: searchKeyword });
    setBatchDisabling(false);
  };

  return (
    <>
      <div className='router-toolbar router-block-gap-sm'>
        <div className='router-toolbar-start'>
          {selectionMode === selectionModeNone ? (
            <>
              <Button
                className='router-page-button'
                as={Link}
                to='/channel/add'
                disabled={actionBusy}
              >
                {t('channel.buttons.add')}
              </Button>
              <Button
                className='router-page-button'
                color='orange'
                disabled={actionBusy}
                onClick={() => {
                  setSelectionMode(selectionModeDisable);
                  setSelectedChannelIds([]);
                }}
              >
                {t('channel.buttons.disable_channel')}
              </Button>
              <Button
                className='router-page-button'
                negative
                disabled={actionBusy}
                onClick={() => {
                  setSelectionMode(selectionModeDelete);
                  setSelectedChannelIds([]);
                }}
              >
                {t('channel.buttons.delete_channel')}
              </Button>
            </>
          ) : (
            <>
              <Button
                className='router-page-button'
                negative={selectionMode === selectionModeDelete}
                color={
                  selectionMode === selectionModeDisable ? 'orange' : undefined
                }
                loading={batchDeleting || batchDisabling}
                disabled={batchDeleting || batchDisabling}
                onClick={() => {
                  if (selectionMode === selectionModeDisable) {
                    confirmBatchDisable();
                    return;
                  }
                  confirmBatchDelete();
                }}
              >
                {t('channel.buttons.confirm')}
              </Button>
              <Button
                className='router-page-button'
                disabled={batchDeleting || batchDisabling}
                onClick={cancelBatchSelection}
              >
                {t('channel.buttons.cancel')}
              </Button>
            </>
          )}
          <Button
            className='router-page-button'
            onClick={refresh}
            loading={loading}
            disabled={actionBusy}
          >
            {t('channel.buttons.refresh')}
          </Button>
        </div>

        <Form onSubmit={searchChannels} className='router-search-form-md'>
          <Form.Input
            className='router-section-input'
            icon='search'
            iconPosition='left'
            placeholder={t('channel.search')}
            value={searchKeyword}
            loading={searching}
            onChange={handleKeywordChange}
          />
        </Form>
      </div>
      <Table
        basic={'very'}
        compact
        className='router-hover-table router-list-table'
      >
        <Table.Header>
          <Table.Row>
            {inBatchSelectMode && (
              <Table.HeaderCell collapsing textAlign='center'>
                <Form.Checkbox
                  checked={allPagedSelected}
                  onChange={(e, { checked }) => {
                    togglePagedSelection(!!checked);
                  }}
                />
              </Table.HeaderCell>
            )}
            <Table.HeaderCell
              className='router-sortable-header'
              onClick={() => {
                sortChannel('name');
              }}
            >
              {t('channel.table.id')}
            </Table.HeaderCell>
            <Table.HeaderCell
              className='router-sortable-header'
              onClick={() => {
                sortChannel('protocol');
              }}
            >
              {t('channel.table.type')}
            </Table.HeaderCell>
            <Table.HeaderCell
              className='router-sortable-header'
              onClick={() => {
                sortChannel('status');
              }}
            >
              {t('channel.table.status')}
            </Table.HeaderCell>
            <Table.HeaderCell
              className='router-sortable-header'
              onClick={() => {
                sortChannel('capabilities');
              }}
            >
              {t('channel.table.capabilities')}
            </Table.HeaderCell>
            <Table.HeaderCell
              className='router-sortable-header'
              onClick={() => {
                sortChannel('balance');
              }}
            >
              {t('channel.table.balance')}
            </Table.HeaderCell>
            <Table.HeaderCell
              className='router-sortable-header'
              onClick={() => {
                sortChannel('priority');
              }}
            >
              {t('channel.table.priority')}
            </Table.HeaderCell>
            <Table.HeaderCell className='router-table-action-cell'>
              {t('channel.table.actions')}
            </Table.HeaderCell>
          </Table.Row>
        </Table.Header>

        <Table.Body>
          {pagedChannels.map((channel, idx) => {
            if (channel.deleted) return <></>;
            return (
              <Table.Row
                key={channel.id}
                onClick={() => openChannelByStatus(channel)}
                className={
                  inBatchSelectMode ? undefined : 'router-row-clickable'
                }
              >
                {inBatchSelectMode && (
                  <Table.Cell
                    collapsing
                    textAlign='center'
                    onClick={stopRowClick}
                  >
                    <Form.Checkbox
                      checked={selectedChannelIds.includes(channel.id)}
                      onChange={(e, { checked }) => {
                        toggleChannelSelection(channel.id, !!checked);
                      }}
                    />
                  </Table.Cell>
                )}
                <Table.Cell>{renderChannelName(channel, t)}</Table.Cell>
                <Table.Cell>
                  {renderProtocol(channel.protocol, protocolMap)}
                </Table.Cell>
                <Table.Cell>{renderStatus(channel.status, t)}</Table.Cell>
                <Table.Cell>
                  {renderCapabilities(channel.capabilities, t)}
                </Table.Cell>
                <Table.Cell onClick={stopRowClick}>
                  <Popup
                    trigger={
                      <span
                        onClick={() => {
                          updateChannelBalance(
                            channel.id,
                            getChannelDisplayName(channel),
                            idx
                          );
                        }}
                        className='router-row-clickable'
                      >
                        {renderBalance(channel.protocol, channel.balance, t)}
                      </span>
                    }
                    content={t('channel.table.click_to_update')}
                    basic
                  />
                </Table.Cell>
                <Table.Cell onClick={stopRowClick}>
                  <Popup
                    trigger={
                      <Input
                        className='router-inline-input router-inline-input-short'
                        type='number'
                        defaultValue={channel.priority}
                        onBlur={(event) => {
                          manageChannel(
                            channel.id,
                            'priority',
                            idx,
                            event.target.value
                          );
                        }}
                      />
                    }
                    content={t('channel.table.priority_tip')}
                    basic
                  />
                </Table.Cell>
                <Table.Cell
                  className='router-table-action-cell'
                  onClick={stopRowClick}
                >
                  <div className='router-action-group-tight'>
                    <Button
                      className='router-inline-button'
                      onClick={() => {
                        manageChannel(
                          channel.id,
                          channel.status === 1 ? 'disable' : 'enable',
                          idx
                        );
                      }}
                    >
                      {channel.status === 1
                        ? t('channel.buttons.disable')
                        : t('channel.buttons.enable')}
                    </Button>
                    <Button
                      className='router-inline-button'
                      as={Link}
                      to={`/channel/add?copy_from=${channel.id}`}
                    >
                      {t('channel.buttons.copy')}
                    </Button>
                    <Button
                      className='router-inline-button'
                      as={Link}
                      to={'/channel/edit/' + channel.id}
                    >
                      {t('channel.buttons.edit')}
                    </Button>
                  </div>
                </Table.Cell>
              </Table.Row>
            );
          })}
        </Table.Body>

        <Table.Footer>
          <Table.Row>
            <Table.HeaderCell colSpan={footerColSpan}>
              <Pagination
                className='router-page-pagination'
                floated='right'
                activePage={activePage}
                onPageChange={onPaginationChange}
                siblingRange={1}
                totalPages={Math.max(
                  1,
                  Math.ceil(totalChannels / ITEMS_PER_PAGE)
                )}
              />
            </Table.HeaderCell>
          </Table.Row>
        </Table.Footer>
      </Table>
    </>
  );
};

export default ChannelsTable;
