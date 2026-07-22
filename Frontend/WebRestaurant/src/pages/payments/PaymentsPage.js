/**
 * PaymentsPage.js
 * Container principal da pagina de pagamentos do WebRestaurant.
 * Gerencia as 5 abas: Dashboard, Aprovacoes, Estornos, Workflow, Responsabilidade.
 */
import React, { useState } from 'react';
import '../../styles/payments.css';
import PaymentDashboard from './PaymentDashboard';
import PaymentApprovals from './PaymentApprovals';
import PaymentChargebacks from './PaymentChargebacks';
import PaymentWorkflow from './PaymentWorkflow';
import PaymentResponsibility from './PaymentResponsibility';

/** Configuracao das abas disponiveis na pagina de pagamentos */
const TABS = [
  { key: 'dashboard', label: 'Dashboard', icon: '\u25C6' },
  { key: 'approvals', label: 'Aprovacoes', icon: '\u2713' },
  { key: 'chargebacks', label: 'Estornos', icon: '\u21A9' },
  { key: 'workflow', label: 'Workflow', icon: '\u26A1' },
  { key: 'responsibility', label: 'Responsabilidade', icon: '\uD83D\uDC65' },
];

/**
 * PaymentsPage — Componente container da pagina de pagamentos.
 * Renderiza barra de navegacao por abas e o conteudo da aba ativa.
 * @returns {JSX.Element} Pagina completa de pagamentos com 5 abas
 */
export default function PaymentsPage() {
  const [activeTab, setActiveTab] = useState('dashboard');

  /** Renderiza o componente da aba selecionada */
  const renderTab = () => {
    switch (activeTab) {
      case 'dashboard': return <PaymentDashboard />;
      case 'approvals': return <PaymentApprovals />;
      case 'chargebacks': return <PaymentChargebacks />;
      case 'workflow': return <PaymentWorkflow />;
      case 'responsibility': return <PaymentResponsibility />;
      default: return <PaymentDashboard />;
    }
  };

  return (
    <div>
      {/* Barra de navegacao por abas */}
      <div className="pp-tabs">
        {TABS.map((tab) => (
          <button
            key={tab.key}
            className={`pp-tab ${activeTab === tab.key ? 'active' : ''}`}
            onClick={() => setActiveTab(tab.key)}
          >
            {tab.icon} {tab.label}
          </button>
        ))}
      </div>
      {renderTab()}
    </div>
  );
}
