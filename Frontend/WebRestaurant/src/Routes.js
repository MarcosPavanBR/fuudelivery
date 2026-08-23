import React, { Suspense, lazy } from "react";
import { useAuth } from "./context/AuthContext";
import LoginPage from "./pages/login";
import Home from "./pages/home";
import { Routes as ReactRoutes, Route, useLocation } from "react-router-dom";

// Code splitting: cada página vira um chunk separado, carregado on-demand.
// Login e Home ficam eager (primeira renderização); o resto é pesado
// (kanban/dnd, gráficos, carteira) e só carrega quando a rota é visitada.
const Cardapio = lazy(() => import("./pages/cardapio/products/Cardapio"));
const Perfil = lazy(() => import("./pages/perfil"));
const Taxes = lazy(() => import("./pages/perfil/taxes"));
const ChangePassword = lazy(() => import("./pages/perfil/password"));
const MinhaCarteira = lazy(() => import("./pages/wallet/MinhaCarteira"));
const RegisterEstablishment = lazy(
  () => import("./pages/registration/RegisterEstablishment")
);
const Reports = lazy(() => import("./pages/reports/Reports"));

const RouteFallback = () => (
  <div className="flex h-screen w-full items-center justify-center">
    <div className="animate-pulse text-sm text-gray-500">Carregando…</div>
  </div>
);

export default function PrivateRoute() {
  const { user } = useAuth();
  const location = useLocation();

  // Rotas públicas (acessíveis sem login)
  const publicRoutes = ["/cadastrar-restaurante"];

  // Se não está autenticado e não é rota pública, mostra login
  if (!user && !publicRoutes.includes(location.pathname)) {
    return <LoginPage />;
  }

  return (
    <ReactRoutes location={location}>
      {/* Rotas públicas */}
      <Route
        path="/cadastrar-restaurante"
        element={
          <Suspense fallback={<RouteFallback />}>
            <RegisterEstablishment />
          </Suspense>
        }
      />

      {/* Rotas autenticadas */}
      <Route path="/" element={<Home />} />
      <Route
        path="/gestor-cardapio"
        element={
          <Suspense fallback={<RouteFallback />}>
            <Cardapio />
          </Suspense>
        }
      />
      <Route
        path="/perfil"
        element={
          <Suspense fallback={<RouteFallback />}>
            <Perfil />
          </Suspense>
        }
      />
      <Route
        path="/carteira"
        element={
          <Suspense fallback={<RouteFallback />}>
            <MinhaCarteira />
          </Suspense>
        }
      />
      <Route
        path="/taxas"
        element={
          <Suspense fallback={<RouteFallback />}>
            <Taxes />
          </Suspense>
        }
      />
      <Route
        path="/alterar-senha"
        element={
          <Suspense fallback={<RouteFallback />}>
            <ChangePassword />
          </Suspense>
        }
      />
      <Route
        path="/relatorios"
        element={
          <Suspense fallback={<RouteFallback />}>
            <Reports />
          </Suspense>
        }
      />
    </ReactRoutes>
  );
}
