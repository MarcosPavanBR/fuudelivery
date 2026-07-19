import Routes from "./Routes";
import { AuthProvider } from "./context/AuthContext";
import "./index.css";
import React from "react";
import { HashRouter } from "react-router-dom";
import { ToastContainer } from "react-toastify";
import "react-toastify/dist/ReactToastify.css";

const App = () => {
  return (
    <HashRouter>
      <AuthProvider>
        <Routes />
        <ToastContainer />
      </AuthProvider>
    </HashRouter>
  );
};

export default App;
