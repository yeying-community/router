import { Navigate, useLocation } from 'react-router-dom';
import { buildLoginPath } from '../helpers/authRedirect';

function PrivateRoute({ children }) {
  const location = useLocation();
  if (!localStorage.getItem('user')) {
    return <Navigate to={buildLoginPath(location)} replace />;
  }
  return children;
}

export { PrivateRoute };
