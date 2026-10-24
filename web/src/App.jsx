import React, {
  Suspense,
  lazy,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from 'react';
import {
  Navigate,
  Route,
  Routes,
  useLocation,
  useNavigate,
} from 'react-router-dom';
import i18n from './i18n.jsx';
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
import { applyChartThemeToDocument } from './router-ui';
import {
  resolveInitialThemeMode,
  applyThemeMode,
} from './router-ui/theme/store';
import { UserContext } from './context/User';
import { StatusContext } from './context/Status';
import { WEB3_TOKEN_STORAGE_KEY } from './helpers/web3';
import { buildLoginPath } from './helpers/authRedirect';
import { logoutWallet, restoreWalletSession } from './services/web3Auth';
import { useWalletProviderStatus } from './hooks/useWalletProviderStatus';
import { useCanManageUsers, useIsAdmin } from './hooks/useAuth';
import AdminLayout from './layouts/AdminLayout';
import UserLayout from './layouts/UserLayout';
import UserWorkspaceLayout from './layouts/UserWorkspaceLayout';
import ChannelLayout from './pages/Channel/ChannelLayout';
import EditChannel from './pages/Channel/EditChannel';
import AddChannel from './pages/Channel/AddChannel';
import UserPageLayout from './pages/User/UserLayout';
import UserDetail from './pages/User/EditUser';
import AddUser from './pages/User/AddUser';
import Log from './pages/Log';
import LogDetail from './pages/Log/Detail';
import Group from './pages/Group';
import PackageDetail from './pages/Package/Detail';
import WorkspaceSetting from './pages/Setting/Workspace';
import AdminSetting from './pages/Setting/Admin';
import RedemptionLayout from './pages/Redemption/RedemptionLayout';
import EditRedemption from './pages/Redemption/EditRedemption';
import RedemptionDetail from './pages/Redemption/RedemptionDetail';
import TopupPlanDetail from './pages/AdminTopup/Detail';
import EntitlementLayout from './pages/Entitlement/EntitlementLayout';
import AdminUserTaskDetailPage from './pages/Task/AdminUserTaskDetailPage';
import Task, { TASK_PAGE_KIND_WORKSPACE_USER } from './pages/Task';
import WorkspaceTaskDetailPage from './pages/Task/WorkspaceTaskDetailPage';
import PaymentRecordDetail from './pages/Records/PaymentRecordDetail';
import RedemptionRecordDetail from './pages/Records/RedemptionRecordDetail';
import AdminDashboard from './pages/AdminDashboard';
import Providers from './pages/Providers';
import FinanceLayout from './pages/BillingFinance/FinanceLayout';

const RegisterForm = lazy(() => import('./components/RegisterForm'));
const LoginForm = lazy(() => import('./components/LoginForm'));
const PasswordResetForm = lazy(() => import('./components/PasswordResetForm'));
const PasswordResetConfirm = lazy(() =>
  import('./components/PasswordResetConfirm')
);
const Token = lazy(() => import('./pages/Token'));
const EditToken = lazy(() => import('./pages/Token/EditToken'));
const AdminTokens = lazy(() => import('./pages/Token/AdminTokens'));
const TopUp = lazy(() => import('./pages/TopUp'));
const TopUpOrderDetail = lazy(() => import('./pages/TopUp/TopUpOrderDetail'));
const TopUpOrderReturn = lazy(() => import('./pages/TopUp/TopUpOrderReturn'));
const QuotaHistoryPage = lazy(() => import('./pages/TopUp/QuotaHistoryPage'));
const QuotaCardDetailPage = lazy(() =>
  import('./pages/TopUp/QuotaCardDetailPage')
);
const Chat = lazy(() => import('./pages/Chat'));
const ServicePricing = lazy(() => import('./pages/ServicePricing'));
const WorkspaceModels = lazy(() => import('./pages/WorkspaceModels'));
const HelpDoc = lazy(() => import('./pages/HelpDoc'));
const RouterGuideDoc = lazy(() => import('./pages/HelpDoc/RouterGuideDoc'));
const WorkspaceStart = lazy(() => import('./pages/WorkspaceStart'));
const PersonalRouting = lazy(() => import('./pages/PersonalRouting'));
const CommunityServices = lazy(() => import('./pages/CommunityServices'));

const APP_VERSION = import.meta.env.VITE_APP_VERSION || '';

function AdminOnlyRoute({ children }) {
  // Trust the in-memory UserContext (populated by /api/v1/user/self), not
  // localStorage. While UserContext is bootstrapping, show a Loading shell
  // instead of bouncing to /workspace/entry — otherwise a slow self-fetch
  // makes the admin layout look inaccessible.
  const [userState] = useContext(UserContext);
  const hasUser = Boolean(userState?.user);
  const isAdminUser = useIsAdmin();
  if (!hasUser) return <Loading />;
  if (!isAdminUser) return <Navigate to='/workspace/entry' replace />;
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
        const response = await API.get(
          '/api/v1/public/user/onboarding/progress'
        );
        const data = response?.data?.success
          ? response?.data?.data || null
          : null;
        const hasApiCall = !!data?.has_api_call;
        if (!active) {
          return;
        }
        setTargetPath(
          hasApiCall ? '/workspace/topup?tab=quota' : '/workspace/start'
        );
      } catch (error) {
        if (!active) {
          return;
        }
        // 网络/服务抖动时静默降级到主页,避免把登录用户红条 + 踢到新手页。
        // 401 由 api.jsx/showError 的统一分支自行处理跳转,这里不重复弹。
        setTargetPath('/workspace/topup?tab=quota');
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
          ? '/admin/setting?tab=basic&section=general'
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
      to={`/admin/redemption/${suffix}${search ? `?${search}` : ''}${
        location.hash
      }`}
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
      to={`/workspace/token/${suffix}${search ? `?${search}` : ''}${
        location.hash
      }`}
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

// Redirect a legacy standalone route onto a shell tab, merging the incoming
// query so deep-link params survive the move to `?tab=`.
function TabRedirect({ to, tab }) {
  const location = useLocation();
  const nextSearchParams = new URLSearchParams(location.search);
  if (tab) {
    nextSearchParams.set('tab', tab);
  }
  const search = nextSearchParams.toString();
  return (
    <Navigate
      to={`${to}${search ? `?${search}` : ''}${location.hash}`}
      state={location.state}
      replace
    />
  );
}

// Channel alerts are now operational signals inside the duty station rather
// than a second acknowledgement surface. Preserve old bookmarks and filters,
// but scope the destination to channel incidents explicitly.
function DutyAlertsRedirect() {
  const location = useLocation();
  const nextSearchParams = new URLSearchParams(location.search);
  const legacyStatus = (nextSearchParams.get('status') || '')
    .trim()
    .toLowerCase();
  nextSearchParams.delete('tab');
  nextSearchParams.set(
    'view',
    legacyStatus === 'resolved' ? 'resolved' : 'incidents'
  );
  nextSearchParams.set('domain', 'channels');
  return (
    <Navigate
      to={`/admin/dashboard?${nextSearchParams.toString()}${location.hash}`}
      state={location.state}
      replace
    />
  );
}

// Map the legacy cross-route finance URLs (`/admin/finance/overview|profit|
// procurement?…`) onto the single finance shell tab, merging any remaining
// query so drill-down params and `return_to` survive the redirect.
function FinanceTabRedirect() {
  const location = useLocation();
  const suffix = location.pathname.startsWith('/admin/finance/')
    ? location.pathname.slice('/admin/finance/'.length)
    : '';
  const tab = suffix.split('/')[0] || 'overview';
  const nextSearchParams = new URLSearchParams(location.search);
  nextSearchParams.set('tab', tab);
  const search = nextSearchParams.toString();
  return (
    <Navigate
      to={`/admin/finance${search ? `?${search}` : ''}${location.hash}`}
      state={location.state}
      replace
    />
  );
}

// The dashboard's former section switcher (`/admin/dashboard?section=…`) has
// been split into per-entity shells. Old deep links (bookmarks, external CTAs)
// redirect onto the matching shell tab, carrying any remaining query through;
// spending/overview/trend and the bare dashboard still render AdminDashboard.
const DASHBOARD_SECTION_TARGETS = {
  channels: { to: '/admin/channel', tab: 'health' },
  users: { to: '/admin/user', tab: 'analytics' },
  models: { to: '/workspace/service/models', tab: 'operations' },
};

function DashboardSectionRedirect() {
  const location = useLocation();
  const params = new URLSearchParams(location.search);
  const section = (params.get('section') || '').trim().toLowerCase();
  const target = DASHBOARD_SECTION_TARGETS[section];
  if (!target) {
    return <AdminDashboard />;
  }
  params.delete('section');
  params.set('tab', target.tab);
  const search = params.toString();
  return (
    <Navigate
      to={`${target.to}${search ? `?${search}` : ''}${location.hash}`}
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
  const walletSessionRecoveryRef = useRef(null);

  const clearWalletSession = useCallback(async () => {
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
    statusDispatch({ type: 'unset' });
    localStorage.removeItem('user');
    localStorage.removeItem(WEB3_TOKEN_STORAGE_KEY);
    localStorage.removeItem('wallet_token_expires_at');
    localStorage.removeItem('status');
    if (location.pathname !== '/login') {
      navigate(buildLoginPath(location), { replace: true });
    }
  }, [location, navigate, statusDispatch, userDispatch]);

  const isWalletSessionActive = useCallback(() => {
    return Boolean(localStorage.getItem(WEB3_TOKEN_STORAGE_KEY));
  }, []);

  const getCurrentUserWalletAddress = useCallback(() => {
    try {
      const user = JSON.parse(localStorage.getItem('user') || '{}');
      return String(user?.wallet_address || '')
        .trim()
        .toLowerCase();
    } catch (error) {
      return '';
    }
  }, []);

  const handleWalletConnected = useCallback(() => {
    window.clearTimeout(walletDisconnectTimerRef.current);
    walletDisconnectTimerRef.current = null;
  }, []);

  const recoverWalletSession = useCallback(async () => {
    if (!isWalletSessionActive()) {
      return false;
    }
    if (walletSessionRecoveryRef.current) {
      return walletSessionRecoveryRef.current;
    }
    const task = (async () => {
      try {
        const restored = await restoreWalletSession();
        if (!restored?.token) {
          return false;
        }
        try {
          const user = JSON.parse(localStorage.getItem('user') || '{}');
          if (user?.id) {
            userDispatch({
              type: 'login',
              payload: {
                ...user,
                token: restored.token,
              },
            });
          }
        } catch (error) {
          // The SDK token remains usable even if the legacy user cache is malformed.
        }
        return true;
      } catch (error) {
        return false;
      }
    })();
    walletSessionRecoveryRef.current = task;
    try {
      return await task;
    } finally {
      walletSessionRecoveryRef.current = null;
    }
  }, [isWalletSessionActive, userDispatch]);

  const handleWalletDisconnected = useCallback(() => {
    if (!isWalletSessionActive() || walletDisconnectTimerRef.current) {
      return;
    }
    walletDisconnectTimerRef.current = window.setTimeout(async () => {
      walletDisconnectTimerRef.current = null;
      const recovered = await recoverWalletSession();
      if (!recovered && isWalletSessionActive()) {
        clearWalletSession().then();
      }
    }, 2200);
  }, [clearWalletSession, isWalletSessionActive, recoverWalletSession]);

  const handleWalletAccountsChanged = useCallback(
    (accounts) => {
      window.clearTimeout(walletDisconnectTimerRef.current);
      walletDisconnectTimerRef.current = null;
      if (!isWalletSessionActive()) {
        return;
      }
      const currentWalletAddress = getCurrentUserWalletAddress();
      const nextWalletAddress = String(accounts?.[0] || '')
        .trim()
        .toLowerCase();
      if (nextWalletAddress === '') {
        handleWalletDisconnected();
        return;
      }
      if (
        currentWalletAddress === '' ||
        currentWalletAddress !== nextWalletAddress
      ) {
        clearWalletSession().then();
      }
    },
    [
      clearWalletSession,
      getCurrentUserWalletAddress,
      handleWalletDisconnected,
      isWalletSessionActive,
    ]
  );

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
            i18n.t('common.new_version_notice', { version: data.version })
          );
        }
      } else {
        showError(message || i18n.t('common.server_unreachable'));
      }
    } catch (error) {
      showError(error.message || i18n.t('common.server_unreachable'));
    }
  }, [statusDispatch]);

  useEffect(() => {
    // Resolve the active theme (stored > system > default) and commit it to
    // the DOM before any chart renders. The MutationObserver below then
    // re-applies chart CSS vars whenever the user (or system) flips the mode.
    applyThemeMode(resolveInitialThemeMode());
    applyChartThemeToDocument();
    if (typeof MutationObserver === 'undefined') return undefined;
    const observer = new MutationObserver(() => applyChartThemeToDocument());
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['data-theme', 'class'],
    });
    observer.observe(document.body, {
      attributes: true,
      attributeFilter: ['data-theme', 'class'],
    });
    return () => observer.disconnect();
  }, []);

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
    if (!isWalletSessionActive()) {
      return undefined;
    }
    const recover = () => {
      if (document.visibilityState && document.visibilityState !== 'visible') {
        return;
      }
      recoverWalletSession().then();
    };
    recover();
    window.addEventListener('focus', recover);
    document.addEventListener('visibilitychange', recover);
    const timer = window.setInterval(recover, 10 * 60 * 1000);
    return () => {
      window.removeEventListener('focus', recover);
      document.removeEventListener('visibilitychange', recover);
      window.clearInterval(timer);
    };
  }, [isWalletSessionActive, location.pathname, recoverWalletSession]);

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
      </Route>

      <Route
        element={
          <PrivateRoute>
            <UserWorkspaceLayout />
          </PrivateRoute>
        }
      >
        <Route
          path='/workspace/entry'
          element={<UserWorkspaceEntryRedirect />}
        />
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
          path='/workspace/channels'
          element={
            <Suspense fallback={<Loading />}>
              <PersonalRouting />
            </Suspense>
          }
        />
        <Route
          path='/workspace/personal-routing'
          element={<TabRedirect to='/workspace/channels' tab='channels' />}
        />
        <Route
          path='/workspace/publisher'
          element={<TabRedirect to='/workspace/channels' tab='supply' />}
        />
        <Route
          path='/workspace/community-services'
          element={
            <Suspense fallback={<Loading />}>
              <CommunityServices />
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
          path='/workspace/topup/return'
          element={
            <Suspense fallback={<Loading />}>
              <TopUpOrderReturn />
            </Suspense>
          }
        />
        <Route
          path='/workspace/topup/history'
          element={
            <Suspense fallback={<Loading />}>
              <QuotaHistoryPage />
            </Suspense>
          }
        />
        <Route
          path='/workspace/topup/cards/:kind/:id'
          element={
            <Suspense fallback={<Loading />}>
              <QuotaCardDetailPage />
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
              <Task pageKind={TASK_PAGE_KIND_WORKSPACE_USER} />
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
          element={<Navigate to='/workspace/topup?tab=quota' replace />}
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
          path='/workspace/service/pricing/history'
          element={
            <TabRedirect to='/workspace/service/pricing' tab='records' />
          }
        />
        <Route
          path='/workspace/service/models'
          element={
            <Suspense fallback={<Loading />}>
              <WorkspaceModels />
            </Suspense>
          }
        />
        <Route
          path='/workspace/service/router-guide'
          element={
            <Suspense fallback={<Loading />}>
              <RouterGuideDoc />
            </Suspense>
          }
        />
        <Route
          path='/workspace/service/cli-guide'
          element={
            <Suspense fallback={<Loading />}>
              <HelpDoc />
            </Suspense>
          }
        />
        <Route path='/workspace/setting' element={<WorkspaceSetting />} />
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
        <Route path='/admin/channel' element={<ChannelLayout />} />
        <Route
          path='/admin/channel/tasks'
          element={<TabRedirect to='/admin/channel' />}
        />
        <Route
          path='/admin/channel/tasks/:id'
          element={<TabRedirect to='/admin/channel' tab='health' />}
        />
        <Route
          path='/admin/channel/edit/:id'
          element={<ChannelEditRedirect />}
        />
        <Route path='/admin/channel/detail/:id' element={<EditChannel />} />
        <Route path='/admin/channel/add' element={<AddChannel />} />
        <Route path='/admin/provider' element={<Providers />} />
        <Route
          path='/admin/publisher'
          element={<TabRedirect to='/admin/channel' tab='supply' />}
        />
        <Route path='/admin/group' element={<Group />} />
        <Route path='/admin/group/detail/:id' element={<Group />} />
        <Route path='/admin/entitlement' element={<EntitlementLayout />} />
        <Route
          path='/admin/entitlement/package/detail/:id'
          element={<PackageDetail />}
        />
        <Route
          path='/admin/entitlement/payments'
          element={<TabRedirect to='/admin/entitlement' tab='records' />}
        />
        <Route
          path='/admin/entitlement/topup/detail/:id'
          element={<TopupPlanDetail />}
        />
        <Route
          path='/admin/entitlement/topup/payment/:id'
          element={<PaymentRecordDetail />}
        />
        <Route
          path='/admin/entitlement/payments/:id'
          element={<PaymentRecordDetail />}
        />
        <Route
          path='/admin/redemption/records'
          element={<TabRedirect to='/admin/redemption' tab='records' />}
        />
        <Route
          path='/admin/redemption/records/:id'
          element={<RedemptionRecordDetail />}
        />
        <Route
          path='/admin/user/detail/:id/payment/:paymentId'
          element={<PaymentRecordDetail />}
        />
        <Route path='/admin/redemption' element={<RedemptionLayout />} />
        <Route
          path='/admin/redemption/edit/:id'
          element={<RedemptionEditRedirect />}
        />
        <Route path='/admin/redemption/:id' element={<RedemptionDetail />} />
        <Route path='/admin/redemption/add' element={<EditRedemption />} />
        <Route path='/admin/user' element={<UserPageLayout />} />
        <Route path='/admin/user/detail/:id' element={<UserDetail />} />
        <Route path='/admin/user/edit' element={<UserEditRedirect />} />
        <Route path='/admin/user/edit/:id' element={<UserEditRedirect />} />
        <Route path='/admin/user/add' element={<AddUser />} />
        <Route path='/admin/dashboard' element={<DashboardSectionRedirect />} />
        <Route path='/admin/alerts' element={<DutyAlertsRedirect />} />
        <Route path='/admin/finance' element={<FinanceLayout />} />
        <Route path='/admin/finance/*' element={<FinanceTabRedirect />} />
        <Route path='/admin/token' element={<AdminTokens />} />
        <Route path='/admin/token/:id' element={<EditToken admin />} />
        <Route path='/admin/log' element={<Log />} />
        <Route path='/admin/log/:id' element={<LogDetail />} />
        <Route
          path='/admin/task'
          element={<TabRedirect to='/admin/user' tab='tasks' />}
        />
        <Route path='/admin/task/:id' element={<AdminUserTaskDetailPage />} />
        <Route path='/admin/setting' element={<AdminSetting />} />
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
