import React, { Suspense, lazy, useCallback, useContext, useEffect, useState } from 'react';
import { Navigate, Route, Routes, useLocation } from 'react-router-dom';
import Loading from './components/Loading';
import { PrivateRoute } from './components/PrivateRoute';
import NotFound from './pages/NotFound';
import {
  API,
  getLogo,
  getSystemName,
  isAdmin,
  showError,
  showNotice,
} from './helpers';
import { UserContext } from './context/User';
import { StatusContext } from './context/Status';
import AdminLayout from './layouts/AdminLayout';
import UserLayout from './layouts/UserLayout';
import UserWorkspaceLayout from './layouts/UserWorkspaceLayout';

const User = lazy(() => import('./pages/User'));
const RegisterForm = lazy(() => import('./components/RegisterForm'));
const LoginForm = lazy(() => import('./components/LoginForm'));
const Setting = lazy(() => import('./pages/Setting'));
const UserDetail = lazy(() => import('./pages/User/EditUser'));
const AddUser = lazy(() => import('./pages/User/AddUser'));
const PasswordResetForm = lazy(() => import('./components/PasswordResetForm'));
const PasswordResetConfirm = lazy(() => import('./components/PasswordResetConfirm'));
const Channel = lazy(() => import('./pages/Channel'));
const Token = lazy(() => import('./pages/Token'));
const EditToken = lazy(() => import('./pages/Token/EditToken'));
const EditChannel = lazy(() => import('./pages/Channel/EditChannel'));
const AddChannel = lazy(() => import('./pages/Channel/AddChannel'));
const Redemption = lazy(() => import('./pages/Redemption'));
const EditRedemption = lazy(() => import('./pages/Redemption/EditRedemption'));
const RedemptionDetail = lazy(() => import('./pages/Redemption/RedemptionDetail'));
const TopUp = lazy(() => import('./pages/TopUp'));
const TopUpOrderDetail = lazy(() => import('./pages/TopUp/TopUpOrderDetail'));
const Log = lazy(() => import('./pages/Log'));
const LogDetail = lazy(() => import('./pages/Log/Detail'));
const Chat = lazy(() => import('./pages/Chat'));
const Dashboard = lazy(() => import('./pages/Dashboard'));
const AdminDashboard = lazy(() => import('./pages/AdminDashboard'));
const Providers = lazy(() => import('./pages/Providers'));
const Group = lazy(() => import('./pages/Group'));
const Package = lazy(() => import('./pages/Package'));
const PackageDetail = lazy(() => import('./pages/Package/Detail'));
const AdminTopup = lazy(() => import('./pages/AdminTopup'));
const Task = lazy(() => import('./pages/Task'));
const TaskDetail = lazy(() => import('./pages/Task/Detail'));
const FlowPage = lazy(() => import('./pages/Flow'));
const TopupReconcileDetail = lazy(() => import('./pages/Flow/TopupReconcileDetail'));
const TopupDetail = lazy(() => import('./pages/Flow/TopupDetail'));
const PackageFlowDetail = lazy(() => import('./pages/Flow/PackageDetail'));
const RedemptionFlowDetail = lazy(() => import('./pages/Flow/RedemptionDetail'));
const ServicePricing = lazy(() => import('./pages/ServicePricing'));
const HelpDoc = lazy(() => import('./pages/HelpDoc'));
const WorkspaceStart = lazy(() => import('./pages/WorkspaceStart'));

const APP_VERSION = import.meta.env.VITE_APP_VERSION || '';

function AdminOnlyRoute({ children }) {
  if (!isAdmin()) {
    return <Navigate to='/workspace/entry' replace />;
  }
  return children;
}

function RootRedirect() {
  return (
    <Navigate
      to={isAdmin() ? '/admin/dashboard' : '/workspace/entry'}
      replace
    />
  );
}

function DashboardRedirect() {
  return (
    <Navigate
      to={isAdmin() ? '/admin/dashboard' : '/workspace/entry'}
      replace
    />
  );
}

