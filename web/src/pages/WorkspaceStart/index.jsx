import React from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'react-router-dom';
import { AppButton, AppFilterHeader, AppSection } from '../../router-ui';

const WorkspaceStart = () => {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const chatLink = String(localStorage.getItem('chat_link') || '').trim();

  return (
    <div className='dashboard-container router-workspace-start-page'>
      <AppFilterHeader
        breadcrumbs={[
          { key: 'workspace', label: t('header.user_workspace') },
          { key: 'service', label: t('header.service') },
          { key: 'start', label: t('workspace_start.title'), active: true },
        ]}
        title={t('workspace_start.title')}
      />

      <div className='router-workspace-start-grid'>
        <AppSection className='router-workspace-start-card'>
          <div className='router-workspace-start-card-index'>1</div>
          <div className='router-workspace-start-card-title'>
            {t('workspace_start.steps.pricing.title')}
          </div>
          <div className='router-workspace-start-card-body'>
            {t('workspace_start.steps.pricing.description')}
          </div>
          <AppButton
            type='button'
            className='router-inline-button'
            onClick={() => navigate('/workspace/service/pricing')}
          >
            {t('workspace_start.actions.view_pricing')}
          </AppButton>
        </AppSection>

        <AppSection className='router-workspace-start-card'>
          <div className='router-workspace-start-card-index'>2</div>
          <div className='router-workspace-start-card-title'>
            {t('workspace_start.steps.token.title')}
          </div>
          <div className='router-workspace-start-card-body'>
            {t('workspace_start.steps.token.description')}
          </div>
          <AppButton
            type='button'
            className='router-inline-button'
            onClick={() => navigate('/workspace/token/add')}
          >
            {t('workspace_start.actions.create_token')}
          </AppButton>
        </AppSection>

        <AppSection className='router-workspace-start-card'>
          <div className='router-workspace-start-card-index'>3</div>
          <div className='router-workspace-start-card-title'>
            {t('workspace_start.steps.call.title')}
          </div>
          <div className='router-workspace-start-card-body'>
            {t('workspace_start.steps.call.description')}
          </div>
          <div className='router-workspace-start-option-list'>
            {chatLink !== '' ? (
              <div className='router-workspace-start-option-item'>
                <div className='router-workspace-start-option-title'>
                  {t('workspace_start.steps.call.chat.title')}
                </div>
                <div className='router-workspace-start-option-description'>
                  {t('workspace_start.steps.call.chat.description')}
                </div>
                <AppButton
                  type='button'
                  className='router-inline-button'
                  onClick={() => window.open(chatLink, '_blank', 'noopener,noreferrer')}
                >
                  {t('workspace_start.actions.open_chat')}
                </AppButton>
              </div>
            ) : null}

            <div className='router-workspace-start-option-item'>
              <div className='router-workspace-start-option-title'>
                {t('workspace_start.steps.call.terminal.title')}
              </div>
              <div className='router-workspace-start-option-description'>
                {t('workspace_start.steps.call.terminal.description')}
              </div>
              <AppButton
                type='button'
                className='router-inline-button'
                onClick={() => navigate('/workspace/service/help')}
              >
                {t('workspace_start.actions.view_guide')}
              </AppButton>
            </div>
          </div>
        </AppSection>
      </div>
    </div>
  );
};

export default WorkspaceStart;
