export const resolveAdminSettingLocation = (rawTab, rawSection) => {
  const normalizedTab = (rawTab || '').trim().toLowerCase();
  const normalizedSection = (rawSection || '').trim().toLowerCase();

  if (normalizedTab === 'currency' || normalizedTab === 'exchange') {
    return { tab: 'payment', section: normalizedTab };
  }

  if (
    normalizedTab === 'system' ||
    normalizedTab === 'general' ||
    normalizedTab === 'smtp' ||
    normalizedTab === 'login' ||
    normalizedTab === 'basic'
  ) {
    return { tab: 'basic', section: normalizedSection || 'general' };
  }

  if (normalizedTab === 'operation') {
    if (['monitor', 'retry', 'log'].includes(normalizedSection)) {
      return { tab: 'runtime', section: normalizedSection };
    }
    if (
      ['general', 'payment', 'automation', 'pricing', 'balance'].includes(
        normalizedSection,
      )
    ) {
      return { tab: 'billing', section: normalizedSection || 'balance' };
    }
    return { tab: 'billing', section: 'balance' };
  }

  if (
    normalizedTab === 'monitor' ||
    normalizedTab === 'retry' ||
    normalizedTab === 'log_setting' ||
    normalizedTab === 'runtime'
  ) {
    return {
      tab: 'runtime',
      section:
        normalizedTab === 'log_setting'
          ? 'log'
          : normalizedSection || (normalizedTab === 'runtime' ? 'monitor' : normalizedTab),
    };
  }

  if (
    normalizedTab === 'other' ||
    normalizedTab === 'notice' ||
    normalizedTab === 'content'
  ) {
    return {
      tab: 'content',
      section: normalizedTab === 'notice' ? 'notice' : normalizedSection || 'notice',
    };
  }

  return {
    tab: normalizedTab,
    section: normalizedSection,
  };
};
