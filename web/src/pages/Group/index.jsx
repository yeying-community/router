import React from 'react';
import { useParams } from 'react-router-dom';
import GroupsManager from '../../components/GroupsManager';

const Group = () => {
  const { id: detailGroupId } = useParams();
  return (
    <div className='dashboard-container'>
      <GroupsManager detailGroupId={detailGroupId || ''} />
    </div>
  );
};

export default Group;
