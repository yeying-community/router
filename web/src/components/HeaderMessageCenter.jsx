import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { marked } from 'marked';
import { useTranslation } from 'react-i18next';
import { useLocation, useNavigate } from 'react-router-dom';
import { API, timestamp2string } from '../helpers';
import { AppButton, AppIcon, AppPopover } from '../router-ui';

const READ_MESSAGE_IDS_STORAGE_KEY = 'router.header.read_message_ids';
const WORKSPACE_ADMIN = 'admin';
const WORKSPACE_USER = 'user';
const MESSAGE_KIND_PLATFORM = 'platform';

const readStoredMessageIDs = () => {
  try {
    const raw = localStorage.getItem(READ_MESSAGE_IDS_STORAGE_KEY);
    if (!raw) {
      return [];
    }
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed)
      ? parsed.map((item) => (item || '').toString().trim()).filter(Boolean)
      : [];
  } catch {
    return [];
  }
};

const writeStoredMessageIDs = (ids) => {
  try {
    localStorage.setItem(READ_MESSAGE_IDS_STORAGE_KEY, JSON.stringify(ids));
  } catch {
    // ignore storage errors
  }
};

const hashContent = (value) => {
  const text = (value || '').toString();
  let hash = 0;
  for (let i = 0; i < text.length; i += 1) {
    hash = (hash * 31 + text.charCodeAt(i)) >>> 0;
  }
  return hash.toString(16);
};

