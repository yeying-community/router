import React, { forwardRef } from 'react';
import { Checkbox } from 'antd';

const AppCheckbox = forwardRef(function AppCheckbox(
  {
    className = '',
    checked,
    disabled,
    indeterminate,
    name,
    onChange,
    children,
    ...props
  },
  ref,
) {
  const nextClassName = ['router-ui-checkbox', className]
    .filter(Boolean)
    .join(' ');

  const handleChange = (event) => {
    if (typeof onChange === 'function') {
      onChange(event, {
        name,
        checked: event?.target?.checked,
        value: event?.target?.checked,
      });
    }
  };

  return (
    <Checkbox
      {...props}
      ref={ref}
      className={nextClassName}
      checked={checked}
      disabled={disabled}
      indeterminate={indeterminate}
      onChange={handleChange}
    >
      {children}
    </Checkbox>
  );
});

export default AppCheckbox;