function UserWorkspaceEntryRedirect() {
  const [targetPath, setTargetPath] = useState('');

  useEffect(() => {
    let active = true;

    const resolveTargetPath = async () => {
      try {
        const [packageResponse, balanceResponse] = await Promise.all([
          API.get('/api/v1/public/user/package/subscription'),
          API.get('/api/v1/public/user/topup/balance/summary'),
        ]);
        const packageData = packageResponse?.data?.success
          ? packageResponse?.data?.data || null
          : null;
        const balanceData = balanceResponse?.data?.success
          ? balanceResponse?.data?.data || null
          : null;
        const hasActivePackage = String(packageData?.package_id || '').trim() !== '';
        const totalBalance = Number(
          balanceData?.total_yyc_balance ?? balanceData?.yyc_balance ?? balanceData?.quota ?? 0,
        );
        const hasBalance = Number.isFinite(totalBalance) && totalBalance > 0;
        if (!active) {
          return;
        }
        setTargetPath(hasActivePackage || hasBalance ? '/workspace/dashboard' : '/workspace/start');
      } catch (error) {
        if (!active) {
          return;
        }
        setTargetPath('/workspace/start');
      }
    };

    resolveTargetPath().then();

    return () => {
      active = false;
    };
  }, []);

  if (targetPath === '') {
    return <Loading />;
  }

  return <Navigate to={targetPath} replace />;
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
        element={<Navigate to='/workspace/entry' replace />}
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
          element={<Navigate to='/workspace/entry' replace />}
        />
      </Route>

      <Route
        element={
          <PrivateRoute>
            <UserWorkspaceLayout />
          </PrivateRoute>
        }
      >
        <Route path='/workspace/entry' element={<UserWorkspaceEntryRedirect />} />
        <Route
          path='/workspace/start'
          element={
            <Suspense fallback={<Loading />}>
              <WorkspaceStart />
            </Suspense>
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
        <Route
          path='/workspace/token'
          element={
            <Suspense fallback={<Loading />}>
              <Token />
            </Suspense>
          }
        />
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
        <Route
          path='/workspace/log'
          element={
            <Suspense fallback={<Loading />}>
              <Log />
            </Suspense>
          }
        />
        <Route
          path='/workspace/log/:id'
          element={
            <Suspense fallback={<Loading />}>
              <LogDetail />
            </Suspense>
          }
        />
        <Route
          path='/workspace/task'
          element={
            <Suspense fallback={<Loading />}>
              <Task />
            </Suspense>
          }
        />
        <Route
          path='/workspace/task/:id'
          element={
            <Suspense fallback={<Loading />}>
              <TaskDetail />
            </Suspense>
          }
        />
        <Route
          path='/workspace/dashboard'
          element={
            <Suspense fallback={<Loading />}>
              <Dashboard />
            </Suspense>
          }
        />
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
        <Route
          path='/admin/channel'
          element={
            <Suspense fallback={<Loading />}>
              <Channel />
            </Suspense>
          }
        />
        <Route
          path='/admin/channel/tasks'
          element={
            <Suspense fallback={<Loading />}>
              <Task />
            </Suspense>
          }
        />
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
              <AddChannel />
            </Suspense>
          }
        />
        <Route
          path='/admin/provider'
          element={
            <Suspense fallback={<Loading />}>
              <Providers />
            </Suspense>
          }
        />
        <Route
          path='/admin/group'
          element={
            <Suspense fallback={<Loading />}>
              <Group />
            </Suspense>
          }
        />
        <Route
          path='/admin/group/detail/:id'
          element={
            <Suspense fallback={<Loading />}>
              <Group />
            </Suspense>
          }
        />
        <Route
          path='/admin/package'
          element={
            <Suspense fallback={<Loading />}>
              <Package />
            </Suspense>
          }
        />
        <Route
          path='/admin/package/detail/:id'
          element={
            <Suspense fallback={<Loading />}>
              <PackageDetail />
            </Suspense>
          }
        />
        <Route
          path='/admin/topup'
          element={
            <Suspense fallback={<Loading />}>
              <AdminTopup />
            </Suspense>
          }
        />
        <Route
          path='/admin/flow/topup'
          element={
            <Suspense fallback={<Loading />}>
              <FlowPage kind='topup' />
            </Suspense>
          }
        />
        <Route
          path='/admin/flow/topup/:id'
          element={
            <Suspense fallback={<Loading />}>
              <TopupDetail />
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
          path='/admin/flow/package/:id'
          element={
            <Suspense fallback={<Loading />}>
              <PackageFlowDetail />
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
        <Route
          path='/admin/flow/redemption/:id'
          element={
            <Suspense fallback={<Loading />}>
              <RedemptionFlowDetail />
            </Suspense>
          }
        />
        <Route
          path='/admin/redemption'
          element={
            <Suspense fallback={<Loading />}>
              <Redemption />
            </Suspense>
          }
        />
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
        <Route
          path='/admin/user'
          element={
            <Suspense fallback={<Loading />}>
              <User />
            </Suspense>
          }
        />
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
        <Route
          path='/admin/dashboard'
          element={
            <Suspense fallback={<Loading />}>
              <AdminDashboard />
            </Suspense>
          }
        />
        <Route
          path='/admin/log'
          element={
            <Suspense fallback={<Loading />}>
              <Log />
            </Suspense>
          }
        />
        <Route
          path='/admin/log/:id'
          element={
            <Suspense fallback={<Loading />}>
              <LogDetail />
            </Suspense>
          }
        />
        <Route
          path='/admin/task'
          element={
            <Suspense fallback={<Loading />}>
              <Task />
            </Suspense>
          }
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
        element={<Navigate to='/workspace/entry' replace />}
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