const stripRichText = (value) =>
  (value || '')
    .toString()
    .replace(/<[^>]*>/g, ' ')
    .replace(/[#>*_`~\[\]\(\)-]/g, ' ')
    .replace(/\s+/g, ' ')
    .trim();

const normalizeTaskStatus = (value) => {
  const normalized = (value || '').toString().trim().toLowerCase();
  switch (normalized) {
    case 'pending':
    case 'queued':
      return 'pending';
    case 'running':
    case 'processing':
    case 'in_progress':
      return 'running';
    case 'succeeded':
    case 'success':
    case 'completed':
      return 'succeeded';
    case 'failed':
    case 'error':
    case 'unsupported':
      return 'failed';
    case 'canceled':
    case 'cancelled':
      return 'canceled';
    default:
      return normalized || 'pending';
  }
};

const resolveWorkspace = (pathname = '') =>
  pathname.startsWith('/admin/') ? WORKSPACE_ADMIN : WORKSPACE_USER;

const buildNoticeMessages = (notice, t) => {
  const content = (notice || '').toString().trim();
  if (content === '') {
    return [];
  }
  return [
    {
      id: `notice:${hashContent(content)}`,
      kind: MESSAGE_KIND_PLATFORM,
      source: 'notice',
      workspace: 'both',
      level: 'info',
      title: t('header.messages.notice_title'),
      summary: stripRichText(content),
      html: marked.parse(content),
      href: '',
      createdAt: 0,
      createdAtLabel: t('header.messages.notice_source'),
    },
  ];
};

const buildUserTaskMessages = (items, t) =>
  (Array.isArray(items) ? items : [])
    .map((item) => {
      const status = normalizeTaskStatus(item?.status);
      const taskId = (item?.task_id || item?.id || '').toString().trim();
      if (taskId === '' || (status !== 'failed' && status !== 'running')) {
        return null;
      }
      const model = (item?.model || '').toString().trim();
      const provider = (item?.provider || '').toString().trim();
      const resultUrl = (item?.result_url || '').toString().trim();
      const summaryParts = [
        model,
        provider,
        status === 'failed'
          ? t('header.messages.user_task_failed_summary')
          : t('header.messages.user_task_running_summary'),
      ].filter(Boolean);
      return {
        id: `user-task:${taskId}:${status}:${resultUrl}`,
        kind: MESSAGE_KIND_PLATFORM,
        source: 'task',
        workspace: WORKSPACE_USER,
        level: status === 'failed' ? 'error' : 'info',
        title:
          status === 'failed'
            ? t('header.messages.user_task_failed_title')
            : t('header.messages.user_task_running_title'),
        summary: summaryParts.join(' · '),
        html: '',
        href: `/workspace/task/${taskId}`,
        createdAt: Number(item?.created_at || 0),
        createdAtLabel:
          Number(item?.created_at || 0) > 0
            ? timestamp2string(Number(item.created_at))
            : t('header.messages.task_source_user'),
      };
    })
    .filter(Boolean);

function HeaderMessageCenter() {
  const { t } = useTranslation();
  const location = useLocation();
  const navigate = useNavigate();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [messages, setMessages] = useState([]);
  const [readMessageIDs, setReadMessageIDs] = useState(() => readStoredMessageIDs());
  const currentWorkspace = useMemo(
    () => resolveWorkspace(location.pathname),
    [location.pathname],
  );

  const unreadCount = useMemo(
    () =>
      messages.filter((message) => !readMessageIDs.includes(message.id)).length,
    [messages, readMessageIDs],
  );

  const markMessagesAsRead = useCallback((ids) => {
    const normalizedIDs = Array.from(
      new Set(
        (Array.isArray(ids) ? ids : [])
          .map((item) => (item || '').toString().trim())
          .filter(Boolean),
      ),
    );
    if (normalizedIDs.length === 0) {
      return;
    }
    setReadMessageIDs((previous) => {
      const next = Array.from(new Set([...previous, ...normalizedIDs]));
      writeStoredMessageIDs(next);
      return next;
    });
  }, []);

  const markAllAsRead = useCallback(() => {
    markMessagesAsRead(messages.map((message) => message.id));
  }, [markMessagesAsRead, messages]);

  const loadMessages = useCallback(async () => {
    setLoading(true);
    try {
      const requests = [API.get('/api/v1/public/notice')];
      if (currentWorkspace !== WORKSPACE_ADMIN) {
        requests.push(
          API.get('/api/v1/public/user/tasks', {
            params: {
              page: 1,
              page_size: 5,
              status: 'running,failed',
            },
          }),
        );
      }
      const responses = await Promise.all(requests);
      const [noticeResponse, taskResponse] = responses;
      const notice =
        noticeResponse?.data?.success === true
          ? noticeResponse?.data?.data || ''
          : '';
      const noticeMessages = buildNoticeMessages(notice, t);
      const taskMessages =
        currentWorkspace === WORKSPACE_ADMIN
          ? []
          : buildUserTaskMessages(taskResponse?.data?.success === true ? taskResponse?.data?.data?.items || [] : [], t);
      const nextMessages = [
        ...taskMessages,
        ...noticeMessages,
      ].sort(
        (left, right) => Number(right.createdAt || 0) - Number(left.createdAt || 0),
      );
      setMessages(nextMessages);
    } catch {
      setMessages([]);
    } finally {
      setLoading(false);
    }
  }, [currentWorkspace, t]);

  useEffect(() => {
    loadMessages().then();
  }, [loadMessages]);

  useEffect(() => {
    if (!open || messages.length === 0) {
      return;
    }
    markMessagesAsRead(messages.map((message) => message.id));
  }, [markMessagesAsRead, messages, open]);

  const handleMessageClick = useCallback(
    (message) => {
      if (!message?.href) {
        return;
      }
      markMessagesAsRead([message.id]);
      setOpen(false);
      navigate(message.href);
    },
    [markMessagesAsRead, navigate],
  );

  const renderMessageContent = useCallback((message) => {
    if (!message?.html) {
      return null;
    }
    return (
      <div
        className='router-header-message-item-content'
        dangerouslySetInnerHTML={{ __html: message.html }}
      />
    );
  }, []);

  const renderPlatformMessage = useCallback(
    (message, unread) => {
      const className = [
        'router-header-message-item',
        'kind-platform',
        unread ? 'unread' : '',
        message?.level ? `level-${message.level}` : '',
      ]
        .filter(Boolean)
        .join(' ');
      return (
        <div key={message.id} className={className}>
          <div className='router-header-message-item-meta'>
            <div className='router-header-message-item-meta-main'>
              <div className='router-header-message-item-kicker'>
                {t('header.messages.filter_platform')}
              </div>
              <div className='router-header-message-item-title'>
                {message.title}
              </div>
            </div>
            <div className='router-header-message-item-source'>
              {message.createdAtLabel}
            </div>
          </div>
          <div className='router-header-message-item-summary'>
            {message.summary || '-'}
          </div>
          {renderMessageContent(message)}
          {message?.href ? (
            <div className='router-header-message-item-actions'>
              <AppButton
                type='button'
                size='small'
                onClick={() => handleMessageClick(message)}
              >
                {t('header.messages.view_details')}
              </AppButton>
            </div>
          ) : null}
        </div>
      );
    },
    [handleMessageClick, renderMessageContent, t],
  );

  return (
    <AppPopover
      trigger='click'
      placement='bottomRight'
      open={open}
      onOpenChange={setOpen}
      content={
        <div className='router-header-message-center'>
          <div className='router-header-message-center-header'>
            <div className='router-header-message-center-heading'>
              <div className='router-header-message-center-title'>
                {t('header.messages.title')}
              </div>
              <div className='router-header-message-center-subtitle'>
                {t('header.messages.unread_count', { count: unreadCount })}
              </div>
            </div>
            <AppButton
              type='button'
              className='router-inline-button'
              disabled={messages.length === 0 || unreadCount === 0}
              onClick={markAllAsRead}
            >
              {t('header.messages.mark_all_read')}
            </AppButton>
          </div>
          <div className='router-header-message-center-body'>
            {loading ? (
              <div className='router-header-message-empty'>
                {t('common.loading')}
              </div>
            ) : messages.length === 0 ? (
              <div className='router-header-message-empty'>
                {t('header.messages.empty')}
              </div>
            ) : (
              messages.map((message) => {
                const unread = !readMessageIDs.includes(message.id);
                return renderPlatformMessage(message, unread);
              })
            )}
          </div>
        </div>
      }
    >
      <button
        type='button'
        className='router-header-toolbar-icon router-header-message-trigger'
        aria-label={t('header.messages.title')}
        title={t('header.messages.title')}
      >
        <AppIcon name='comments' className='router-header-trigger-icon' />
        {unreadCount > 0 ? (
          <span className='router-header-message-badge'>
            {unreadCount > 99 ? '99+' : unreadCount}
          </span>
        ) : null}
      </button>
    </AppPopover>
  );
}

export default HeaderMessageCenter;
