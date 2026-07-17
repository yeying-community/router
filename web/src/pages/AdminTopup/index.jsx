import React from 'react';
import { useNavigate } from 'react-router-dom';
import TopupPlansManager from '../../components/TopupPlansManager';

const AdminTopup = () => {
  const navigate = useNavigate();

  return (
    <div className='dashboard-container'>
      <TopupPlansManager
        headerMeta={
          <button
            type='button'
            className='router-breadcrumb-link router-page-header-link'
            onClick={() => navigate('/admin/topup/records')}
          >
            充值记录
          </button>
        }
      />
    </div>
  );
};

export default AdminTopup;
