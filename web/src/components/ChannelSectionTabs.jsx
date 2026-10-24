import React from 'react';
import { useTranslation } from 'react-i18next';
import SectionTabs from './SectionTabs';

// Channels domain sub-nav: list / health. Operational incidents are handled in
// the duty station so channels do not expose a second acknowledgement surface.
// All tabs point at /admin/channel?tab=... so the ChannelLayout shell keeps
// its breadcrumb/header mounted and swaps the body in place.
function ChannelSectionTabs({ active = 'list' }) {
  const { t } = useTranslation();
  return (
    <SectionTabs
      active={active}
      tabs={[
        { key: 'list', label: t('channel.tabs.list'), to: '/admin/channel?tab=list' },
        {
          key: 'health',
          label: t('channel.tabs.health'),
          to: '/admin/channel?tab=health',
        },
      ]}
    />
  );
}

export default ChannelSectionTabs;
