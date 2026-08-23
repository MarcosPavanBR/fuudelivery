import React, { Component, Suspense, lazy } from "react";
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import { AuthProvider, useAuth } from "./context/AuthContext";
// Code splitting: páginas pesadas carregam sob demanda (React.lazy).
// Login e Dashboard ficam eager — são o primeiro paint e o destino pós-login.
import Login from "./pages/Login.jsx";
import Dashboard from "./pages/Dashboard.jsx";
const Establishments = lazy(() => import("./pages/Establishments.jsx"));
const Users = lazy(() => import("./pages/Users.jsx"));
const Orders = lazy(() => import("./pages/Orders.jsx"));
const DeliveryMen = lazy(() => import("./pages/DeliveryMen.jsx"));
const Payments = lazy(() => import("./pages/Payments.jsx"));
const Financeiro = lazy(() => import("./pages/Financeiro.jsx"));
const Settings = lazy(() => import("./pages/Settings.jsx"));
import Layout from "./components/Layout.jsx";
import { FiLoader } from "react-icons/fi";
import { ToastContainer } from "react-toastify";
import "react-toastify/dist/ReactToastify.css";

class ErrorBoundary extends Component {
  constructor(props) {
    super(props);
    this.state = { hasError: false, error: null };
  }
  static getDerivedStateFromError(error) {
    return { hasError: true, error };
  }
  render() {
    if (this.state.hasError) {
      return (
        <div style={{ padding: 40, fontFamily: "monospace", background: "#fff", minHeight: "100vh" }}>
          <h1 style={{ color: "#DC2626" }}>Erro na aplicação</h1>
          <pre style={{ whiteSpace: "pre-wrap", marginTop: 16, color: "#333" }}>
            {this.state.error?.message}
          </pre>
          <pre style={{ whiteSpace: "pre-wrap", marginTop: 8, color: "#666", fontSize: 12 }}>
            {this.state.error?.stack}
          </pre>
        </div>
      );
    }
    return this.props.children;
  }
}

const ProtectedRoute = ({ children }) => {
  const { user, loading } = useAuth();

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <FiLoader className="animate-spin h-8 w-8" style={{ color: "#DC2626" }} />
      </div>
    );
  }

  if (!user) {
    return <Navigate to="/login" replace />;
  }

  return children;
};

function AppRoutes() {
  const { user, loading } = useAuth();

  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-gray-50">
        <FiLoader className="animate-spin h-8 w-8" style={{ color: "#DC2626" }} />
      </div>
    );
  }

  return (
    <Routes>
      <Route path="/login" element={user ? <Navigate to="/" replace /> : <Login />} />
      <Route
        element={
          <ProtectedRoute>
            <Layout />
          </ProtectedRoute>
        }
      >
        <Route path="/" element={<Dashboard />} />
        {/* Páginas lazy: o Suspense do App() exibe spinner durante o fetch do chunk */}
        <Route path="/establishments" element={<Establishments />} />
        <Route path="/users" element={<Users />} />
        <Route path="/orders" element={<Orders />} />
        <Route path="/delivery-men" element={<DeliveryMen />} />
        <Route path="/payments" element={<Payments />} />
        <Route path="/financeiro" element={<Financeiro />} />
        <Route path="/settings" element={<Settings />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

export default function App() {
  return (
    <ErrorBoundary>
      <BrowserRouter>
        <AuthProvider>
          {/* Suspense cobre as rotas React.lazy — fallback consistente com o loading inicial */}
          <Suspense
            fallback={
              <div className="min-h-screen flex items-center justify-center bg-gray-50">
                <FiLoader className="animate-spin h-8 w-8" style={{ color: "#DC2626" }} />
              </div>
            }
          >
            <AppRoutes />
          </Suspense>
          <ToastContainer position="top-right" autoClose={3000} />
        </AuthProvider>
      </BrowserRouter>
    </ErrorBoundary>
  );
}
