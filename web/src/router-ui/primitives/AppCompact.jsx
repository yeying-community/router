import React from 'react';
import { Space } from 'antd';

function AppCompact({ className = '', block = false, children, ...props }) {
  return (
    <Space.Compact className={className} block={block} {...props}>
      {children}
    </Space.Compact>
  );
}

export default AppCompact;
