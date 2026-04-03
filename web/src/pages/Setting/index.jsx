import React from 'react';
import { useTranslation } from 'react-i18next';
import { Card, Grid, Menu } from 'semantic-ui-react';
import { useLocation, useSearchParams } from 'react-router-dom';
import SystemSetting from '../../components/SystemSetting';
import { isRoot } from '../../helpers';
import OtherSetting from '../../components/OtherSetting';
import PersonalSetting from '../../components/PersonalSetting';
import OperationSetting from '../../components/OperationSetting';
import ExchangeRateSetting from '../../components/ExchangeRateSetting';
import CurrencySetting from '../../components/CurrencySetting';

const Setting = () => {
  const { t } = useTranslation();
  const location = useLocation();
  const [searchParams, setSearchParams] = useSearchParams();
  const isAdminWorkspace = location.pathname.startsWith('/admin/');

  if (!isAdminWorkspace) {
    return (
      <div className='dashboard-container'>
        <Card fluid className='chart-card'>
          <Card.Content>
            <Card.Header className='header router-page-title'>
              {t('setting.title')}
            </Card.Header>
            <PersonalSetting />
          </Card.Content>
        </Card>
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
      key: 'general',
      label: t('setting.system.general.title'),
      sections: [
        { key: 'general', label: t('setting.system.general.title') },
      ],
    });
    menuGroups.push({
      key: 'smtp',
      label: t('setting.system.smtp.title'),
      sections: [
        { key: 'smtp', label: t('setting.system.smtp.title') },
      ],
    });
    menuGroups.push({
      key: 'login',
      label: t('setting.system.login.title'),
      sections: [
        { key: 'login', label: t('setting.system.login.title') },
      ],
    });
    menuGroups.push({
      key: 'monitor',
      label: t('setting.operation.monitor.title'),
      sections: [
        { key: 'monitor', label: t('setting.operation.monitor.title') },
      ],
    });
    menuGroups.push({
      key: 'log_setting',
      label: t('setting.operation.log.title'),
      sections: [
        { key: 'log', label: t('setting.operation.log.title') },
      ],
    });
    menuGroups.push({
      key: 'notice',
      label: t('setting.system.notice', '站点公告'),
      sections: [
        { key: 'notice', label: t('setting.system.notice', '站点公告') },
      ],
    });
    menuGroups.push({
      key: 'content',
      label: t('setting.other.content.title'),
      sections: [
        { key: 'content', label: t('setting.other.content.title') },
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
        ? 'smtp'
        : rawRequestedSection === 'login'
          ? 'login'
          : rawRequestedSection === 'monitor'
            ? 'monitor'
            : rawRequestedSection === 'log'
              ? 'log_setting'
              : 'general'
      : rawRequestedTab === 'operation'
        ? rawRequestedSection === 'monitor'
          ? 'monitor'
          : rawRequestedSection === 'log'
            ? 'log_setting'
            : 'operation'
        : rawRequestedTab === 'other'
          ? rawRequestedSection === 'content'
            ? 'content'
            : 'notice'
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
      : rawRequestedSection;
  const activeSection =
    sectionKeys.includes(requestedSection) && requestedSection !== ''
      ? requestedSection
      : sectionKeys[0] || '';

  const goToSection = (tab, section) => {
    const nextParams = new URLSearchParams(searchParams);
    nextParams.set('tab', tab);
    nextParams.set('section', section);
    setSearchParams(nextParams);
  };

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
    if (
      activeTab === 'general' ||
      activeTab === 'smtp' ||
      activeTab === 'login'
    ) {
      return <SystemSetting section={activeSection} />;
    }
    if (activeTab === 'monitor' || activeTab === 'log_setting') {
      return <OperationSetting section={activeSection} />;
    }
    if (activeTab === 'notice' || activeTab === 'content') {
      return <OtherSetting section={activeSection} />;
    }
    return <div className='router-empty-cell'>{t('setting.empty_admin', '暂无可配置项')}</div>;
  };

  const pageTitle = activeGroup?.label || t('setting.title');
  const singleGroupMode = visibleMenuGroups.length === 1;
  const hideSettingsMenu = singleGroupMode;

  return (
    <div className='dashboard-container'>
      <Card fluid className='chart-card'>
        <Card.Content>
          <Card.Header className='header router-page-title'>
            {pageTitle}
          </Card.Header>
          {visibleMenuGroups.length > 0 ? (
            hideSettingsMenu ? (
              renderContent()
            ) : (
              <Grid stackable columns={2} className='router-settings-layout'>
                <Grid.Column width={3} className='router-settings-menu-column'>
                  <Menu fluid vertical className='router-settings-menu'>
                    {visibleMenuGroups.map((group) => (
                      <Menu.Item key={group.key} className='router-settings-menu-group'>
                        <Menu.Header>{group.label}</Menu.Header>
                        <Menu.Menu>
                          {group.sections.map((section) => (
                            <Menu.Item
                              key={`${group.key}-${section.key}`}
                              active={
                                activeTab === group.key &&
                                activeSection === section.key
                              }
                              onClick={() => goToSection(group.key, section.key)}
                            >
                              {section.label}
                            </Menu.Item>
                          ))}
                        </Menu.Menu>
                      </Menu.Item>
                    ))}
                  </Menu>
                </Grid.Column>
                <Grid.Column width={13}>
                  {renderContent()}
                </Grid.Column>
              </Grid>
            )
          ) : (
            <div className='router-empty-cell'>
              {t('setting.empty_admin', '暂无可配置项')}
            </div>
          )}
        </Card.Content>
      </Card>
    </div>
  );
};

export default Setting;
