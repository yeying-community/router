import React, { forwardRef } from 'react';
import { Tag } from 'antd';

const COLOR_MAP = {
  grey: 'default',
  black: 'default',
  olive: 'processing',
  violet: 'purple',
};

const AppTag = forwardRef(function AppTag(
  { className = '', children, basic, ...props },
  ref,
) {
  const nextClassName = ['router-ui-tag', className].filter(Boolean).join(' ');
  const nextColor = COLOR_MAP[props.color] || props.color;
  return (
    <Tag
      {...props}
      ref={ref}
      color={nextColor}
      className={nextClassName}
    >
      {children}
    </Tag>
  );
});

export default AppTag;
