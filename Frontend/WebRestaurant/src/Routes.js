import React from "react";
import { useAuth } from "./context/AuthContext";
import LoginPage from "./pages/login";
import Home from "./pages/home";
import { Routes as ReactRoutes, Route } from "react-router-dom";
import Cardapio from "./pages/cardapio/products/Cardapio";
import Perfil from "./pages/perfil";
import Taxes from "./pages/perfil/taxes";
import ChangePassword from "./pages/perfil/password";
import MinhaCarteira from "./pages/wallet/MinhaCarteira";
import RegisterEstablishment from "./pages/registration/RegisterEstablishment";
import Reports from "./pages/reports/Reports";

export default function PrivateRoute() {
  const { user } = useAuth();

  // Rotas públicas (acessíveis sem login)
  const publicRoutes = ["/cadastrar-restaurante"];

  // Se não está autenticado e não é rota pública, mostra login
  if (!user && !publicRoutes.includes(window.location.hash.replace("#", ""))) {
    return <LoginPage />;
  }

  return (
    <ReactRoutes>
      {/* Rotas públicas */}
      <Route path="/cadastrar-restaurante" element={<RegisterEstablishment />} />

      {/* Rotas autenticadas */}
      <Route path="/" element={<Home />} />
      <Route path="/gestor-cardapio" element={<Cardapio />} />
      <Route path="/perfil" element={<Perfil />} />
      <Route path="/carteira" element={<MinhaCarteira />} />
      <Route path="/taxas" element={<Taxes />} />
      <Route path="/alterar-senha" element={<ChangePassword />} />
      <Route path="/relatorios" element={<Reports />} />
    </ReactRoutes>
  );
}
