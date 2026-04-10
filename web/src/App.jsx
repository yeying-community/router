import React, { Suspense, useCallback, useContext, useEffect } from 'react';
import { Navigate, Route, Routes, useLocation } from 'react-router-dom';
import Loading from './components/Loading';
import User from './pages/User';
import { PrivateRoute } from './components/PrivateRoute';
import RegisterForm from './components/RegisterForm';
import LoginForm from './components/LoginForm';
import NotFound from './pages/NotFound';
import Setting from './pages/Setting';
import UserDetail from './pages/User/EditUser';
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
import TopUpOrderDetail from './pages/TopUp/TopUpOrderDetail';
import Log from './pages/Log';
import LogDetail from './pages/Log/Detail';
import Chat from './pages/Chat';
import Dashboard from './pages/Dashboard';
import AdminDashboard from './pages/AdminDashboard';
import Providers from './pages/Providers';
import Group from './pages/Group';
import Package from './pages/Package';
import PackageDetail from './pages/Package/Detail';
import AdminTopup from './pages/AdminTopup';
import Task from './pages/Task';
import TaskDetail from './pages/Task/Detail';
import FlowPage from './pages/Flow';
import TopupReconcileDetail from './pages/Flow/TopupReconcileDetail';
import ServicePricing from './pages/ServicePricing';
import HelpDoc from './pages/HelpDoc';
import AdminLayout from './layouts/AdminLayout';
import UserLayout from './layouts/UserLayout';
import UserWorkspaceLayout from './layouts/UserWorkspaceLayout';

const APP_VERSION = import.meta.env.VITE_APP_VERSION || '';

function AdminOnlyRoute({ children }) {
  if (!isAdmin()) {
    return <Navigate to='/workspace/service/pricing' replace />;
  }
  return children;
}

function RootRedirect() {
  return (
    <Navigate
      to={isAdmin() ? '/admin/dashboard' : '/workspace/service/pricing'}
      replace
    />
  );
}

function DashboardRedirect() {
  return (
    <Navigate
      to={isAdmin() ? '/admin/dashboard' : '/workspace/service/pricing'}
      replace
    />
  );
}

function SettingRedirect() {
  return (
    <Navigate
      to={
        isAdmin()
          ? '/admin/setting?tab=general&section=general'
          : '/workspace/setting'
      }
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
      state={location.state}
      replace
    />
  );
}

function ChannelEditRedirect() {
  const location = useLocation();
  const suffix = location.pathname.startsWith('/admin/channel/edit/')
    ? location.pathname.slice('/admin/channel/edit/'.length)
    : '';
  return (
    <Navigate
      to={`/admin/channel/detail/${suffix}${location.search}${location.hash}`}
      state={location.state}
      replace
    />
  );
}

function UserEditRedirect() {
  const location = useLocation();
  const suffix = location.pathname.startsWith('/admin/user/edit/')
    ? location.pathname.slice('/admin/user/edit/'.length)
    : '';
  const targetPath = suffix ? `/admin/user/detail/${suffix}` : '/admin/user';
  return (
    <Navigate
      to={`${targetPath}${location.search}${location.hash}`}
      state={location.state}
      replace
    />
  );
}

function RedemptionEditRedirect() {
  const location = useLocation();
  const suffix = location.pathname.startsWith('/admin/redemption/edit/')
    ? location.pathname.slice('/admin/redemption/edit/'.length)
    : '';
  const nextSearchParams = new URLSearchParams(location.search);
  nextSearchParams.set('edit', '1');
  const search = nextSearchParams.toString();
  return (
    <Navigate
      to={`/admin/redemption/${suffix}${search ? `?${search}` : ''}${location.hash}`}
      state={location.state}
      replace
    />
  );
}

function TokenEditRedirect() {
  const location = useLocation();
  const suffix = location.pathname.startsWith('/workspace/token/edit/')
    ? location.pathname.slice('/workspace/token/edit/'.length)
    : '';
  const nextSearchParams = new URLSearchParams(location.search);
  nextSearchParams.set('edit', '1');
  const search = nextSearchParams.toString();
  return (
    <Navigate
      to={`/workspace/token/${suffix}${search ? `?${search}` : ''}${location.hash}`}
      state={location.state}
      replace
    />
  );
}

