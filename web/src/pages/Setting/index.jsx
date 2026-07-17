import React from 'react';
import { useTranslation } from 'react-i18next';
import { useLocation, useSearchParams } from 'react-router-dom';
import { AppFilterHeader, AppSection } from '../../router-ui';
import SystemSetting from '../../components/SystemSetting';
import { isRoot } from '../../helpers';
import OtherSetting from '../../components/OtherSetting';
import PersonalSetting from '../../components/PersonalSetting';
import OperationSetting from '../../components/OperationSetting';
import ExchangeRateSetting from '../../components/ExchangeRateSetting';
import CurrencySetting from '../../components/CurrencySetting';
import { resolveAdminSettingLocation } from '../../helpers/adminSetting';

const SettingCard = ({ title, description, children }) => (
  <AppSection
    className='router-settings-card'
    title={
      <div className='router-settings-card-heading'>
        <div className='router-settings-card-title'>{title}</div>
        {description ? (
          <div className='router-settings-card-description'>{description}</div>
        ) : null}
      </div>
    }
  >
    {children}
  </AppSection>
);

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
      key: 'payment',
      label: t('setting.groups.payment'),
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
  const pageTitle = t('setting.title');

  const renderContent = () => {
    if (activeTab === 'basic') {
      return (
        <div className='router-settings-page-content'>
          <SettingCard
            title={t('setting.system.general.title')}
            description={t('setting.system.general.description')}
          >
            <SystemSetting section='general' showSectionTitle={false} />
          </SettingCard>
          <SettingCard
            title={t('setting.system.smtp.title')}
            description={t('setting.system.smtp.description')}
          >
            <SystemSetting section='smtp' showSectionTitle={false} />
          </SettingCard>
          <SettingCard
            title={t('setting.system.login.title')}
            description={t('setting.system.login.description')}
          >
            <SystemSetting section='login' showSectionTitle={false} />
          </SettingCard>
        </div>
      );
    }
    if (activeTab === 'payment') {
      return (
        <div className='router-settings-page-content'>
          <SettingCard
            title={t('setting.operation.payment.title')}
            description={t('setting.operation.payment.description')}
          >
            <OperationSetting section='payment' showSectionTitle={false} />
          </SettingCard>
          <SettingCard
            title={t('setting.currency.catalog.title')}
            description={t('setting.currency.catalog.description')}
          >
            <CurrencySetting section='catalog' showSectionTitle={false} />
          </SettingCard>
          <SettingCard
            title={t('setting.exchange.title')}
            description={t('setting.exchange.description')}
          >
            <ExchangeRateSetting section='rates' showSectionTitle={false} />
          </SettingCard>
        </div>
      );
    }
    if (activeTab === 'billing') {
      return (
        <div className='router-settings-page-content'>
          <SettingCard
            title={t('setting.operation.quota.title')}
            description={t('setting.operation.quota.description')}
          >
            <OperationSetting section='balance' showSectionTitle={false} />
          </SettingCard>
          <SettingCard
            title={t('setting.operation.automation.title')}
            description={t('setting.operation.automation.description')}
          >
            <OperationSetting section='automation' showSectionTitle={false} />
          </SettingCard>
          <SettingCard
            title={t('setting.operation.pricing.title')}
          >
            <OperationSetting section='pricing' showSectionTitle={false} />
          </SettingCard>
        </div>
      );
    }
    if (activeTab === 'content') {
      return (
        <div className='router-settings-page-content'>
          <SettingCard
            title={t('setting.system.notice')}
            description={t('setting.system.notice_description')}
          >
            <OtherSetting section='notice' showSectionTitle={false} />
          </SettingCard>
          <SettingCard
            title={t('setting.other.content.title')}
            description={t('setting.other.content.description')}
          >
            <OtherSetting section='content' showSectionTitle={false} />
          </SettingCard>
        </div>
      );
    }
    if (activeTab === 'runtime') {
      return (
        <div className='router-settings-page-content'>
          <SettingCard
            title={t('setting.operation.monitor.title')}
            description={t('setting.operation.monitor.description')}
          >
            <OperationSetting section='monitor' showSectionTitle={false} />
          </SettingCard>
          <SettingCard
            title={t('setting.operation.retry.title')}
            description={t('setting.operation.retry.description')}
          >
            <OperationSetting section='retry' showSectionTitle={false} />
          </SettingCard>
          <SettingCard
            title={t('setting.operation.log.title')}
            description={t('setting.operation.log.description')}
          >
            <OperationSetting section='log' showSectionTitle={false} />
          </SettingCard>
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
