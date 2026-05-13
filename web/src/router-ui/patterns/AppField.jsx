import React from 'react';

function AppField({
  className = '',
  label,
  extra,
  hint,
  required = false,
  readOnly = false,
  children,
}) {
  const nextClassName = [
    'router-ui-field',
    readOnly ? 'readonly' : '',
    className,
  ]
    .filter(Boolean)
    .join(' ');
  return (
    <div className={nextClassName}>
      {label ? (
        <label className='router-ui-field-label'>
          <span>{label}</span>
          {required ? <span className='router-ui-field-required'>*</span> : null}
        </label>
      ) : null}
      {extra ? <div className='router-ui-field-extra'>{extra}</div> : null}
      <div className='router-ui-field-control'>{children}</div>
      {hint ? <div className='router-ui-field-hint'>{hint}</div> : null}
    </div>
  );
}

export default AppField;
