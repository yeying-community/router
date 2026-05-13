import React from 'react';
import BusinessFlowTable from '../../components/BusinessFlowTable';

const FlowPage = ({ kind }) => {
  return (
    <div className='dashboard-container'>
      <BusinessFlowTable kind={kind} />
    </div>
  );
};

export default FlowPage;
