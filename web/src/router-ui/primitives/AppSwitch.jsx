import React, { forwardRef } from 'react';
import { Switch } from 'antd';

const AppSwitch = forwardRef(function AppSwitch(
  {
    className = '',
    checked,
    disabled,
    loading,
    name,
    size = 'small',
    onChange,
    ...props
  },
  ref,
) {
  const nextClassName = ['router-ui-switch', className]
    .filter(Boolean)
    .join(' ');

  const handleChange = (nextChecked, event) => {
    if (typeof onChange === 'function') {
      onChange(event ?? null, {
        name,
        checked: nextChecked,
        value: nextChecked,
      });
    }
  };

  return (
    <Switch
      {...props}
      ref={ref}
      className={nextClassName}
      checked={checked}
      disabled={disabled}
      loading={loading}
      size={size}
      onChange={handleChange}
    />
  );
});

export default AppSwitch;
