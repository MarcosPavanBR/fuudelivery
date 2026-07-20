import Routes from "./Routes";
import { AuthProvider } from "./context/AuthContext";
import "./index.css";
import React from "react";
import { HashRouter } from "react-router-dom";
import { ToastContainer } from "react-toastify";
import "react-toastify/dist/ReactToastify.css";

class ErrorBoundary extends React.Component {
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
          <h1 style={{ color: "#EA1D2C" }}>Erro na aplicação</h1>
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

const App = () => {
  return (
    <ErrorBoundary>
      <HashRouter>
        <AuthProvider>
          <Routes />
          <ToastContainer />
        </AuthProvider>
      </HashRouter>
    </ErrorBoundary>
  );
};

export default App;
