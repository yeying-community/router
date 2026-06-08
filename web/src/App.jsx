import React, { Suspense, lazy, useCallback, useContext, useEffect, useRef, useState } from 'react';
import { Navigate, Route, Routes, useLocation, useNavigate } from 'react-router-dom';
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
import { WEB3_TOKEN_STORAGE_KEY } from './helpers/web3';
import { logoutWallet } from './services/web3Auth';
import { useWalletProviderStatus } from './hooks/useWalletProviderStatus';
import AdminLayout from './layouts/AdminLayout';
import UserLayout from './layouts/UserLayout';
import UserWorkspaceLayout from './layouts/UserWorkspaceLayout';
import Channel from './pages/Channel';
import EditChannel from './pages/Channel/EditChannel';
import AddChannel from './pages/Channel/AddChannel';
import User from './pages/User';
import UserDetail from './pages/User/EditUser';
import AddUser from './pages/User/AddUser';
import Log from './pages/Log';
import LogDetail from './pages/Log/Detail';
import Group from './pages/Group';
import Package from './pages/Package';
import PackageDetail from './pages/Package/Detail';
import Setting from './pages/Setting';
import Redemption from './pages/Redemption';
import EditRedemption from './pages/Redemption/EditRedemption';
import RedemptionDetail from './pages/Redemption/RedemptionDetail';
import AdminTopup from './pages/AdminTopup';
import AdminChannelTaskPage from './pages/Task/AdminChannelTaskPage';
import AdminChannelTaskDetailPage from './pages/Task/AdminChannelTaskDetailPage';
import AdminUserTaskPage from './pages/Task/AdminUserTaskPage';
import AdminUserTaskDetailPage from './pages/Task/AdminUserTaskDetailPage';
import WorkspaceTaskPage from './pages/Task/WorkspaceTaskPage';
import WorkspaceTaskDetailPage from './pages/Task/WorkspaceTaskDetailPage';
import FlowPage from './pages/Flow';
import TopupReconcileDetail from './pages/Flow/TopupReconcileDetail';
import TopupDetail from './pages/Flow/TopupDetail';
import PackageFlowDetail from './pages/Flow/PackageDetail';
import RedemptionFlowDetail from './pages/Flow/RedemptionDetail';
import AdminDashboard from './pages/AdminDashboard';
import AdminAlerts from './pages/AdminAlerts';
import Providers from './pages/Providers';

