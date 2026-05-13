import React, { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate } from 'react-router-dom';
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
import {
  AppButton,
  AppFilterHeader,
  AppIcon,
  AppInput,
  AppInputNumber,
  AppPagination,
  AppTable,
  AppTooltip,
} from '../router-ui';

const normalizeAsyncTaskStatus = (value) => {
  const normalized = (value || '').toString().trim().toLowerCase();
  switch (normalized) {
    case 'pending':
    case 'running':
    case 'succeeded':
    case 'failed':
    case 'canceled':
      return normalized;
    default:
      return 'pending';
  }
};

async function fetchTaskById(taskId) {
  const normalizedTaskId = (taskId || '').toString().trim();
  if (normalizedTaskId === '') {
    throw new Error('fetch task failed');
  }
  const res = await API.get(`/api/v1/admin/tasks/${normalizedTaskId}`);
  const { success, message, data } = res.data || {};
  if (!success) {
    throw new Error(message || 'fetch task failed');
  }
  return data || null;
}

function renderTimestamp(timestamp) {
  return <>{timestamp2string(timestamp)}</>;
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
const ChannelsTable = () => {
  const { t } = useTranslation();
  const location = useLocation();
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
  const currentPagePath = `${location.pathname}${location.search}${location.hash}`;
  const [balanceRefreshTasks, setBalanceRefreshTasks] = useState({});
  const [protocolMap, setProtocolMap] = useState(() =>
    buildProtocolMap(getChannelProtocolOptions(), t)
  );

  const processChannelData = useCallback((channel) => {
    const next = { ...channel };
    next.id = (next.id || '').toString().trim();
    next.protocol = (next.protocol || '').toString().trim().toLowerCase();
    next.created_time = Number(next.created_time || 0);
    next.updated_at = Number(next.updated_at || 0);
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

  useEffect(() => {
    const taskEntries = Object.entries(balanceRefreshTasks || {});
    if (taskEntries.length === 0) {
      return undefined;
    }
    const hasActiveTasks = taskEntries.some(([, task]) =>
      ['pending', 'running'].includes(normalizeAsyncTaskStatus(task?.status))
    );
    if (!hasActiveTasks) {
      return undefined;
    }
    const timer = window.setInterval(async () => {
      const updates = await Promise.all(
        taskEntries.map(async ([channelId, task]) => {
          const status = normalizeAsyncTaskStatus(task?.status);
          if (!['pending', 'running'].includes(status)) {
            return { channelId, task };
          }
          try {
            const latestTask = await fetchTaskById(task.id);
            return { channelId, task: latestTask || task };
          } catch {
            return { channelId, task };
          }
        })
      );
      const nextTaskMap = {};
      const finishedTasks = [];
      updates.forEach(({ channelId, task }) => {
        const status = normalizeAsyncTaskStatus(task?.status);
        if (['pending', 'running'].includes(status)) {
          nextTaskMap[channelId] = task;
        } else {
          finishedTasks.push({ channelId, task });
        }
      });
      setBalanceRefreshTasks(nextTaskMap);
      if (finishedTasks.length === 0) {
        return;
      }
      const succeeded = finishedTasks.filter(
        ({ task }) => normalizeAsyncTaskStatus(task?.status) === 'succeeded'
      );
      if (succeeded.length > 0) {
        setLoading(true);
        await loadChannels({ page: activePage, keyword: searchKeyword });
      }
      finishedTasks.forEach(({ channelId, task }) => {
        const targetChannel = channels.find((item) => item.id === channelId);
        const channelName = getChannelDisplayName(targetChannel);
        if (normalizeAsyncTaskStatus(task?.status) === 'succeeded') {
          showSuccess(t('channel.messages.balance_update_success', { name: channelName }));
          return;
        }
        showError(
          task?.error_message ||
            t('channel.messages.balance_update_failed', { name: channelName })
        );
      });
    }, 1500);
    return () => window.clearInterval(timer);
  }, [
    activePage,
    balanceRefreshTasks,
    channels,
    loadChannels,
    searchKeyword,
    t,
  ]);

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
          <AppTooltip title={t('channel.table.status_disabled_tip')}>
            <span className='router-text-danger'>
              {t('channel.table.status_disabled')}
            </span>
          </AppTooltip>
        );
      case 3:
        return (
          <AppTooltip title={t('channel.table.status_auto_disabled_tip')}>
            <span className='router-text-warning'>
              {t('channel.table.status_auto_disabled')}
            </span>
          </AppTooltip>
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
    try {
      const res = await API.get(`/api/v1/admin/channel/update_balance/${id}/`);
      const { success, message, data, meta } = res.data || {};
      if (!success) {
        showError(message);
        return;
      }
      const task = data?.task;
      if (!task?.id) {
        showError(t('channel.messages.balance_update_submit_failed'));
        return;
      }
      setBalanceRefreshTasks((prev) => ({
        ...prev,
        [id]: task,
      }));
      showSuccess(
        meta?.reused
          ? t('channel.messages.balance_update_reused', { name })
          : t('channel.messages.balance_update_submitted', { name })
      );
    } catch (error) {
      showError(
        error?.message || t('channel.messages.balance_update_submit_failed')
      );
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
  const visibleChannels = pagedChannels.filter((channel) => !channel.deleted);
  const pagedChannelIds = pagedChannels
    .filter((channel) => !channel.deleted)
    .map((channel) => channel.id);
  const allPagedSelected =
    pagedChannelIds.length > 0 &&
    pagedChannelIds.every((id) => selectedChannelIds.includes(id));
  const inBatchSelectMode = false;
  const actionBusy = loading;

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

  const openChannelByStatus = async (channel) => {
    if (!channel || !channel.id || inBatchSelectMode) {
      return;
    }
    navigate(`/channel/detail/${channel.id}`, {
      state: {
        from: currentPagePath,
      },
    });
  };

  const stopRowClick = (event) => {
    event.stopPropagation();
  };

  const tableRowSelection = undefined;

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
      <AppFilterHeader
        breadcrumbs={[
          { key: 'admin', label: t('header.admin_workspace') },
          { key: 'resource', label: t('header.resource') },
          { key: 'channel', label: t('header.channel'), active: true },
        ]}
        title={t('header.channel')}
        actions={
          <div className='router-list-toolbar-actions'>
            <AppButton
              className='router-page-button'
              color='blue'
              disabled={actionBusy}
              onClick={() => navigate('/channel/add')}
            >
              {t('channel.buttons.add')}
            </AppButton>
            <AppButton
              className='router-page-button'
              onClick={refresh}
              loading={loading}
              disabled={actionBusy}
            >
              {t('channel.buttons.refresh')}
            </AppButton>
          </div>
        }
        query={
          <div className='router-list-toolbar-query'>
            <AppInput
              className='router-section-input'
              icon='search'
              iconPosition='left'
              fluid
              placeholder={t('channel.search')}
              value={searchKeyword}
              loading={searching}
              onChange={handleKeywordChange}
            />
          </div>
        }
      />
      <AppTable
        className='router-hover-table router-list-table'
        pagination={false}
        rowKey={(channel) => channel.id}
        dataSource={visibleChannels}
        rowSelection={tableRowSelection}
        locale={{ emptyText: '-' }}
        onRow={(channel) => ({
          onClick: () => openChannelByStatus(channel),
          className: inBatchSelectMode ? undefined : 'router-row-clickable',
        })}
        columns={[
          {
            title: (
              <span
                className='router-sortable-header'
                onClick={() => {
                  sortChannel('name');
                }}
              >
                {t('channel.table.id')}
              </span>
            ),
            dataIndex: 'name',
            key: 'name',
            render: (_, channel) => renderChannelName(channel, t),
          },
          {
            title: (
              <span
                className='router-sortable-header'
                onClick={() => {
                  sortChannel('protocol');
                }}
              >
                {t('channel.table.type')}
              </span>
            ),
            dataIndex: 'protocol',
            key: 'protocol',
            render: (value) => renderProtocol(value, protocolMap),
          },
          {
            title: (
              <span
                className='router-sortable-header'
                onClick={() => {
                  sortChannel('status');
                }}
              >
                {t('channel.table.status')}
              </span>
            ),
            dataIndex: 'status',
            key: 'status',
            render: (value) => renderStatus(value, t),
          },
          {
            title: (
              <span
                className='router-sortable-header'
                onClick={() => {
                  sortChannel('created_time');
                }}
              >
                {t('channel.table.created_time')}
              </span>
            ),
            dataIndex: 'created_time',
            key: 'created_time',
            render: (value) => (value ? renderTimestamp(value) : '-'),
          },
          {
            title: (
              <span
                className='router-sortable-header'
                onClick={() => {
                  sortChannel('updated_at');
                }}
              >
                {t('channel.table.updated_at')}
              </span>
            ),
            dataIndex: 'updated_at',
            key: 'updated_at',
            render: (value) => (value ? renderTimestamp(value) : '-'),
          },
          {
            title: (
              <span
                className='router-sortable-header'
                onClick={() => {
                  sortChannel('capabilities');
                }}
              >
                {t('channel.table.capabilities')}
              </span>
            ),
            dataIndex: 'capabilities',
            key: 'capabilities',
            render: (value) => renderCapabilities(value, t),
          },
          {
            title: (
              <span
                className='router-sortable-header'
                onClick={() => {
                  sortChannel('balance');
                }}
              >
                {t('channel.table.balance')}
              </span>
            ),
            dataIndex: 'balance',
            key: 'balance',
            render: (_, channel, idx) => (
              <div onClick={stopRowClick}>
                <AppTooltip title={t('channel.table.click_to_update')}>
                  <span
                    onClick={() => {
                      if (balanceRefreshTasks[channel.id]) {
                        return;
                      }
                      updateChannelBalance(
                        channel.id,
                        getChannelDisplayName(channel),
                        idx,
                      );
                    }}
                    className='router-row-clickable'
                  >
                    {balanceRefreshTasks[channel.id] ? (
                      <>
                        <AppIcon name='spinner' className='router-spin-icon' />
                        {renderBalance(channel.protocol, channel.balance, t)}
                      </>
                    ) : (
                      renderBalance(channel.protocol, channel.balance, t)
                    )}
                  </span>
                </AppTooltip>
              </div>
            ),
          },
          {
            title: (
              <span
                className='router-sortable-header'
                onClick={() => {
                  sortChannel('priority');
                }}
              >
                {t('channel.table.priority')}
              </span>
            ),
            dataIndex: 'priority',
            key: 'priority',
            render: (value, channel, idx) => (
              <div onClick={stopRowClick}>
                <AppTooltip title={t('channel.table.priority_tip')}>
                  <AppInputNumber
                    className='router-inline-number-input router-inline-input-short'
                    defaultValue={value}
                    onBlur={(event) => {
                      manageChannel(
                        channel.id,
                        'priority',
                        idx,
                        event.target.value,
                      );
                    }}
                  />
                </AppTooltip>
              </div>
            ),
          },
          {
            title: t('channel.table.actions'),
            key: 'actions',
            className: 'router-table-action-cell router-channel-action-cell',
            render: (_, channel, idx) => (
              <div
                className='router-action-group-tight'
                onClick={stopRowClick}
              >
                <AppButton
                  className='router-inline-button'
                  color={channel.status === 1 ? undefined : 'blue'}
                  onClick={() => {
                    manageChannel(
                      channel.id,
                      channel.status === 1 ? 'disable' : 'enable',
                      idx,
                    );
                  }}
                >
                  {channel.status === 1
                    ? t('channel.buttons.disable')
                    : t('channel.buttons.enable')}
                </AppButton>
                <AppButton
                  className='router-inline-button'
                  onClick={() => navigate(`/channel/add?copy_from=${channel.id}`)}
                >
                  {t('channel.buttons.copy')}
                </AppButton>
                <AppButton
                  className='router-inline-button'
                  color='red'
                  onClick={() => {
                    manageChannel(channel.id, 'delete', idx);
                  }}
                >
                  {t('channel.buttons.delete')}
                </AppButton>
              </div>
            ),
          },
        ]}
      />
      <div className='router-pagination-wrap'>
        <AppPagination
          className='router-page-pagination'
          activePage={activePage}
          onPageChange={onPaginationChange}
          siblingRange={1}
        totalPages={Math.max(1, Math.ceil(totalChannels / ITEMS_PER_PAGE))}
      />
      </div>
    </>
  );
};

export default ChannelsTable;
