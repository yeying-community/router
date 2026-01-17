import React from 'react';

const Chat = () => {
  const chatLink = localStorage.getItem('chat_link');

  return (
    <iframe
      src={chatLink}
      title='chat'
      style={{ width: '100%', height: '85vh', border: 'none' }}
    />
  );
};


export default Chat;
