import React from "react";
import { useAuth } from "./context/AuthContext";
import LoginPage from "./pages/login";
import Home from "./pages/home";
import { Routes as ReactRoutes, Route } from "react-router-dom";
import Cardapio from "./pages/cardapio/products/Cardapio";
import Perfil from "./pages/perfil";
import Taxes from "./pages/perfil/taxes";
import ChangePassword from "./pages/perfil/password";
import WalletPage from "./pages/perfil/wallet";

export default function PrivateRoute() {
  const { user } = useAuth();

  if (!user) return <LoginPage />;

  return (
    <ReactRoutes>
      <Route path="/" element={<Home />} />
      <Route path="/gestor-cardapio" element={<Cardapio />} />
      <Route path="/perfil" element={<Perfil />} />
      <Route path="/carteira" element={<WalletPage />} />
      <Route path="/taxas" element={<Taxes />} />
      <Route path="/alterar-senha" element={<ChangePassword />} />
    </ReactRoutes>
  );
}
