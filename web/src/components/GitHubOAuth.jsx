import React, { useContext, useEffect, useState } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { API, showError, showSuccess } from '../helpers';
import { UserContext } from '../context/User';
import { AppSpin } from '../router-ui';
import { resolvePostLoginPath } from '../helpers/authRedirect';

const GitHubOAuth = () => {
  const [searchParams] = useSearchParams();

  const [, userDispatch] = useContext(UserContext);
  const [prompt, setPrompt] = useState('处理中...');

  let navigate = useNavigate();

  const sendCode = async (code, state, count) => {
    const res = await API.get(
      `/api/v1/public/oauth/github?code=${code}&state=${state}`,
    );
    const { success, message, data } = res.data;
    if (success) {
      if (message === 'bind') {
        showSuccess('绑定成功！');
        navigate('/setting');
      } else {
        userDispatch({ type: 'login', payload: data });
        localStorage.setItem('user', JSON.stringify(data));
        navigate(
          resolvePostLoginPath(
            searchParams,
            Number(data?.role) >= 10 ? '/admin/dashboard' : '/workspace/entry',
          ),
          { replace: true },
        );
      }
    } else {
      showError(message);
      if (count === 0) {
        setPrompt(`操作失败，重定向至登录界面中...`);
        navigate('/setting'); // in case this is failed to bind GitHub
        return;
      }
      count++;
      setPrompt(`出现错误，第 ${count} 次重试中...`);
      await new Promise((resolve) => setTimeout(resolve, count * 2000));
      await sendCode(code, state, count);
    }
  };

  useEffect(() => {
    let code = searchParams.get('code');
    let state = searchParams.get('state');
    sendCode(code, state, 0).then();
  }, []);

  return (
    <div className='router-loading-shell router-loading-shell-md'>
      <AppSpin size='large' description={prompt} />
    </div>
  );
};

export default GitHubOAuth;
