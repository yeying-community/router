import React from 'react';
import ReactDOM from 'react-dom/client';
import { BrowserRouter, useLocation } from 'react-router-dom';
import { Container } from 'semantic-ui-react';
import App from './App';
import Header from './components/Header';
import Footer from './components/Footer';
import 'semantic-ui-css/semantic.min.css';
import './index.css';
import { UserProvider } from './context/User';
import { ToastContainer } from 'react-toastify';
import 'react-toastify/dist/ReactToastify.css';
import { StatusProvider } from './context/Status';
import './i18n';

function AppShell() {
  const location = useLocation();
  const isLoginPage = location.pathname === '/login';

  return (
    <>
      {!isLoginPage && <Header />}
      {isLoginPage ? (
        <App />
      ) : (
        <Container className={'main-content'}>
          <App />
        </Container>
      )}
      <ToastContainer />
      {!isLoginPage && <Footer />}
    </>
  );
}

const root = ReactDOM.createRoot(document.getElementById('root'));
root.render(
  <React.StrictMode>
    <StatusProvider>
      <UserProvider>
        <BrowserRouter>
          <AppShell />
        </BrowserRouter>
      </UserProvider>
    </StatusProvider>
  </React.StrictMode>
);
