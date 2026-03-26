import { App as AntdApp, ConfigProvider, theme } from 'antd';
import { useEffect, useState } from 'react';
import { BrowserRouter, Route } from 'react-router';
import { Routes } from 'react-router';

import { useVersionCheck } from './shared/hooks/useVersionCheck';

import { IS_CLOUD, IS_PADDLE_SANDBOX, PADDLE_CLIENT_TOKEN } from './constants';
import { userApi } from './entity/users';
import { AuthPageComponent } from './pages/AuthPageComponent';
import { OAuthCallbackPage } from './pages/OAuthCallbackPage';
import { OauthStorageComponent } from './pages/OauthStorageComponent';
import { ThemeProvider, useTheme } from './shared/theme';
import { MainScreenComponent } from './widgets/main/MainScreenComponent';

function AppContent() {
  const [isAuthorized, setIsAuthorized] = useState(false);
  const { resolvedTheme } = useTheme();

  useVersionCheck();

  useEffect(() => {
    if (IS_CLOUD && PADDLE_CLIENT_TOKEN) {
      Paddle.Environment.set(IS_PADDLE_SANDBOX ? 'sandbox' : 'production');
      Paddle.Initialize({
        token: PADDLE_CLIENT_TOKEN,
        eventCallback: (event) => {
          window.dispatchEvent(new CustomEvent('paddle-event', { detail: event }));
        },
      });
    }
  }, []);

  useEffect(() => {
    const isAuthorized = userApi.isAuthorized();
    setIsAuthorized(isAuthorized);

    userApi.addAuthListener(() => {
      setIsAuthorized(userApi.isAuthorized());
    });
  }, []);

  return (
    <ConfigProvider
      theme={{
        algorithm: resolvedTheme === 'dark' ? theme.darkAlgorithm : theme.defaultAlgorithm,
        token: {
          colorPrimary: '#155dfc', // Tailwind blue-600
        },
      }}
    >
      <AntdApp>
        <BrowserRouter>
          <Routes>
            <Route path="/auth/callback" element={<OAuthCallbackPage />} />
            <Route path="/storages/google-oauth" element={<OauthStorageComponent />} />
            <Route
              path="/"
              element={!isAuthorized ? <AuthPageComponent /> : <MainScreenComponent />}
            />
          </Routes>
        </BrowserRouter>
      </AntdApp>
    </ConfigProvider>
  );
}

function App() {
  return (
    <ThemeProvider>
      <AppContent />
    </ThemeProvider>
  );
}

export default App;
