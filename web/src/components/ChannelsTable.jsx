import React, {useCallback, useEffect, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {Button, Dropdown, Form, Icon, Input, Label, Pagination, Popup, Table,} from 'semantic-ui-react';
import {Link, useNavigate} from 'react-router-dom';
import {
  API,
  loadChannelModels,
  showError,
  showInfo,
  showSuccess,
  timestamp2string,
} from '../helpers';

import {ITEMS_PER_PAGE} from '../constants';
import {getChannelOptions, loadChannelOptions} from '../helpers/helper';
import {renderNumber} from '../helpers/render';

function renderTimestamp(timestamp) {
  return <>{timestamp2string(timestamp)}</>;
}

function buildTypeMap(options, t) {
  const typeMap = {};
  if (Array.isArray(options)) {
    options.forEach((option) => {
      if (option && Number.isInteger(option.value)) {
        typeMap[option.value] = option;
      }
    });
  }
  typeMap[0] = {
    value: 0,
    text: t('channel.table.status_unknown'),
    color: 'grey',
  };
  return typeMap;
}

function renderType(type, typeMap) {
  const option = typeMap[type];
  const colorMap = {
    grey: 'rgba(0, 0, 0, 0.5)',
    green: '#1f8f4b',
    red: '#d64545',
    yellow: '#b58105',
    olive: '#7f8b24',
    blue: '#2185d0',
    orange: '#c66900',
  };
  return (
    <span style={{ color: colorMap[option?.color] || 'inherit', fontWeight: 500 }}>
      {option ? option.text : type}
    </span>
  );
}

function renderBalance(type, balance, t) {
  switch (type) {
    case 1: // OpenAI
        if (balance === 0) {
            return <span>{t('channel.table.balance_not_supported')}</span>;
        }
      return <span>${balance.toFixed(2)}</span>;
    case 4: // CloseAI
      return <span>¥{balance.toFixed(2)}</span>;
    case 8: // 自定义
      return <span>${balance.toFixed(2)}</span>;
    case 5: // OpenAI-SB
      return <span>¥{(balance / 10000).toFixed(2)}</span>;
    case 10: // AI Proxy
      return <span>{renderNumber(balance)}</span>;
    case 12: // API2GPT
      return <span>¥{balance.toFixed(2)}</span>;
    case 13: // AIGC2D
      return <span>{renderNumber(balance)}</span>;
    case 20: // OpenRouter
      return <span>${balance.toFixed(2)}</span>;
    case 36: // DeepSeek
      return <span>¥{balance.toFixed(2)}</span>;
    case 44: // SiliconFlow
      return <span>¥{balance.toFixed(2)}</span>;
    default:
      return <span>{t('channel.table.balance_not_supported')}</span>;
  }
}

const selectionModeNone = '';
const selectionModeTest = 'test';
const selectionModeDelete = 'delete';
const selectionModeDisable = 'disable';
const channelStatusCreating = 4;

const ChannelsTable = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const [channels, setChannels] = useState([]);
  const [loading, setLoading] = useState(true);
  const [activePage, setActivePage] = useState(1);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [searching, setSearching] = useState(false);
  const [selectionMode, setSelectionMode] = useState(selectionModeNone);
  const [batchTesting, setBatchTesting] = useState(false);
  const [batchDeleting, setBatchDeleting] = useState(false);
  const [batchDisabling, setBatchDisabling] = useState(false);
  const [selectedChannelIds, setSelectedChannelIds] = useState([]);
  const [typeMap, setTypeMap] = useState(() =>
    buildTypeMap(getChannelOptions(), t)
  );

  const processChannelData = useCallback((channel) => {
    if (channel.models === '') {
      channel.models = [];
      channel.test_model = '';
      channel.model_options = [];
    } else {
      channel.models = channel.models.split(',');
      if (channel.models.length > 0) {
        channel.test_model = channel.models.includes(channel.test_model)
          ? channel.test_model
          : channel.models[0];
      } else {
        channel.test_model = '';
      }
      channel.model_options = channel.models.map((model) => {
        return {
          key: model,
          text: model,
          value: model,
        };
      });
    }
    return channel;
  }, []);

  const loadChannels = useCallback(async (startIdx) => {
    const res = await API.get(`/api/v1/admin/channel/?p=${startIdx}`);
    const { success, message, data } = res.data;
    if (success) {
      let localChannels = data.map(processChannelData);
      if (startIdx === 0) {
        setChannels(localChannels);
      } else {
        setChannels((prev) => {
          let next = [...prev];
          next.splice(
            startIdx * ITEMS_PER_PAGE,
            data.length,
            ...localChannels
          );
          return next;
        });
      }
    } else {
      showError(message);
    }
    setLoading(false);
  }, [processChannelData]);

  const onPaginationChange = (e, { activePage }) => {
    (async () => {
      if (activePage === Math.ceil(channels.length / ITEMS_PER_PAGE) + 1) {
        // In this case we have to load more data and then append them.
        await loadChannels(activePage - 1);
      }
      setActivePage(activePage);
    })();
  };

  const refresh = async () => {
    setLoading(true);
    await loadChannels(activePage - 1);
  };

  useEffect(() => {
    loadChannels(0)
      .then()
      .catch((reason) => {
        showError(reason);
      });
    loadChannelModels().then();
  }, [loadChannels]);

  useEffect(() => {
    let disposed = false;
    setTypeMap(buildTypeMap(getChannelOptions(), t));
    loadChannelOptions().then((options) => {
      if (disposed) {
        return;
      }
      setTypeMap(buildTypeMap(options, t));
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
    let data = { id };
    let res;
    switch (action) {
      case 'delete':
        res = await API.delete(`/api/v1/admin/channel/${id}/`);
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
      let channel = res.data.data;
      let newChannels = [...channels];
      let realIdx = (activePage - 1) * ITEMS_PER_PAGE + idx;
      if (action === 'delete') {
        newChannels[realIdx].deleted = true;
      } else {
        newChannels[realIdx].status = channel.status;
      }
      setChannels(newChannels);
    } else {
      showError(message);
    }
  };

  const renderStatus = (status, t) => {
    const plainStatusText = (text, color) => (
      <span style={{ color, fontWeight: 500 }}>{text}</span>
    );
    switch (status) {
      case 1:
        return plainStatusText(t('channel.table.status_enabled'), '#1f8f4b');
      case 2:
        return (
          <Popup
            trigger={
              <span style={{ color: '#d64545', fontWeight: 500 }}>
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
              <span style={{ color: '#b58105', fontWeight: 500 }}>
                {t('channel.table.status_auto_disabled')}
              </span>
            }
            content={t('channel.table.status_auto_disabled_tip')}
            basic
          />
        );
      case channelStatusCreating:
        return plainStatusText(t('channel.table.status_creating'), '#2185d0');
      default:
        return plainStatusText(t('channel.table.status_unknown'), 'rgba(0, 0, 0, 0.5)');
    }
  };

  const renderResponseTime = (responseTime, t) => {
    let time = responseTime / 1000;
    time = time.toFixed(2) + 's';
    if (responseTime === 0) {
      return <span style={{ color: 'rgba(0, 0, 0, 0.5)' }}>{t('channel.table.not_tested')}</span>;
    } else if (responseTime <= 1000) {
      return <span style={{ color: '#1f8f4b', fontWeight: 500 }}>{time}</span>;
    } else if (responseTime <= 3000) {
      return <span style={{ color: '#7f8b24', fontWeight: 500 }}>{time}</span>;
    } else if (responseTime <= 5000) {
      return <span style={{ color: '#b58105', fontWeight: 500 }}>{time}</span>;
    } else {
      return <span style={{ color: '#d64545', fontWeight: 500 }}>{time}</span>;
    }
  };

  const searchChannels = async () => {
    if (searchKeyword === '') {
      // if keyword is blank, load files instead.
      await loadChannels(0);
      setActivePage(1);
      return;
    }
    setSearching(true);
    const res = await API.get(`/api/v1/admin/channel/search?keyword=${searchKeyword}`);
    const { success, message, data } = res.data;
    if (success) {
      let localChannels = data.map(processChannelData);
      setChannels(localChannels);
      setActivePage(1);
    } else {
      showError(message);
    }
    setSearching(false);
  };

  const switchTestModel = async (idx, model) => {
    let realIdx = (activePage - 1) * ITEMS_PER_PAGE + idx;
    const currentChannel = channels[realIdx];
    if (!currentChannel) {
      return;
    }
    const previousModel = currentChannel.test_model;
    const channelId = currentChannel.id;
    const selectedModel = typeof model === 'string' ? model : '';

    setChannels((prev) => {
      if (!prev[realIdx]) return prev;
      const next = [...prev];
      next[realIdx] = {
        ...next[realIdx],
        test_model: selectedModel,
      };
      return next;
    });

    try {
      const res = await API.put('/api/v1/admin/channel/test_model', {
        id: channelId,
        test_model: selectedModel,
      });
      const { success, message } = res.data;
      if (!success) {
        setChannels((prev) => {
          if (!prev[realIdx]) return prev;
          const next = [...prev];
          next[realIdx] = {
            ...next[realIdx],
            test_model: previousModel,
          };
          return next;
        });
        showError(message || 'Operation failed');
      }
    } catch (error) {
      setChannels((prev) => {
        if (!prev[realIdx]) return prev;
        const next = [...prev];
        next[realIdx] = {
          ...next[realIdx],
          test_model: previousModel,
        };
        return next;
      });
      showError(error?.message || error);
    }
  };

  const runChannelTest = async (channel, absoluteIndex, silent = false) => {
    if (!channel || absoluteIndex < 0) {
      return false;
    }
    setChannels((prev) => {
      if (!prev[absoluteIndex]) return prev;
      const next = [...prev];
      next[absoluteIndex] = {
        ...next[absoluteIndex],
        testing: true,
      };
      return next;
    });

    let success = false;
    let responseTime = 0;
    try {
      const modelName = channel.test_model || '';
      const res = await API.get(`/api/v1/admin/channel/test/${channel.id}?model=${modelName}`);
      const { success: ok, message, time, model } = res.data || {};
      success = !!ok;
      responseTime = Number(time || 0) * 1000;
      if (success) {
        if (!silent) {
          showSuccess(
            t('channel.messages.test_success', {
              name: channel.name,
              model,
              time,
              message,
            })
          );
        }
      } else if (!silent) {
        showError(message || '测试失败');
      }
    } catch (error) {
      if (!silent) {
        showError(error?.message || error);
      }
    } finally {
      setChannels((prev) => {
        if (!prev[absoluteIndex]) return prev;
        const next = [...prev];
        next[absoluteIndex] = {
          ...next[absoluteIndex],
          response_time: responseTime,
          test_time: Date.now() / 1000,
          testing: false,
        };
        return next;
      });
    }
    return success;
  };

  const testChannel = async (channel, idx) => {
    const absoluteIndex = (activePage - 1) * ITEMS_PER_PAGE + idx;
    await runChannelTest(channel, absoluteIndex, false);
  };

  const updateChannelBalance = async (id, name, idx) => {
    const res = await API.get(`/api/v1/admin/channel/update_balance/${id}/`);
    const { success, message, balance } = res.data;
    if (success) {
      let newChannels = [...channels];
      let realIdx = (activePage - 1) * ITEMS_PER_PAGE + idx;
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
      if (!isNaN(a[key])) {
        // If the value is numeric, subtract to sort
        return a[key] - b[key];
      } else {
        // If the value is not numeric, sort as strings
        return ('' + a[key]).localeCompare(b[key]);
      }
    });
    if (sortedChannels[0].id === channels[0].id) {
      sortedChannels.reverse();
    }
    setChannels(sortedChannels);
    setLoading(false);
  };

  const startIndex = (activePage - 1) * ITEMS_PER_PAGE;
  const pagedChannels = channels.slice(startIndex, activePage * ITEMS_PER_PAGE);
  const pagedChannelIds = pagedChannels
    .filter((channel) => !channel.deleted)
    .map((channel) => channel.id);
  const allPagedSelected =
    pagedChannelIds.length > 0 &&
    pagedChannelIds.every((id) => selectedChannelIds.includes(id));
  const inBatchSelectMode = selectionMode !== selectionModeNone;
  const footerColSpan = 8 + (inBatchSelectMode ? 1 : 0);
  const actionBusy = batchTesting || batchDeleting || batchDisabling;

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

  const resolveCreatingStep = (channel) => {
    if (!channel) {
      return 1;
    }
    if (channel.type === 43) {
      return 1;
    }
    const models = Array.isArray(channel.models) ? channel.models : [];
    if (models.length > 0) {
      return 3;
    }
    return 2;
  };

  const openChannelByStatus = (channel) => {
    if (!channel || !channel.id || inBatchSelectMode) {
      return;
    }
    if (channel.status === channelStatusCreating) {
      const step = resolveCreatingStep(channel);
      navigate(
        `/channel/add?draft_id=${encodeURIComponent(channel.id)}&step=${step}`
      );
      return;
    }
    navigate(`/channel/edit/${channel.id}`);
  };

  const collectSelectedTargets = () => {
    return selectedChannelIds
      .map((id) => {
        const absoluteIndex = channels.findIndex((channel) => channel.id === id);
        if (absoluteIndex < 0) return null;
        return {
          id,
          absoluteIndex,
          channel: channels[absoluteIndex],
        };
      })
      .filter(Boolean);
  };

  const confirmBatchTest = async () => {
    if (selectedChannelIds.length === 0) {
      showInfo(t('channel.messages.batch_test_select_required'));
      return;
    }
    const targets = collectSelectedTargets();

    if (targets.length === 0) {
      showInfo(t('channel.messages.batch_test_select_required'));
      return;
    }

    // Exit selection mode immediately after confirm.
    setSelectionMode(selectionModeNone);
    setSelectedChannelIds([]);
    setBatchTesting(true);

    const results = await Promise.allSettled(
      targets.map((target) =>
        runChannelTest(target.channel, target.absoluteIndex, true)
      )
    );
    let successCount = 0;
    results.forEach((result) => {
      if (result.status === 'fulfilled' && result.value) {
        successCount += 1;
      }
    });
    const failedCount = results.length - successCount;
    showInfo(
      t('channel.messages.batch_test_done', {
        success: successCount,
        failed: failedCount,
      })
    );
    setBatchTesting(false);
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
    setBatchDisabling(false);
  };

  return (
    <>
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: '12px',
          flexWrap: 'wrap',
          marginBottom: '12px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
          {selectionMode === selectionModeNone ? (
            <>
              <Button size='tiny' as={Link} to='/channel/add' disabled={actionBusy}>
                {t('channel.buttons.add')}
              </Button>
              <Button
                size='tiny'
                disabled={actionBusy}
                onClick={() => {
                  setSelectionMode(selectionModeTest);
                  setSelectedChannelIds([]);
                }}
              >
                {t('channel.buttons.test_channel')}
              </Button>
              <Button
                size='tiny'
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
                size='tiny'
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
                size='tiny'
                positive={selectionMode === selectionModeTest}
                negative={selectionMode === selectionModeDelete}
                color={selectionMode === selectionModeDisable ? 'orange' : undefined}
                loading={batchTesting || batchDeleting || batchDisabling}
                disabled={batchTesting || batchDeleting || batchDisabling}
                onClick={() => {
                  if (selectionMode === selectionModeTest) {
                    confirmBatchTest();
                    return;
                  }
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
                size='tiny'
                disabled={batchTesting || batchDeleting || batchDisabling}
                onClick={cancelBatchSelection}
              >
                {t('channel.buttons.cancel')}
              </Button>
            </>
          )}
          <Button size='tiny' onClick={refresh} loading={loading} disabled={actionBusy}>
            {t('channel.buttons.refresh')}
          </Button>
        </div>

        <Form onSubmit={searchChannels} style={{ width: '320px', maxWidth: '100%' }}>
          <Form.Input
            icon='search'
            iconPosition='left'
            placeholder={t('channel.search')}
            value={searchKeyword}
            loading={searching}
            onChange={handleKeywordChange}
          />
        </Form>
      </div>
      <Table basic={'very'} compact size='small'>
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
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortChannel('name');
              }}
            >
              {t('channel.table.name')}
            </Table.HeaderCell>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortChannel('type');
              }}
            >
              {t('channel.table.type')}
            </Table.HeaderCell>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortChannel('status');
              }}
            >
              {t('channel.table.status')}
            </Table.HeaderCell>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortChannel('response_time');
              }}
            >
              {t('channel.table.response_time')}
            </Table.HeaderCell>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortChannel('balance');
              }}
            >
              {t('channel.table.balance')}
            </Table.HeaderCell>
            <Table.HeaderCell
              style={{ cursor: 'pointer' }}
              onClick={() => {
                sortChannel('priority');
              }}
            >
              {t('channel.table.priority')}
            </Table.HeaderCell>
            <Table.HeaderCell>
              {t('channel.table.test_model')}
            </Table.HeaderCell>
            <Table.HeaderCell style={{ width: '280px' }}>
              {t('channel.table.actions')}
            </Table.HeaderCell>
          </Table.Row>
        </Table.Header>

        <Table.Body>
          {pagedChannels
            .map((channel, idx) => {
              if (channel.deleted) return <></>;
              return (
                <Table.Row key={channel.id}>
                  {inBatchSelectMode && (
                    <Table.Cell collapsing textAlign='center'>
                      <Form.Checkbox
                        checked={selectedChannelIds.includes(channel.id)}
                        onChange={(e, { checked }) => {
                          toggleChannelSelection(channel.id, !!checked);
                        }}
                      />
                    </Table.Cell>
                  )}
                  <Table.Cell>
                    <span
                      role='button'
                      tabIndex={0}
                      onClick={() => openChannelByStatus(channel)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter' || event.key === ' ') {
                          event.preventDefault();
                          openChannelByStatus(channel);
                        }
                      }}
                      style={{
                        cursor: inBatchSelectMode ? 'default' : 'pointer',
                        color: inBatchSelectMode ? 'inherit' : '#2185d0',
                        textDecoration: inBatchSelectMode ? 'none' : 'underline',
                      }}
                    >
                      {channel.name ? channel.name : t('channel.table.no_name')}
                    </span>
                  </Table.Cell>
                  <Table.Cell>{renderType(channel.type, typeMap)}</Table.Cell>
                  <Table.Cell>{renderStatus(channel.status, t)}</Table.Cell>
                  <Table.Cell>
                    <Popup
                      content={
                        channel.test_time
                          ? renderTimestamp(channel.test_time)
                          : t('channel.table.not_tested')
                      }
                      key={channel.id}
                      trigger={
                        channel.testing ? (
                          <Icon name='spinner' loading />
                        ) : (
                          renderResponseTime(channel.response_time, t)
                        )
                      }
                      basic
                    />
                  </Table.Cell>
                  <Table.Cell>
                    <Popup
                      trigger={
                        <span
                          onClick={() => {
                            updateChannelBalance(channel.id, channel.name, idx);
                          }}
                          style={{ cursor: 'pointer' }}
                        >
                          {renderBalance(channel.type, channel.balance, t)}
                        </span>
                      }
                      content={t('channel.table.click_to_update')}
                      basic
                    />
                  </Table.Cell>
                  <Table.Cell>
                    <Popup
                      trigger={
                        <Input
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
                        >
                          <input style={{ maxWidth: '60px' }} />
                        </Input>
                      }
                      content={t('channel.table.priority_tip')}
                      basic
                    />
                  </Table.Cell>
                  <Table.Cell>
                    <Dropdown
                      placeholder={t('channel.table.select_test_model')}
                      selection
                      options={channel.model_options}
                      value={channel.test_model}
                      onChange={(event, data) => {
                        switchTestModel(idx, data.value);
                      }}
                    />
                  </Table.Cell>
                  <Table.Cell style={{ width: '280px' }}>
                    <div
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        flexWrap: 'wrap',
                        gap: '4px',
                        rowGap: '4px',
                      }}
                    >
                      <Button
                        size={'tiny'}
                        positive
                        onClick={() => {
                          testChannel(channel, idx);
                        }}
                      >
                        {t('channel.buttons.test')}
                      </Button>
                      <Button
                        size={'tiny'}
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
                        size={'tiny'}
                        as={Link}
                        to={`/channel/add?copy_from=${channel.id}`}
                      >
                        {t('channel.buttons.copy')}
                      </Button>
                      <Button
                        size={'tiny'}
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
                floated='right'
                activePage={activePage}
                onPageChange={onPaginationChange}
                size='tiny'
                siblingRange={1}
                totalPages={
                  Math.ceil(channels.length / ITEMS_PER_PAGE) +
                  (channels.length % ITEMS_PER_PAGE === 0 ? 1 : 0)
                }
              />
            </Table.HeaderCell>
          </Table.Row>
        </Table.Footer>
      </Table>
    </>
  );
};

export default ChannelsTable;
