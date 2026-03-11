import React, { Suspense, useCallback, useContext, useEffect } from 'react';
import { Navigate, Route, Routes, useLocation } from 'react-router-dom';
import Loading from './components/Loading';
import User from './pages/User';
import { PrivateRoute } from './components/PrivateRoute';
import RegisterForm from './components/RegisterForm';
import LoginForm from './components/LoginForm';
import NotFound from './pages/NotFound';
import Setting from './pages/Setting';
import EditUser from './pages/User/EditUser';
import AddUser from './pages/User/AddUser';
import {
  API,
  getLogo,
  getSystemName,
  isAdmin,
  showError,
  showNotice,
} from './helpers';
import PasswordResetForm from './components/PasswordResetForm';
import PasswordResetConfirm from './components/PasswordResetConfirm';
import { UserContext } from './context/User';
import { StatusContext } from './context/Status';
import Channel from './pages/Channel';
import Token from './pages/Token';
import EditToken from './pages/Token/EditToken';
import EditChannel from './pages/Channel/EditChannel';
import Redemption from './pages/Redemption';
import EditRedemption from './pages/Redemption/EditRedemption';
import RedemptionDetail from './pages/Redemption/RedemptionDetail';
import TopUp from './pages/TopUp';
import Log from './pages/Log';
import Chat from './pages/Chat';
import Dashboard from './pages/Dashboard';
import Providers from './pages/Providers';
import Group from './pages/Group';
import Task from './pages/Task';
import TaskDetail from './pages/Task/Detail';
import AdminLayout from './layouts/AdminLayout';
import UserLayout from './layouts/UserLayout';

const APP_VERSION = import.meta.env.VITE_APP_VERSION || '';

function AdminOnlyRoute({ children }) {
  if (!isAdmin()) {
    return (
      <Navigate
        to='/workspace/token'
        replace
      />
    );
  }
  return children;
}

function RootRedirect() {
  return (
    <Navigate
      to={isAdmin() ? '/admin/dashboard' : '/workspace/token'}
      replace
    />
  );
}

function DashboardRedirect() {
  return (
    <Navigate
      to={isAdmin() ? '/admin/dashboard' : '/workspace/dashboard'}
      replace
    />
  );
}

function SettingRedirect() {
  return (
    <Navigate
      to={isAdmin() ? '/admin/setting' : '/workspace/setting'}
      replace
    />
  );
}

function PrefixRedirect({ from, to }) {
  const location = useLocation();
  const suffix = location.pathname.startsWith(from)
    ? location.pathname.slice(from.length)
    : '';
  const targetPath = `${to}${suffix}`;
  return (
    <Navigate
      to={`${targetPath}${location.search}${location.hash}`}
      replace
    />
  );
}