const RegisterForm = lazy(() => import('./components/RegisterForm'));
const LoginForm = lazy(() => import('./components/LoginForm'));
const PasswordResetForm = lazy(() => import('./components/PasswordResetForm'));
const PasswordResetConfirm = lazy(() => import('./components/PasswordResetConfirm'));
const Token = lazy(() => import('./pages/Token'));
const EditToken = lazy(() => import('./pages/Token/EditToken'));
const TopUp = lazy(() => import('./pages/TopUp'));
const TopUpOrderDetail = lazy(() => import('./pages/TopUp/TopUpOrderDetail'));
const Chat = lazy(() => import('./pages/Chat'));
const Dashboard = lazy(() => import('./pages/Dashboard'));
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
  const navigate = useNavigate();
  const location = useLocation();
  const walletDisconnectTimerRef = useRef(null);

  const clearWalletSession = useCallback(
    async (message) => {
      window.clearTimeout(walletDisconnectTimerRef.current);
      walletDisconnectTimerRef.current = null;
      try {
        await API.get('/api/v1/public/user/logout', {
          skipErrorHandler: true,
        });
      } catch (error) {
        // The local session must still be cleared if the server logout fails.
      }
      try {
        await logoutWallet();
      } catch (error) {
        // Ignore wallet SDK logout errors while clearing a stale session.
      }
      userDispatch({ type: 'logout' });
      localStorage.removeItem('user');
      localStorage.removeItem(WEB3_TOKEN_STORAGE_KEY);
      localStorage.removeItem('wallet_token_expires_at');
      if (message) {
        showNotice(message);
      }
      if (location.pathname !== '/login') {
        navigate('/login', { replace: true });
      }
    },
    [location.pathname, navigate, userDispatch],
  );

  const isWalletSessionActive = useCallback(() => {
    return Boolean(localStorage.getItem(WEB3_TOKEN_STORAGE_KEY));
  }, []);

  const getCurrentUserWalletAddress = useCallback(() => {
    try {
      const user = JSON.parse(localStorage.getItem('user') || '{}');
      return String(user?.wallet_address || '').trim().toLowerCase();
    } catch (error) {
      return '';
    }
  }, []);

  const handleWalletAccountsChanged = useCallback(
    (accounts) => {
      window.clearTimeout(walletDisconnectTimerRef.current);
      walletDisconnectTimerRef.current = null;
      if (!isWalletSessionActive()) {
        return;
      }
      const currentWalletAddress = getCurrentUserWalletAddress();
      const nextWalletAddress = String(accounts?.[0] || '').trim().toLowerCase();
      if (
        currentWalletAddress === '' ||
        nextWalletAddress === '' ||
        currentWalletAddress !== nextWalletAddress
      ) {
        clearWalletSession('钱包账户已变更，请重新登录').then();
      }
    },
    [clearWalletSession, getCurrentUserWalletAddress, isWalletSessionActive],
  );

  const handleWalletConnected = useCallback(() => {
    window.clearTimeout(walletDisconnectTimerRef.current);
    walletDisconnectTimerRef.current = null;
  }, []);

  const handleWalletDisconnected = useCallback(() => {
    if (!isWalletSessionActive() || walletDisconnectTimerRef.current) {
      return;
    }
    walletDisconnectTimerRef.current = window.setTimeout(() => {
      walletDisconnectTimerRef.current = null;
      if (isWalletSessionActive()) {
        clearWalletSession('钱包连接已断开，请重新登录').then();
      }
    }, 2200);
  }, [clearWalletSession, isWalletSessionActive]);

  useWalletProviderStatus({
    onAccountsChanged: handleWalletAccountsChanged,
    onConnect: handleWalletConnected,
    onDisconnect: handleWalletDisconnected,
  });

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

  useEffect(() => {
    return () => {
      window.clearTimeout(walletDisconnectTimerRef.current);
    };
  }, []);

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
              <WorkspaceTaskPage />
            </Suspense>
          }
        />
        <Route
          path='/workspace/task/:id'
          element={
            <Suspense fallback={<Loading />}>
              <WorkspaceTaskDetailPage />
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
          element={<Setting />}
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
          path='/admin/channel/tasks'
          element={<AdminChannelTaskPage />}
        />
        <Route
          path='/admin/channel/tasks/:id'
          element={<AdminChannelTaskDetailPage />}
        />
        <Route
          path='/admin/channel/edit/:id'
          element={<ChannelEditRedirect />}
        />
        <Route
          path='/admin/channel/detail/:id'
          element={<EditChannel />}
        />
        <Route
          path='/admin/channel/add'
          element={<AddChannel />}
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
          path='/admin/group/detail/:id'
          element={<Group />}
        />
        <Route
          path='/admin/package'
          element={<Package />}
        />
        <Route
          path='/admin/package/detail/:id'
          element={<PackageDetail />}
        />
        <Route
          path='/admin/topup'
          element={<AdminTopup />}
        />
        <Route
          path='/admin/flow/topup'
          element={<FlowPage kind='topup' />}
        />
        <Route
          path='/admin/flow/topup/:id'
          element={<TopupDetail />}
        />
        <Route
          path='/admin/flow/topup-reconcile'
          element={<FlowPage kind='topup-reconcile' />}
        />
        <Route
          path='/admin/flow/topup-reconcile/:id'
          element={<TopupReconcileDetail />}
        />
        <Route
          path='/admin/flow/package'
          element={<FlowPage kind='package' />}
        />
        <Route
          path='/admin/flow/package/:id'
          element={<PackageFlowDetail />}
        />
        <Route
          path='/admin/flow/redemption'
          element={<FlowPage kind='redemption' />}
        />
        <Route
          path='/admin/flow/redemption/:id'
          element={<RedemptionFlowDetail />}
        />
        <Route
          path='/admin/redemption'
          element={<Redemption />}
        />
        <Route
          path='/admin/redemption/edit/:id'
          element={<RedemptionEditRedirect />}
        />
        <Route
          path='/admin/redemption/:id'
          element={<RedemptionDetail />}
        />
        <Route
          path='/admin/redemption/add'
          element={<EditRedemption />}
        />
        <Route
          path='/admin/user'
          element={<User />}
        />
        <Route
          path='/admin/user/detail/:id'
          element={<UserDetail />}
        />
        <Route
          path='/admin/user/edit'
          element={<UserEditRedirect />}
        />
        <Route
          path='/admin/user/edit/:id'
          element={<UserEditRedirect />}
        />
        <Route
          path='/admin/user/add'
          element={<AddUser />}
        />
        <Route
          path='/admin/dashboard'
          element={<AdminDashboard />}
        />
        <Route
          path='/admin/alerts'
          element={<AdminAlerts />}
        />
        <Route
          path='/admin/log'
          element={<Log />}
        />
        <Route
          path='/admin/log/:id'
          element={<LogDetail />}
        />
        <Route
          path='/admin/task'
          element={<AdminUserTaskPage />}
        />
        <Route
          path='/admin/task/:id'
          element={<AdminUserTaskDetailPage />}
        />
        <Route
          path='/admin/setting'
          element={<Setting />}
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
