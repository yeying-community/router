import React from 'react';
import { useNavigate } from 'react-router-dom';
import PackagesManager from '../../components/PackagesManager';

const Package = () => {
  const navigate = useNavigate();

  return (
    <div className='dashboard-container'>
      <PackagesManager
        headerMeta={
          <button
            type='button'
            className='router-breadcrumb-link router-page-header-link'
            onClick={() => navigate('/admin/package/records')}
          >
            购买记录
          </button>
        }
      />
    </div>
  );
};

export default Package;
