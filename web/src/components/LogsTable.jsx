import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Button,
  Dropdown,
  Form,
  Header,
  Label,
  Pagination,
  Select,
  Table,
} from 'semantic-ui-react';
import {
  API,
  copy,
  isAdmin,
  showError,
  showSuccess,
  showWarning,
  timestamp2string,
} from '../helpers';
import { useTranslation } from 'react-i18next';

import { ITEMS_PER_PAGE } from '../constants';
import { renderColorLabel, renderQuota } from '../helpers/render';
import { Link } from 'react-router-dom';

function renderTimestamp(timestamp, trace_id) {
  return (
    <code
      onClick={async () => {
        if (await copy(trace_id)) {
          showSuccess(`已复制 Trace ID：${trace_id}`);
        } else {
          showWarning(`Trace ID 复制失败：${trace_id}`);
        }
      }}
      className='router-row-clickable'
    >
      {timestamp2string(timestamp)}
    </code>
  );
}

function renderType(type) {
  switch (type) {
    case 1:
      return (
        <Label basic color='green' className='router-tag'>
          充值
        </Label>
      );
    case 2:
      return (
        <Label basic color='olive' className='router-tag'>
          消费
        </Label>
      );
    case 3:
      return (
        <Label basic color='orange' className='router-tag'>
          管理
        </Label>
      );
    case 4:
      return (
        <Label basic color='purple' className='router-tag'>
          系统
        </Label>
      );
    case 5:
      return (
        <Label basic color='violet' className='router-tag'>
          测试
        </Label>
      );
    default:
      return (
        <Label basic color='black' className='router-tag'>
          未知
        </Label>
      );
  }
}

function getColorByElapsedTime(elapsedTime) {
  if (elapsedTime === undefined || 0) return 'black';
  if (elapsedTime < 1000) return 'green';
  if (elapsedTime < 3000) return 'olive';
  if (elapsedTime < 5000) return 'yellow';
  if (elapsedTime < 10000) return 'orange';
  return 'red';
}

function renderDetail(log) {
  return (
    <>
      {log.content}
      <br />
      {log.elapsed_time && (
        <Label
          basic
          className='router-tag'
          color={getColorByElapsedTime(log.elapsed_time)}
        >
          {log.elapsed_time} ms
        </Label>
      )}
      {log.is_stream && (
        <>
          <Label className='router-tag' color='pink'>
            Stream
          </Label>
        </>
      )}
      {log.system_prompt_reset && (
        <>
          <Label basic className='router-tag' color='red'>
            System Prompt Reset
          </Label>
        </>
      )}
    </>
  );
}

function getLogChannelLabel(log) {
  if (!log) {
    return '';
  }
  return log.channel_name || log.channel || '';
}

