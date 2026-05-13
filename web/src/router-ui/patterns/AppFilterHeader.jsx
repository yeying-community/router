import React from 'react';
import AppBreadcrumb from './AppBreadcrumb';
import AppToolbar from './AppToolbar';

function AppFilterHeader({
  className = '',
  breadcrumbs,
  title,
  titleTag: TitleTag = 'div',
  titleClassName = 'router-toolbar-title',
  meta,
  metaClassName = 'router-toolbar-meta',
  actions,
  picker,
  query,
  end,
  startClassName = '',
  endClassName = '',
}) {
  const visibleBreadcrumbs = Array.isArray(breadcrumbs)
    ? breadcrumbs
        .filter(
          (item) =>
            item?.key !== 'admin' &&
            item?.key !== 'workspace' &&
            item?.key !== 'user-workspace',
        )
        .filter(Boolean)
    : breadcrumbs;
  const hasBreadcrumbs =
    Array.isArray(visibleBreadcrumbs)
      ? visibleBreadcrumbs.length > 0
      : !!visibleBreadcrumbs;
  const nextClassName = ['router-log-toolbar', 'router-block-gap-sm', className]
    .filter(Boolean)
    .join(' ');
  const nextStartClassName = ['router-filter-header-title-row', startClassName]
    .filter(Boolean)
    .join(' ');
  const nextQueryClassName = ['router-filter-toolbar-query', endClassName]
    .filter(Boolean)
    .join(' ');
  const hasTitleRow = hasBreadcrumbs || title || meta;
  const hasToolbar = picker || query || actions || end;
  const resolvedTitleRow = hasTitleRow ? (
    <div className={nextStartClassName}>
      {hasBreadcrumbs ? (
        <AppBreadcrumb
          className='router-page-header-breadcrumb'
          items={visibleBreadcrumbs}
        />
      ) : title ? (
        <TitleTag className={titleClassName}>{title}</TitleTag>
      ) : null}
      {meta ? <span className={metaClassName}>{meta}</span> : null}
    </div>
  ) : null;
  const resolvedToolbarStart =
    picker || query ? (
      <>
        {picker}
        {query}
      </>
    ) : null;
  const resolvedToolbarEnd =
    actions || end ? (
      <>
        {actions}
        {end}
      </>
    ) : null;

  return (
    <div className={nextClassName}>
      {resolvedTitleRow}
      {hasToolbar ? (
        <AppToolbar
          className='router-filter-toolbar'
          start={resolvedToolbarStart}
          end={resolvedToolbarEnd}
          startClassName={nextQueryClassName}
          endClassName='router-filter-toolbar-actions'
        />
      ) : null}
    </div>
  );
}

export default AppFilterHeader;