function TopUpTabRedirect() {
  const location = useLocation();
  const suffix = location.pathname.startsWith('/workspace/topup/')
    ? location.pathname.slice('/workspace/topup/'.length)
    : '';
  const tab = suffix.split('/')[0];
  const nextSearchParams = new URLSearchParams(location.search);
  if (tab) {
    nextSearchParams.set('tab', tab);
  }
  const search = nextSearchParams.toString();
  return (
    <Navigate
      to={`/workspace/topup${search ? `?${search}` : ''}${location.hash}`}
      state={location.state}
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
            `新版本可用：${data.version}，请使用快捷键 Shift + F5 刷新页面`,
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
      <Route path='/' element={<RootRedirect />} />
      <Route
        path='/workspace'
        element={<Navigate to='/workspace/service/pricing' replace />}
      />
      <Route
        path='/admin'
        element={<Navigate to='/admin/dashboard' replace />}
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
          element={<Navigate to='/workspace/service/pricing' replace />}
        />
      </Route>

      <Route
        element={
          <PrivateRoute>
            <UserWorkspaceLayout />
          </PrivateRoute>
        }
      >
        <Route
          path='/workspace/chat'
          element={
            <Suspense fallback={<Loading />}>
              <Chat />
            </Suspense>
          }
        />
        <Route path='/workspace/token' element={<Token />} />
        <Route
          path='/workspace/token/:id'
          element={
            <Suspense fallback={<Loading />}>
              <EditToken />
            </Suspense>
          }
        />
        <Route
          path='/workspace/token/edit/:id'
          element={
            <Suspense fallback={<Loading />}>
              <TokenEditRedirect />
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
          path='/workspace/topup/:tab'
          element={
            <Suspense fallback={<Loading />}>
              <TopUpTabRedirect />
            </Suspense>
          }
        />
        <Route
          path='/workspace/topup/orders/:id'
          element={
            <Suspense fallback={<Loading />}>
              <TopUpOrderDetail />
            </Suspense>
          }
        />
        <Route path='/workspace/log' element={<Log />} />
        <Route
          path='/workspace/log/:id'
          element={
            <Suspense fallback={<Loading />}>
              <LogDetail />
            </Suspense>
          }
        />
        <Route path='/workspace/task' element={<Task />} />
        <Route
          path='/workspace/task/:id'
          element={
            <Suspense fallback={<Loading />}>
              <TaskDetail />
            </Suspense>
          }
        />
        <Route path='/workspace/dashboard' element={<Dashboard />} />
        <Route
          path='/workspace/service/pricing'
          element={
            <Suspense fallback={<Loading />}>
              <ServicePricing />
            </Suspense>
          }
        />
        <Route
          path='/workspace/service/help'
          element={
            <Suspense fallback={<Loading />}>
              <HelpDoc />
            </Suspense>
          }
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
        <Route path='/admin/channel' element={<Channel />} />
        <Route path='/admin/channel/tasks' element={<Task />} />
        <Route
          path='/admin/channel/tasks/:id'
          element={
            <Suspense fallback={<Loading />}>
              <TaskDetail />
            </Suspense>
          }
        />
        <Route
          path='/admin/channel/edit/:id'
          element={
            <Suspense fallback={<Loading />}>
              <ChannelEditRedirect />
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
        <Route path='/admin/provider' element={<Providers />} />
        <Route path='/admin/group' element={<Group />} />
        <Route path='/admin/group/detail/:id' element={<Group />} />
        <Route path='/admin/package' element={<Package />} />
        <Route path='/admin/package/detail/:id' element={<PackageDetail />} />
        <Route path='/admin/topup' element={<AdminTopup />} />
        <Route
          path='/admin/flow/topup'
          element={
            <Suspense fallback={<Loading />}>
              <FlowPage kind='topup' />
            </Suspense>
          }
        />
        <Route
          path='/admin/flow/topup-reconcile'
          element={
            <Suspense fallback={<Loading />}>
              <FlowPage kind='topup-reconcile' />
            </Suspense>
          }
        />
        <Route
          path='/admin/flow/topup-reconcile/:id'
          element={
            <Suspense fallback={<Loading />}>
              <TopupReconcileDetail />
            </Suspense>
          }
        />
        <Route
          path='/admin/flow/package'
          element={
            <Suspense fallback={<Loading />}>
              <FlowPage kind='package' />
            </Suspense>
          }
        />
        <Route
          path='/admin/flow/redemption'
          element={
            <Suspense fallback={<Loading />}>
              <FlowPage kind='redemption' />
            </Suspense>
          }
        />
        <Route path='/admin/redemption' element={<Redemption />} />
        <Route
          path='/admin/redemption/edit/:id'
          element={
            <Suspense fallback={<Loading />}>
              <RedemptionEditRedirect />
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
        <Route path='/admin/user' element={<User />} />
        <Route
          path='/admin/user/detail/:id'
          element={
            <Suspense fallback={<Loading />}>
              <UserDetail />
            </Suspense>
          }
        />
        <Route
          path='/admin/user/edit'
          element={
            <Suspense fallback={<Loading />}>
              <UserEditRedirect />
            </Suspense>
          }
        />
        <Route
          path='/admin/user/edit/:id'
          element={
            <Suspense fallback={<Loading />}>
              <UserEditRedirect />
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
        <Route path='/admin/dashboard' element={<AdminDashboard />} />
        <Route path='/admin/log' element={<Log />} />
        <Route
          path='/admin/log/:id'
          element={
            <Suspense fallback={<Loading />}>
              <LogDetail />
            </Suspense>
          }
        />
        <Route path='/admin/task' element={<Task />} />
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
        element={<Navigate to='/workspace/service/pricing' replace />}
      />
      <Route path='/chat' element={<Navigate to='/workspace/chat' replace />} />
      <Route path='/dashboard' element={<DashboardRedirect />} />
      <Route path='/setting' element={<SettingRedirect />} />

      <Route
        path='/channel/*'
        element={<PrefixRedirect from='/channel' to='/admin/channel' />}
      />
      <Route
        path='/provider/*'
        element={<PrefixRedirect from='/provider' to='/admin/provider' />}
      />
      <Route
        path='/group/*'
        element={<PrefixRedirect from='/group' to='/admin/group' />}
      />
      <Route
        path='/package/*'
        element={<PrefixRedirect from='/package' to='/admin/package' />}
      />
      <Route
        path='/redemption/*'
        element={<PrefixRedirect from='/redemption' to='/admin/redemption' />}
      />
      <Route
        path='/user/*'
        element={<PrefixRedirect from='/user' to='/admin/user' />}
      />
      <Route
        path='/token/*'
        element={<PrefixRedirect from='/token' to='/workspace/token' />}
      />
      <Route
        path='/topup/*'
        element={<PrefixRedirect from='/topup' to='/workspace/topup' />}
      />
      <Route
        path='/log/*'
        element={<PrefixRedirect from='/log' to='/workspace/log' />}
      />

      <Route path='*' element={<NotFound />} />
    </Routes>
  );
}

export default App;
