import React from 'react';
import { QRCode } from 'antd';

function AppQRCode({ value, size = 220, ...props }) {
  return <QRCode value={value} size={size} {...props} />;
}

export default AppQRCode;
