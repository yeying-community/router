import React from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useSearchParams } from 'react-router-dom';
import SystemSetting from '../../components/SystemSetting';
import { isRoot } from '../../helpers';
import OtherSetting from '../../components/OtherSetting';
import PersonalSetting from '../../components/PersonalSetting';
import OperationSetting from '../../components/OperationSetting';
import ExchangeRateSetting from '../../components/ExchangeRateSetting';
import CurrencySetting from '../../components/CurrencySetting';
import { AppFilterHeader, AppNavMenu, AppSection } from '../../router-ui';

const Setting = () => {
  const { t } = useTranslation();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
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
      key: 'operation',
      label: t('header.config'),
      sections: [
        { key: 'config', label: t('header.config') },
      ],
    });
    menuGroups.push({
      key: 'currency',
      label: t('setting.tabs.currency'),
      sections: [
        { key: 'catalog', label: t('setting.tabs.currency') },
      ],
    });
    menuGroups.push({
      key: 'exchange',
      label: t('setting.tabs.exchange'),
      sections: [
        { key: 'rates', label: t('setting.tabs.exchange') },
      ],
    });
    menuGroups.push({
      key: 'basic',
      label: t('setting.system.basic.title'),
      sections: [
        { key: 'general', label: t('setting.system.general.title') },
        { key: 'smtp', label: t('setting.system.smtp.title') },
        { key: 'login', label: t('setting.system.login.title') },
      ],
    });
    menuGroups.push({
      key: 'content',
      label: t('setting.system.content.title'),
      sections: [
        { key: 'notice', label: t('setting.system.notice', '站点公告') },
        { key: 'content', label: t('setting.other.content.title') },
      ],
    });
    menuGroups.push({
      key: 'runtime',
      label: t('setting.system.runtime.title'),
      sections: [
        { key: 'monitor', label: t('setting.operation.monitor.title') },
        { key: 'retry', label: t('setting.operation.retry.title') },
        { key: 'log', label: t('setting.operation.log.title') },
      ],
    });
  }

  const tabKeys = menuGroups.map((item) => item.key);
  const rawRequestedTab = (searchParams.get('tab') || '').trim().toLowerCase();
  const rawRequestedSection = (searchParams.get('section') || '')
    .trim()
    .toLowerCase();
  const requestedTab =
    rawRequestedTab === 'system'
      ? rawRequestedSection === 'smtp'
        ? 'basic'
        : rawRequestedSection === 'login'
          ? 'basic'
          : rawRequestedSection === 'monitor'
            ? 'runtime'
            : rawRequestedSection === 'log'
              ? 'runtime'
              : 'basic'
      : rawRequestedTab === 'operation'
        ? rawRequestedSection === 'monitor'
          ? 'runtime'
          : rawRequestedSection === 'log'
            ? 'runtime'
            : rawRequestedSection === 'retry'
              ? 'runtime'
            : 'operation'
        : rawRequestedTab === 'other'
          ? 'content'
          : rawRequestedTab === 'general' ||
              rawRequestedTab === 'smtp' ||
              rawRequestedTab === 'login'
            ? 'basic'
            : rawRequestedTab === 'notice'
              ? 'content'
              : rawRequestedTab === 'monitor' ||
                  rawRequestedTab === 'retry' ||
                  rawRequestedTab === 'log_setting'
                ? 'runtime'
                : rawRequestedTab;
  const visibleMenuGroups =
    tabKeys.includes(requestedTab) && requestedTab !== ''
      ? menuGroups.filter((item) => item.key === requestedTab)
      : menuGroups;
  const visibleTabKeys = visibleMenuGroups.map((item) => item.key);
  const activeTab =
    visibleTabKeys.includes(requestedTab) && requestedTab !== ''
      ? requestedTab
      : visibleTabKeys[0] || '';
  const activeGroup = visibleMenuGroups.find((item) => item.key === activeTab);
  const sectionKeys = (activeGroup?.sections || []).map((item) => item.key);
  const requestedSection =
    activeTab === 'operation' &&
    (rawRequestedSection === 'quota' ||
      rawRequestedSection === 'balance' ||
      rawRequestedSection === 'general' ||
      rawRequestedSection === 'config')
      ? 'config'
      : activeTab === 'basic' && rawRequestedSection === ''
        ? 'general'
        : activeTab === 'content' && rawRequestedSection === ''
          ? 'notice'
          : activeTab === 'runtime' && rawRequestedSection === ''
            ? 'monitor'
            : rawRequestedSection;
  const activeSection =
    sectionKeys.includes(requestedSection) && requestedSection !== ''
      ? requestedSection
      : sectionKeys[0] || '';
  const pageTitle = activeGroup?.label || t('setting.title');
  const activeSectionLabel =
    activeGroup?.sections?.find((item) => item.key === activeSection)?.label || '';
  const singleGroupMode = visibleMenuGroups.length === 1;
  const hideSettingsMenu =
    singleGroupMode &&
    Number(activeGroup?.sections?.length || 0) <= 1;

  const goToSection = (tab, section) => {
    const nextParams = new URLSearchParams(searchParams);
    nextParams.set('tab', tab);
    nextParams.set('section', section);
    setSearchParams(nextParams);
  };

  const settingsMenuItems = visibleMenuGroups.map((group) => ({
    key: group.key,
    type: singleGroupMode ? undefined : 'group',
    label: singleGroupMode ? undefined : group.label,
    children: group.sections.map((section) => ({
      key: `${group.key}:${section.key}`,
      label: section.label,
    })),
  }));

  const renderContent = () => {
    if (activeTab === 'operation') {
      return <OperationSetting section={activeSection} />;
    }
    if (activeTab === 'currency') {
      return <CurrencySetting section={activeSection} />;
    }
    if (activeTab === 'exchange') {
      return <ExchangeRateSetting section={activeSection} />;
    }
    if (activeTab === 'basic') {
      return <SystemSetting section={activeSection} />;
    }
    if (activeTab === 'runtime') {
      return <OperationSetting section={activeSection} />;
    }
    if (activeTab === 'content') {
      return <OtherSetting section={activeSection} />;
    }
    return <div className='router-empty-cell'>{t('setting.empty_admin', '暂无可配置项')}</div>;
  };

  return (
    <div className='dashboard-container'>
      <AppFilterHeader
        breadcrumbs={[
          { key: 'workspace', label: t('header.admin_workspace') },
          {
            key: 'section',
            label:
              activeTab === 'operation' ||
              activeTab === 'currency' ||
              activeTab === 'exchange'
                ? t('header.platform_operation')
                : t('header.setting_center'),
          },
          ...(activeGroup
            ? [
                {
                  key: 'group',
                  label: activeGroup.label,
                  active: activeSectionLabel === '' || activeSectionLabel === activeGroup.label,
                },
              ]
            : []),
          ...(activeSectionLabel !== '' && activeSectionLabel !== activeGroup?.label
            ? [
                {
                  key: 'section-current',
                  label: activeSectionLabel,
                  active: true,
                },
              ]
            : []),
        ]}
        title={pageTitle}
      />
      <AppSection>
        {visibleMenuGroups.length > 0 ? (
          hideSettingsMenu ? (
            renderContent()
          ) : (
            <div className='router-settings-layout'>
              <div className='router-settings-menu-column'>
                <AppNavMenu
                  className='router-settings-nav-menu'
                  mode='inline'
                  selectable
                  items={settingsMenuItems}
                  selectedKeys={[`${activeTab}:${activeSection}`]}
                  onClick={({ key }) => {
                    const [tab, section] = String(key || '').split(':');
                    if (tab && section) {
                      goToSection(tab, section);
                    }
                  }}
                />
              </div>
              <div className='router-settings-content-column'>
                {renderContent()}
              </div>
            </div>
          )
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
