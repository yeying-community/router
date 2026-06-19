import React, { forwardRef } from 'react';
import AppButton from './AppButton';
import AppIcon from './AppIcon';
import AppTooltip from './AppTooltip';

const AppTableActionButton = forwardRef(function AppTableActionButton(
  {
    title,
    icon,
    className = '',
    color,
    disabled = false,
    loading = false,
    ...props
  },
  ref,
) {
  const buttonClassName = [
    'router-inline-button',
    'router-table-action-button',
    className,
  ]
    .filter(Boolean)
    .join(' ');

  return (
    <AppTooltip title={title}>
      <span ref={ref} className='router-table-action-button-wrap'>
        <AppButton
          {...props}
          type='button'
          color={color}
          disabled={disabled}
          loading={loading}
          aria-label={title}
          className={buttonClassName}
        >
          {loading ? null : <AppIcon name={icon} />}
        </AppButton>
      </span>
    </AppTooltip>
  );
});

export default AppTableActionButton;
