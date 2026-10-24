import React, { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate, useParams } from 'react-router-dom';
import Loading from '../../components/Loading';
import { API, showError } from '../../helpers';

const resolveOperationTarget = (operation) => {
  const channelId = (operation?.channel_id || '').toString().trim();
  if (channelId === '') {
    return '/admin/channel';
  }
  const base = `/admin/channel/detail/${encodeURIComponent(channelId)}`;
  switch ((operation?.type || '').toString().trim()) {
    case 'channel_model_test': {
      const query = new URLSearchParams({ tab: 'tests' });
      const model = (operation?.model || '').toString().trim();
      const endpoint = (operation?.endpoint || '').toString().trim();
      if (model !== '') query.set('model', model);
      if (endpoint !== '') query.set('endpoint', endpoint);
      return `${base}?${query.toString()}`;
    }
    case 'channel_refresh_models':
      return `${base}?tab=models`;
    case 'channel_refresh_billing':
    case 'channel_sync_provider_usage':
      return `${base}?tab=procurement`;
    default:
      return base;
  }
};

function AdminChannelTaskDetailPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { id } = useParams();

  useEffect(() => {
    let active = true;
    const redirect = async () => {
      try {
        const response = await API.get(`/api/v1/admin/tasks/${id}`);
        const { success, message, data } = response.data || {};
        if (!success) {
          throw new Error(message || t('task.messages.operation_load_failed'));
        }
        if (active) {
          navigate(resolveOperationTarget(data), { replace: true });
        }
      } catch (error) {
        showError(error?.message || t('task.messages.operation_load_failed'));
        if (active) {
          navigate('/admin/channel', { replace: true });
        }
      }
    };
    redirect();
    return () => {
      active = false;
    };
  }, [id, navigate, t]);

  return <Loading />;
}

export default AdminChannelTaskDetailPage;
