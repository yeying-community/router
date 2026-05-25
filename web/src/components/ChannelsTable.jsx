import React, { useCallback, useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate } from 'react-router-dom';
import {
  API,
  showError,
  showInfo,
  timestamp2string,
} from '../helpers';

import { ITEMS_PER_PAGE } from '../constants';
import {
  CHANNEL_LIST_COLUMN_WIDTHS,
  CHANNEL_LIST_TABLE_MIN_WIDTH,
} from '../constants/tableWidthPresets';
import {
  getChannelProtocolOptions,
  loadChannelProtocolOptions,
} from '../helpers/helper';
import {
  AppButton,
  AppFilterHeader,
  AppInput,
  AppInputNumber,
  AppFormActions,
  AppModal,
  AppPagination,
  AppTable,
  AppTooltip,
} from '../router-ui';

const compareTextValue = (left, right) =>
  String(left || '').localeCompare(String(right || ''));

const compareNumberValue = (left, right) =>
  Number(left || 0) - Number(right || 0);

const compareArrayValue = (left, right) =>
  compareTextValue(
    Array.isArray(left) ? left.join(',') : left,
    Array.isArray(right) ? right.join(',') : right,
  );

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
  const [disableBlockedImpact, setDisableBlockedImpact] = useState(null);
  const currentPagePath = `${location.pathname}${location.search}${location.hash}`;
  const [tableSorter, setTableSorter] = useState({
    columnKey: 'created_time',
    order: 'descend',
  });
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
      if (res?.data?.data?.code === 'channel_disable_blocked') {
        setDisableBlockedImpact(res?.data?.data?.impact || null);
      }
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

  const handleKeywordChange = async (e, { value }) => {
    setSearchKeyword(value.trim());
  };

  const handleTableChange = (_, __, sorter) => {
    if (!sorter || Array.isArray(sorter) || !sorter.columnKey || !sorter.order) {
      setTableSorter({ columnKey: null, order: null });
      return;
    }
    setTableSorter({
      columnKey: sorter.columnKey,
      order: sorter.order,
    });
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
    navigate(`/admin/channel/detail/${channel.id}`, {
      state: {
        from: currentPagePath,
        channelLabel: getChannelDisplayName(channel),
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
          errorCode: res?.data?.data?.code || '',
          impact: res?.data?.data?.impact || null,
        };
      })
    );

    const succeededIds = [];
    let firstFailedMessage = '';
    let firstBlockedImpact = null;
    results.forEach((result) => {
      if (result.status === 'fulfilled' && result.value.success) {
        succeededIds.push(result.value.id);
      } else if (!firstFailedMessage) {
        if (result.status === 'fulfilled') {
          firstFailedMessage = result.value.message || 'Disable failed';
          if (result.value.errorCode === 'channel_disable_blocked') {
            firstBlockedImpact = result.value.impact || null;
          }
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
      if (firstBlockedImpact) {
        setDisableBlockedImpact(firstBlockedImpact);
      }
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
              onClick={() => navigate('/admin/channel/add')}
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
      <div className='router-table-scroll-x'>
        <AppTable
          className='router-hover-table router-list-table router-table-fit-page'
          pagination={false}
          scroll={{ x: CHANNEL_LIST_TABLE_MIN_WIDTH }}
          rowKey={(channel) => channel.id}
          onChange={handleTableChange}
          dataSource={visibleChannels}
          rowSelection={tableRowSelection}
          locale={{ emptyText: '-' }}
          onRow={(channel) => ({
            onClick: () => openChannelByStatus(channel),
            className: inBatchSelectMode ? undefined : 'router-row-clickable',
          })}
          columns={[
          {
            title: t('channel.table.id'),
            dataIndex: 'name',
            key: 'name',
            width: CHANNEL_LIST_COLUMN_WIDTHS.name,
            ellipsis: true,
            sorter: (a, b) => compareTextValue(a.name, b.name),
            sortDirections: ['ascend', 'descend'],
            sortOrder: tableSorter.columnKey === 'name' ? tableSorter.order : null,
            render: (_, channel) => renderChannelName(channel, t),
          },
          {
            title: t('channel.table.type'),
            dataIndex: 'protocol',
            key: 'protocol',
            className: 'router-table-col-type-narrow',
            width: CHANNEL_LIST_COLUMN_WIDTHS.type,
            sorter: (a, b) => compareTextValue(a.protocol, b.protocol),
            sortDirections: ['ascend', 'descend'],
            sortOrder:
              tableSorter.columnKey === 'protocol' ? tableSorter.order : null,
            render: (value) => renderProtocol(value, protocolMap),
          },
          {
            title: t('channel.table.status'),
            dataIndex: 'status',
            key: 'status',
            className: 'router-table-col-status-compact',
            width: CHANNEL_LIST_COLUMN_WIDTHS.status,
            sorter: (a, b) => compareNumberValue(a.status, b.status),
            sortDirections: ['ascend', 'descend'],
            sortOrder: tableSorter.columnKey === 'status' ? tableSorter.order : null,
            render: (value) => renderStatus(value, t),
          },
          {
            title: t('channel.table.created_time'),
            dataIndex: 'created_time',
            key: 'created_time',
            className: 'router-table-col-datetime',
            width: CHANNEL_LIST_COLUMN_WIDTHS.createdAt,
            sorter: (a, b) => compareNumberValue(a.created_time, b.created_time),
            sortDirections: ['ascend', 'descend'],
            sortOrder:
              tableSorter.columnKey === 'created_time' ? tableSorter.order : null,
            render: (value) => (value ? renderTimestamp(value) : '-'),
          },
          {
            title: t('channel.table.updated_at'),
            dataIndex: 'updated_at',
            key: 'updated_at',
            className: 'router-table-col-datetime',
            width: CHANNEL_LIST_COLUMN_WIDTHS.updatedAt,
            sorter: (a, b) => compareNumberValue(a.updated_at, b.updated_at),
            sortDirections: ['ascend', 'descend'],
            sortOrder:
              tableSorter.columnKey === 'updated_at' ? tableSorter.order : null,
            render: (value) => (value ? renderTimestamp(value) : '-'),
          },
          {
            title: t('channel.table.capabilities'),
            dataIndex: 'capabilities',
            key: 'capabilities',
            width: CHANNEL_LIST_COLUMN_WIDTHS.capabilities,
            ellipsis: true,
            sorter: (a, b) => compareArrayValue(a.capabilities, b.capabilities),
            sortDirections: ['ascend', 'descend'],
            sortOrder:
              tableSorter.columnKey === 'capabilities' ? tableSorter.order : null,
            render: (value) => renderCapabilities(value, t),
          },
          {
            title: t('channel.table.priority'),
            dataIndex: 'priority',
            key: 'priority',
            className: 'router-table-col-status-compact',
            width: CHANNEL_LIST_COLUMN_WIDTHS.priority,
            sorter: (a, b) => compareNumberValue(a.priority, b.priority),
            sortDirections: ['ascend', 'descend'],
            sortOrder:
              tableSorter.columnKey === 'priority' ? tableSorter.order : null,
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
            className: 'router-table-col-actions-wide',
            width: CHANNEL_LIST_COLUMN_WIDTHS.actions,
            render: (_, channel, idx) => (
              <div
                className='router-action-group-tight router-table-actions-wide'
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
                  onClick={() =>
                    navigate(`/admin/channel/add?copy_from=${channel.id}`)
                  }
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
      </div>
      <AppModal
        size='small'
        open={!!disableBlockedImpact}
        onClose={() => setDisableBlockedImpact(null)}
        title={t('channel.messages.disable_blocked_title')}
        footer={
          <AppFormActions>
            <AppButton type='button' onClick={() => setDisableBlockedImpact(null)}>
              {t('channel.buttons.confirm')}
            </AppButton>
          </AppFormActions>
        }
      >
        <div className='router-block-gap-sm'>
          <div>{t('channel.messages.disable_blocked_description')}</div>
          {disableBlockedImpact?.channel_id ? (
            <div className='router-text-meta'>
              {t('channel.messages.disable_blocked_channel', {
                channel: disableBlockedImpact.channel_id,
              })}
            </div>
          ) : null}
          <div className='router-block-gap-xs'>
            {(Array.isArray(disableBlockedImpact?.groups)
              ? disableBlockedImpact.groups
              : []
            ).map((item, index) => {
              const groupID = (item?.group || '').toString().trim() || '-';
              const models = Array.isArray(item?.models) ? item.models : [];
              return (
                <div key={`${groupID}-${index}`} className='router-text-wrap'>
                  {models.length > 0
                    ? t('channel.messages.disable_blocked_group_with_models', {
                        group: groupID,
                        models: models.join(', '),
                      })
                    : t('channel.messages.disable_blocked_group', {
                        group: groupID,
                      })}
                </div>
              );
            })}
          </div>
        </div>
      </AppModal>
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