function App() {
  const [, userDispatch] = useContext(UserContext);
  const [, statusDispatch] = useContext(StatusContext);

  const loadUser = useCallback(() => {
    let user = localStorage.getItem('user');
    if (user) {
      let data = JSON.parse(user);
      userDispatch({ type: 'login', payload: data });
    }
  }, [userDispatch]);

  const loadStatus = useCallback(async () => {
    try {
      const res = await API.get('/api/v1/public/status');
      const { success, message, data } = res.data || {};
      if (success && data) {
        localStorage.setItem('status', JSON.stringify(data));
        statusDispatch({ type: 'set', payload: data });
        localStorage.setItem('system_name', data.system_name);
        localStorage.setItem('logo', data.logo);
        localStorage.setItem('footer_html', data.footer_html);
        localStorage.setItem('quota_per_unit', data.quota_per_unit);
        localStorage.setItem('display_in_currency', data.display_in_currency);
        if (data.chat_link) {
          localStorage.setItem('chat_link', data.chat_link);
        } else {
          localStorage.removeItem('chat_link');
        }
        if (
          data.version !== APP_VERSION &&
          data.version !== 'v0.0.0' &&
          APP_VERSION !== ''
        ) {
          showNotice(
            `新版本可用：${data.version}，请使用快捷键 Shift + F5 刷新页面`
          );
        }
      } else {
        showError(message || '无法正常连接至服务器！');
      }
    } catch (error) {
      showError(error.message || '无法正常连接至服务器！');
    }
  }, [statusDispatch]);

  useEffect(() => {
    loadUser();
    loadStatus().then();
    let systemName = getSystemName();
    if (systemName) {
      document.title = systemName;
    }
    let logo = getLogo();
    if (logo) {
      let linkElement = document.querySelector("link[rel~='icon']");
      if (linkElement) {
        linkElement.href = logo;
      }
    }
  }, [loadUser, loadStatus]);

  return (
    <Routes>
      <Route
        path='/'
        element={<RootRedirect />}
      />
      <Route
        path='/workspace'
        element={
          <Navigate
            to='/workspace/token'
            replace
          />
        }
      />
      <Route
        path='/admin'
        element={
          <Navigate
            to='/admin/dashboard'
            replace
          />
        }
      />

      <Route
        path='/login'
        element={
          <Suspense fallback={<Loading />}>
            <LoginForm />
          </Suspense>
        }
      />

      <Route element={<UserLayout />}>
        <Route
          path='/register'
          element={
            <Suspense fallback={<Loading />}>
              <RegisterForm />
            </Suspense>
          }
        />
        <Route
          path='/reset'
          element={
            <Suspense fallback={<Loading />}>
              <PasswordResetForm />
            </Suspense>
          }
        />
        <Route
          path='/user/reset'
          element={
            <Suspense fallback={<Loading />}>
              <PasswordResetConfirm />
            </Suspense>
          }
        />
        <Route
          path='/workspace/about'
          element={
            <Navigate
              to='/workspace/token'
              replace
            />
          }
        />
        <Route
          path='/workspace/chat'
          element={
            <Suspense fallback={<Loading />}>
              <Chat />
            </Suspense>
          }
        />
      </Route>

      <Route
        element={
          <PrivateRoute>
            <UserLayout />
          </PrivateRoute>
        }
      >
        <Route
          path='/workspace/token'
          element={<Token />}
        />
        <Route
          path='/workspace/token/edit/:id'
          element={
            <Suspense fallback={<Loading />}>
              <EditToken />
            </Suspense>
          }
        />
        <Route
          path='/workspace/token/add'
          element={
            <Suspense fallback={<Loading />}>
              <EditToken />
            </Suspense>
          }
        />
        <Route
          path='/workspace/topup'
          element={
            <Suspense fallback={<Loading />}>
              <TopUp />
            </Suspense>
          }
        />
        <Route
          path='/workspace/log'
          element={<Log />}
        />
        <Route
          path='/workspace/dashboard'
          element={<Dashboard />}
        />
        <Route
          path='/workspace/setting'
          element={
            <Suspense fallback={<Loading />}>
              <Setting />
            </Suspense>
          }
        />
      </Route>

      <Route
        element={
          <PrivateRoute>
            <AdminOnlyRoute>
              <AdminLayout />
            </AdminOnlyRoute>
          </PrivateRoute>
        }
      >
        <Route
          path='/admin/channel'
          element={<Channel />}
        />
        <Route
          path='/admin/channel/edit/:id'
          element={
            <Suspense fallback={<Loading />}>
              <EditChannel />
            </Suspense>
          }
        />
        <Route
          path='/admin/channel/detail/:id'
          element={
            <Suspense fallback={<Loading />}>
              <EditChannel />
            </Suspense>
          }
        />
        <Route
          path='/admin/channel/add'
          element={
            <Suspense fallback={<Loading />}>
              <EditChannel />
            </Suspense>
          }
        />
        <Route
          path='/admin/provider'
          element={<Providers />}
        />
        <Route
          path='/admin/group'
          element={<Group />}
        />
        <Route
          path='/admin/redemption'
          element={<Redemption />}
        />
        <Route
          path='/admin/redemption/edit/:id'
          element={
            <Suspense fallback={<Loading />}>
              <EditRedemption />
            </Suspense>
          }
        />
        <Route
          path='/admin/redemption/:id'
          element={
            <Suspense fallback={<Loading />}>
              <RedemptionDetail />
            </Suspense>
          }
        />
        <Route
          path='/admin/redemption/add'
          element={
            <Suspense fallback={<Loading />}>
              <EditRedemption />
            </Suspense>
          }
        />
        <Route
          path='/admin/user'
          element={<User />}
        />
        <Route
          path='/admin/user/edit/:id'
          element={
            <Suspense fallback={<Loading />}>
              <EditUser />
            </Suspense>
          }
        />
        <Route
          path='/admin/user/edit'
          element={
            <Suspense fallback={<Loading />}>
              <EditUser />
            </Suspense>
          }
        />
        <Route
          path='/admin/user/add'
          element={
            <Suspense fallback={<Loading />}>
              <AddUser />
            </Suspense>
          }
        />
        <Route
          path='/admin/dashboard'
          element={<Dashboard />}
        />
        <Route
          path='/admin/log'
          element={<Log />}
        />
        <Route
          path='/admin/task'
          element={<Task />}
        />
        <Route
          path='/admin/task/:id'
          element={
            <Suspense fallback={<Loading />}>
              <TaskDetail />
            </Suspense>
          }
        />
        <Route
          path='/admin/setting'
          element={
            <Suspense fallback={<Loading />}>
              <Setting />
            </Suspense>
          }
        />
      </Route>

      <Route
        path='/about'
        element={
          <Navigate
            to='/workspace/token'
            replace
          />
        }
      />
      <Route
        path='/chat'
        element={
          <Navigate
            to='/workspace/chat'
            replace
          />
        }
      />
      <Route
        path='/dashboard'
        element={<DashboardRedirect />}
      />
      <Route
        path='/setting'
        element={<SettingRedirect />}
      />

      <Route
        path='/channel/*'
        element={
          <PrefixRedirect
            from='/channel'
            to='/admin/channel'
          />
        }
      />
      <Route
        path='/provider/*'
        element={
          <PrefixRedirect
            from='/provider'
            to='/admin/provider'
          />
        }
      />
      <Route
        path='/redemption/*'
        element={
          <PrefixRedirect
            from='/redemption'
            to='/admin/redemption'
          />
        }
      />
      <Route
        path='/user/*'
        element={
          <PrefixRedirect
            from='/user'
            to='/admin/user'
          />
        }
      />
      <Route
        path='/token/*'
        element={
          <PrefixRedirect
            from='/token'
            to='/workspace/token'
          />
        }
      />
      <Route
        path='/topup/*'
        element={
          <PrefixRedirect
            from='/topup'
            to='/workspace/topup'
          />
        }
      />
      <Route
        path='/log/*'
        element={
          <PrefixRedirect
            from='/log'
            to='/workspace/log'
          />
        }
      />

      <Route
        path='*'
        element={<NotFound />}
      />
    </Routes>
  );
}

export default App;