const LogsTable = () => {
  const { t } = useTranslation();
  const [logs, setLogs] = useState([]);
  const [showStat, setShowStat] = useState(false);
  const [loading, setLoading] = useState(true);
  const [activePage, setActivePage] = useState(1);
  const [searchKeyword, setSearchKeyword] = useState('');
  const [logType, setLogType] = useState(0);
  const isAdminUser = isAdmin();
  let now = new Date();
  const [inputs, setInputs] = useState({
    username: '',
    token_name: '',
    model_name: '',
    start_timestamp: timestamp2string(0),
    end_timestamp: timestamp2string(now.getTime() / 1000 + 3600),
    channel: '',
  });
  const {
    username,
    token_name,
    model_name,
    start_timestamp,
    end_timestamp,
    channel,
  } = inputs;

  const [stat, setStat] = useState({
    quota: 0,
    token: 0,
  });
  const [pendingFilterKey, setPendingFilterKey] = useState('token_name');
  const [activeFilterKeys, setActiveFilterKeys] = useState([]);

  const LOG_OPTIONS = [
    { key: '0', text: t('log.type.all'), value: 0 },
    { key: '1', text: t('log.type.topup'), value: 1 },
    { key: '2', text: t('log.type.usage'), value: 2 },
    { key: '3', text: t('log.type.admin'), value: 3 },
    { key: '4', text: t('log.type.system'), value: 4 },
    { key: '5', text: t('log.type.test'), value: 5 },
  ];

  const handleInputChange = (e, { name, value }) => {
    setInputs((inputs) => ({ ...inputs, [name]: value }));
  };

  const conditionalFilterConfig = useMemo(() => {
    const items = [
      {
        key: 'token_name',
        label: t('log.table.token_name'),
        placeholder: t('log.table.token_name_placeholder'),
      },
      {
        key: 'model_name',
        label: t('log.table.model_name'),
        placeholder: t('log.table.model_name_placeholder'),
      },
    ];
    if (isAdminUser) {
      items.push(
        {
          key: 'channel',
          label: t('log.table.channel'),
          placeholder: t('log.table.channel_id_placeholder'),
        },
        {
          key: 'username',
          label: t('log.table.username'),
          placeholder: t('log.table.username_placeholder'),
        }
      );
    }
    return items;
  }, [isAdminUser, t]);

  const conditionalFilterOptions = useMemo(
    () =>
      conditionalFilterConfig.map((item) => ({
        key: item.key,
        text: item.label,
        value: item.key,
      })),
    [conditionalFilterConfig]
  );

  const visibleFilterConfig = useMemo(
    () =>
      conditionalFilterConfig.filter((item) =>
        activeFilterKeys.includes(item.key)
      ),
    [activeFilterKeys, conditionalFilterConfig]
  );

  const addConditionalFilter = useCallback(() => {
    const targetKey = (pendingFilterKey || '').toString().trim();
    if (targetKey === '') {
      return;
    }
    setActiveFilterKeys((prev) => {
      if (prev.includes(targetKey)) {
        return prev;
      }
      return [...prev, targetKey];
    });
  }, [pendingFilterKey]);

  const removeConditionalFilter = useCallback((filterKey) => {
    setActiveFilterKeys((prev) => prev.filter((item) => item !== filterKey));
    setInputs((prev) => ({
      ...prev,
      [filterKey]: '',
    }));
  }, []);

  const getLogSelfStat = async () => {
    let localStartTimestamp = Date.parse(start_timestamp) / 1000;
    let localEndTimestamp = Date.parse(end_timestamp) / 1000;
    const statLogType = logType === 0 ? 2 : logType;
    let res = await API.get(
      `/api/v1/public/log/self/stat?type=${statLogType}&token_name=${token_name}&model_name=${model_name}&start_timestamp=${localStartTimestamp}&end_timestamp=${localEndTimestamp}`
    );
    const { success, message, data } = res.data;
    if (success) {
      setStat(data);
    } else {
      showError(message);
    }
  };

  const getLogStat = async () => {
    let localStartTimestamp = Date.parse(start_timestamp) / 1000;
    let localEndTimestamp = Date.parse(end_timestamp) / 1000;
    const statLogType = logType === 0 ? 2 : logType;
    let res = await API.get(
      `/api/v1/admin/log/stat?type=${statLogType}&username=${username}&token_name=${token_name}&model_name=${model_name}&start_timestamp=${localStartTimestamp}&end_timestamp=${localEndTimestamp}&channel=${channel}`
    );
    const { success, message, data } = res.data;
    if (success) {
      setStat(data);
    } else {
      showError(message);
    }
  };

  const handleEyeClick = async () => {
    if (!showStat) {
      if (isAdminUser) {
        await getLogStat();
      } else {
        await getLogSelfStat();
      }
    }
    setShowStat(!showStat);
  };

  const showUserTokenQuota = () => {
    return logType !== 5;
  };

  const loadLogs = useCallback(
    async (page) => {
      const normalizedPage = Number(page) > 0 ? Number(page) : 1;
      let url = '';
      let localStartTimestamp = Date.parse(start_timestamp) / 1000;
      let localEndTimestamp = Date.parse(end_timestamp) / 1000;
      if (isAdminUser) {
        url = `/api/v1/admin/log/?page=${normalizedPage}&type=${logType}&username=${username}&token_name=${token_name}&model_name=${model_name}&start_timestamp=${localStartTimestamp}&end_timestamp=${localEndTimestamp}&channel=${channel}`;
      } else {
        url = `/api/v1/public/log/self/?page=${normalizedPage}&type=${logType}&token_name=${token_name}&model_name=${model_name}&start_timestamp=${localStartTimestamp}&end_timestamp=${localEndTimestamp}`;
      }
      const res = await API.get(url);
      const { success, message, data } = res.data;
      if (success) {
        if (normalizedPage === 1) {
          setLogs(data);
        } else {
          setLogs((prev) => {
            let next = [...prev];
            next.splice((normalizedPage - 1) * ITEMS_PER_PAGE, data.length, ...data);
            return next;
          });
        }
      } else {
        showError(message);
      }
      setLoading(false);
    },
    [
      isAdminUser,
      logType,
      username,
      token_name,
      model_name,
      start_timestamp,
      end_timestamp,
      channel,
    ]
  );

  const onPaginationChange = (e, { activePage }) => {
    (async () => {
      if (activePage === Math.ceil(logs.length / ITEMS_PER_PAGE) + 1) {
        // In this case we have to load more data and then append them.
        await loadLogs(activePage);
      }
      setActivePage(activePage);
    })();
  };

  const refresh = useCallback(async () => {
    setLoading(true);
    setActivePage(1);
    await loadLogs(1);
  }, [loadLogs]);

  useEffect(() => {
    refresh().then();
  }, [refresh]);

  const sortLog = (key) => {
    if (logs.length === 0) return;
    setLoading(true);
    let sortedLogs = [...logs];
    if (typeof sortedLogs[0][key] === 'string') {
      sortedLogs.sort((a, b) => {
        return ('' + a[key]).localeCompare(b[key]);
      });
    } else {
      sortedLogs.sort((a, b) => {
        if (a[key] === b[key]) return 0;
        if (a[key] > b[key]) return -1;
        if (a[key] < b[key]) return 1;
        return 0;
      });
    }
    if (sortedLogs[0].id === logs[0].id) {
      sortedLogs.reverse();
    }
    setLogs(sortedLogs);
    setLoading(false);
  };

  return (
    <>
      <Header as='h3' className='router-section-title'>
        {t('log.usage_details')}（{t('log.total_quota')}：
        {showStat && renderQuota(stat.quota, t)}
        {!showStat && (
          <span
            onClick={handleEyeClick}
            className='router-row-clickable router-text-muted'
          >
            {t('log.click_to_view')}
          </span>
        )}
        ）
      </Header>
      <Form>
        <div className='router-toolbar router-block-gap-sm'>
          <div className='router-toolbar-start'>
            <Form.Input
              className='router-section-input'
              fluid
              label={t('log.table.start_time')}
              width={4}
              value={start_timestamp}
              type='datetime-local'
              name='start_timestamp'
              onChange={handleInputChange}
            />
            <Form.Input
              className='router-section-input'
              fluid
              label={t('log.table.end_time')}
              width={4}
              value={end_timestamp}
              type='datetime-local'
              name='end_timestamp'
              onChange={handleInputChange}
            />
            <Form.Field width={3}>
              <label>{t('log.filters.add')}</label>
              <Dropdown
                selection
                className='router-section-dropdown'
                options={conditionalFilterOptions}
                value={pendingFilterKey}
                onChange={(e, { value }) => setPendingFilterKey(value)}
              />
            </Form.Field>
            <Form.Field width={2}>
              <label>{t('log.buttons.query')}</label>
              <Button
                type='button'
                className='router-section-button'
                onClick={addConditionalFilter}
              >
                {t('log.filters.add')}
              </Button>
            </Form.Field>
            <Form.Field width={2}>
              <label>{t('log.buttons.query')}</label>
              <Button
                type='button'
                className='router-section-button'
                onClick={refresh}
              >
                {t('log.buttons.submit')}
              </Button>
            </Form.Field>
          </div>
          <div className='router-toolbar-end'>
            <Form.Input
              className='router-section-input'
              icon='search'
              placeholder={t('log.search')}
              value={searchKeyword}
              onChange={(e, { value }) => setSearchKeyword(value)}
            />
          </div>
        </div>
        {visibleFilterConfig.length > 0 && (
          <div className='router-toolbar router-block-gap-sm'>
            <div className='router-toolbar-start'>
              {visibleFilterConfig.map((item) => (
                <Form.Input
                  key={item.key}
                  className='router-section-input'
                  fluid
                  label={
                    <span>
                      {item.label}
                      <button
                        type='button'
                        onClick={() => removeConditionalFilter(item.key)}
                        style={{
                          marginLeft: 8,
                          border: 'none',
                          background: 'transparent',
                          cursor: 'pointer',
                          padding: 0,
                          color: 'inherit',
                        }}
                      >
                        ×
                      </button>
                    </span>
                  }
                  width={3}
                  value={inputs[item.key] || ''}
                  placeholder={item.placeholder}
                  name={item.key}
                  onChange={handleInputChange}
                />
              ))}
            </div>
          </div>
        )}
        {visibleFilterConfig.length === 0 && (
          <div className='router-toolbar-meta'>{t('log.filters.none')}</div>
        )}
      </Form>
      <Table basic={'very'} compact className='router-list-table'>
        <Table.Header>
          <Table.Row>
            <Table.HeaderCell
              className='router-sortable-header'
              onClick={() => {
                sortLog('created_time');
              }}
              width={3}
            >
              {t('log.table.time')}
            </Table.HeaderCell>
            {isAdminUser && (
              <Table.HeaderCell
                className='router-sortable-header'
                onClick={() => {
                  sortLog('channel');
                }}
                width={1}
              >
                {t('log.table.channel')}
              </Table.HeaderCell>
            )}
            <Table.HeaderCell
              className='router-sortable-header'
              onClick={() => {
                sortLog('type');
              }}
              width={1}
            >
              {t('log.table.type')}
            </Table.HeaderCell>
            <Table.HeaderCell
              className='router-sortable-header'
              onClick={() => {
                sortLog('model_name');
              }}
              width={2}
            >
              {t('log.table.model')}
            </Table.HeaderCell>
            {showUserTokenQuota() && (
              <>
                {isAdminUser && (
                  <Table.HeaderCell
                    className='router-sortable-header'
                    onClick={() => {
                      sortLog('username');
                    }}
                    width={2}
                  >
                    {t('log.table.username')}
                  </Table.HeaderCell>
                )}
                <Table.HeaderCell
                  className='router-sortable-header'
                  onClick={() => {
                    sortLog('token_name');
                  }}
                  width={2}
                >
                  {t('log.table.token_name')}
                </Table.HeaderCell>
                <Table.HeaderCell
                  className='router-sortable-header'
                  onClick={() => {
                    sortLog('prompt_tokens');
                  }}
                  width={1}
                >
                  {t('log.table.prompt_tokens')}
                </Table.HeaderCell>
                <Table.HeaderCell
                  className='router-sortable-header'
                  onClick={() => {
                    sortLog('completion_tokens');
                  }}
                  width={1}
                >
                  {t('log.table.completion_tokens')}
                </Table.HeaderCell>
                <Table.HeaderCell
                  className='router-sortable-header'
                  onClick={() => {
                    sortLog('quota');
                  }}
                  width={1}
                >
                  {t('log.table.quota')}
                </Table.HeaderCell>
              </>
            )}
            <Table.HeaderCell>{t('log.table.detail')}</Table.HeaderCell>
          </Table.Row>
        </Table.Header>

        <Table.Body>
          {logs
            .slice(
              (activePage - 1) * ITEMS_PER_PAGE,
              activePage * ITEMS_PER_PAGE
            )
            .map((log, idx) => {
              if (log.deleted) return <></>;
              return (
                <Table.Row key={log.id}>
                  <Table.Cell>
                    {renderTimestamp(log.created_at, log.trace_id)}
                  </Table.Cell>
                  {isAdminUser && (
                    <Table.Cell>
                      {log.channel ? (
                        <Label
                          basic
                          className='router-tag'
                          as={Link}
                          to={`/channel/edit/${log.channel}`}
                        >
                          {getLogChannelLabel(log)}
                        </Label>
                      ) : (
                        ''
                      )}
                    </Table.Cell>
                  )}
                  <Table.Cell>{renderType(log.type)}</Table.Cell>
                  <Table.Cell>
                    {log.model_name ? renderColorLabel(log.model_name) : ''}
                  </Table.Cell>
                  {showUserTokenQuota() && (
                    <>
                      {isAdminUser && (
                        <Table.Cell>
                          {log.username ? (
                            <Label
                              basic
                              className='router-tag'
                              as={Link}
                              to={`/user/edit/${log.user_id}`}
                            >
                              {log.username}
                            </Label>
                          ) : (
                            ''
                          )}
                        </Table.Cell>
                      )}
                      <Table.Cell>
                        {log.token_name ? renderColorLabel(log.token_name) : ''}
                      </Table.Cell>

                      <Table.Cell>
                        {log.prompt_tokens ? log.prompt_tokens : ''}
                      </Table.Cell>
                      <Table.Cell>
                        {log.completion_tokens ? log.completion_tokens : ''}
                      </Table.Cell>
                      <Table.Cell>
                        {log.quota ? renderQuota(log.quota, t, 6) : ''}
                      </Table.Cell>
                    </>
                  )}

                  <Table.Cell>{renderDetail(log)}</Table.Cell>
                </Table.Row>
              );
            })}
        </Table.Body>

        <Table.Footer>
          <Table.Row>
            <Table.HeaderCell colSpan={'10'}>
              <div className='router-toolbar'>
                <div className='router-toolbar-start'>
                  <Select
                    className='router-section-dropdown'
                    placeholder={t('log.type.select')}
                    options={LOG_OPTIONS}
                    name='logType'
                    value={logType}
                    onChange={(e, { name, value }) => {
                      setLogType(value);
                    }}
                  />
                  <Button className='router-page-button' onClick={refresh} loading={loading}>
                    {t('log.buttons.refresh')}
                  </Button>
                </div>
                <Pagination
                  className='router-page-pagination'
                  activePage={activePage}
                  onPageChange={onPaginationChange}
                  siblingRange={1}
                  totalPages={
                    Math.ceil(logs.length / ITEMS_PER_PAGE) +
                    (logs.length % ITEMS_PER_PAGE === 0 ? 1 : 0)
                  }
                />
              </div>
            </Table.HeaderCell>
          </Table.Row>
        </Table.Footer>
      </Table>
    </>
  );
};

export default LogsTable;
