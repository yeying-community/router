import React from 'react';
import { useNavigate } from 'react-router-dom';
import RedemptionsTable from '../../components/RedemptionsTable';

const Redemption = () => {
  const navigate = useNavigate();

  return (
    <div className='dashboard-container'>
      <RedemptionsTable
        headerMeta={
          <button
            type='button'
            className='router-breadcrumb-link router-page-header-link'
            onClick={() => navigate('/admin/redemption/records')}
          >
            兑换记录
          </button>
        }
      />
    </div>
  );
};

export default Redemption;
