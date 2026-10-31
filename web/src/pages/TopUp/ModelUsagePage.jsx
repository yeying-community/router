import React from 'react';
import useUrlState from '../../hooks/useUrlState';
import ModelRankingSection from '../WorkspaceModels/ModelRankingSection';

const ModelUsagePage = () => {
  const [{ keyword, period }, patchFilters] = useUrlState({
    keyword: { param: 'usage_q', default: '' },
    period: {
      param: 'usage_period',
      default: 'last_7_days',
      parse: (raw) =>
        ['last_24_hours', 'last_7_days', 'last_30_days'].includes(raw)
          ? raw
          : 'last_7_days',
    },
  });

  return (
    <ModelRankingSection
      className='router-topup-model-usage'
      headingId='topup-model-usage-heading'
      titleKey='topup.model_usage.title'
      keyword={keyword}
      scope='me'
      period={period}
      onKeywordChange={(value) => patchFilters({ keyword: value })}
      onPeriodChange={(value) => patchFilters({ period: value })}
    />
  );
};

export default ModelUsagePage;
