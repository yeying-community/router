import React from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useSearchParams } from 'react-router-dom';
import { AppDivider, AppFilterHeader, AppSection } from '../../router-ui';
import SystemSetting from '../../components/SystemSetting';
import { isRoot } from '../../helpers';
import OtherSetting from '../../components/OtherSetting';
import PersonalSetting from '../../components/PersonalSetting';
import OperationSetting from '../../components/OperationSetting';
import ExchangeRateSetting from '../../components/ExchangeRateSetting';
import CurrencySetting from '../../components/CurrencySetting';

const resolveAdminSettingLocation = (rawTab, rawSection) => {
  if (rawTab === 'currency') {
    return { tab: 'basic', section: 'general' };
  }
  if (rawTab === 'exchange') {
    return { tab: 'basic', section: 'general' };
  }
  if (
    rawTab === 'system' ||
    rawTab === 'general' ||
    rawTab === 'smtp' ||
    rawTab === 'login'
  ) {
    return { tab: 'basic', section: 'general' };
  }
  if (rawTab === 'operation') {
    if (rawSection === 'monitor' || rawSection === 'retry' || rawSection === 'log') {
      return { tab: 'runtime', section: rawSection };
    }
    return { tab: 'billing', section: 'balance' };
  }
  if (
    rawTab === 'monitor' ||
    rawTab === 'retry' ||
    rawTab === 'log_setting'
  ) {
    return { tab: 'runtime', section: 'monitor' };
  }
  if (rawTab === 'other' || rawTab === 'notice') {
    return { tab: 'content', section: 'notice' };
  }
  return {
    tab: rawTab,
    section: rawSection,
  };
};

const Setting = () => {
  const { t } = useTranslation();
  const location = useLocation();
  const [searchParams] = useSearchParams();
  const isAdminWorkspace = location.pathname.startsWith('/admin/');

  if (!isAdminWorkspace) {
    return (
      <div className='dashboard-container'>
        <AppFilterHeader
          breadcrumbs={[
            { key: 'mine', label: t('header.mine') },
            { key: 'account', label: t('header.account'), active: true },
          ]}
          title={t('header.account')}
        />
        <PersonalSetting />
      </div>
    );
  }

  const menuGroups = [];

  if (isRoot()) {
    menuGroups.push({
      key: 'basic',
      label: t('setting.groups.basic'),
    });
    menuGroups.push({
      key: 'billing',
      label: t('setting.groups.billing'),
    });
    menuGroups.push({
      key: 'content',
      label: t('setting.groups.content'),
    });
    menuGroups.push({
      key: 'runtime',
      label: t('setting.groups.runtime'),
    });
  }

  const rawRequestedTab = (searchParams.get('tab') || '').trim().toLowerCase();
  const rawRequestedSection = (searchParams.get('section') || '')
    .trim()
    .toLowerCase();
  const { tab: requestedTab } = resolveAdminSettingLocation(
    rawRequestedTab,
    rawRequestedSection,
  );
  const tabKeys = menuGroups.map((item) => item.key);
  const activeTab =
    tabKeys.includes(requestedTab) && requestedTab !== ''
      ? requestedTab
      : tabKeys[0] || '';
  const activeGroup = menuGroups.find((item) => item.key === activeTab);
  const pageTitle = activeGroup?.label || t('setting.title');

  const renderContent = () => {
    if (activeTab === 'basic') {
      return (
        <div className='router-settings-page-content'>
          <SystemSetting section='all' />
          <AppDivider className='router-settings-page-divider' />
          <CurrencySetting section='catalog' />
          <AppDivider className='router-settings-page-divider' />
          <ExchangeRateSetting section='rates' />
        </div>
      );
    }
    if (activeTab === 'billing') {
      return (
        <div className='router-settings-page-content'>
          <OperationSetting section='billing' />
        </div>
      );
    }
    if (activeTab === 'content') {
      return (
        <div className='router-settings-page-content'>
          <OtherSetting section='all' />
        </div>
      );
    }
    if (activeTab === 'runtime') {
      return (
        <div className='router-settings-page-content'>
          <OperationSetting section='runtime' />
        </div>
      );
    }
    return <div className='router-empty-cell'>{t('setting.empty_admin', '暂无可配置项')}</div>;
  };

  return (
    <div className='dashboard-container'>
      <AppFilterHeader
        breadcrumbs={[
          { key: 'workspace', label: t('header.admin_workspace') },
          { key: 'section', label: t('header.setting') },
          ...(activeGroup
            ? [
                {
                  key: 'group',
                  label: activeGroup.label,
                  active: true,
                },
              ]
            : []),
        ]}
        title={pageTitle}
      />
      <AppSection className='router-settings-page-section'>
        {menuGroups.length > 0 ? (
          renderContent()
        ) : (
          <div className='router-empty-cell'>
            {t('setting.empty_admin', '暂无可配置项')}
          </div>
        )}
      </AppSection>
    </div>
  );
};

export default Setting;
