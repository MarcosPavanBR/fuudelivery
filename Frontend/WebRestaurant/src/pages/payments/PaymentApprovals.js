import React from 'react';
import '../../styles/payments.css';

const AUTO_RULES = [
  { label: 'Valor ate R$ 1.000', type: 'pass', result: 'Auto', color: 'var(--pp-accent)' },
  { label: 'Score de risco abaixo de 20', type: 'pass', result: 'Auto', color: 'var(--pp-accent)' },
  { label: 'Sem reclamacoes nos ultimos 30 dias', type: 'pass', result: 'Auto', color: 'var(--pp-accent)' },
  { label: 'Pedido confirmado pelo cliente', type: 'pass', result: 'Auto', color: 'var(--pp-accent)' },
];

const MANUAL_RULES = [
  { label: 'Valor acima de R$ 5.000', type: 'fail', result: 'Manual', color: 'var(--pp-danger)' },
  { label: 'Score de risco acima de 60', type: 'fail', result: 'Compliance', color: 'var(--pp-danger)' },
  { label: 'Reclamacao ou estorno ativo', type: 'fail', result: 'Bloqueado', color: 'var(--pp-danger)' },
  { label: 'Multiplos saques em 24h', type: 'fail', result: 'Compliance', color: 'var(--pp-danger)' },
];

function VerifyCard({ title, titleColor, rules }) {
  return (
    <div className="pp-verify-card">
      <h3><span style={{ color: titleColor }}>{'\u25CF'}</span>{title}</h3>
      {rules.map((r, i) => (
        <div key={i} className="pp-verify-item">
          <div className="pp-verify-left">
            <div className={`pp-verify-icon ${r.type}`}>{r.type === 'pass' ? '\u2713' : '\u2715'}</div>
            <span className="pp-verify-label">{r.label}</span>
          </div>
          <span className="pp-verify-value" style={{ color: r.color }}>{r.result}</span>
        </div>
      ))}
    </div>
  );
}

export default function PaymentApprovals() {
  return (
    <div className="pp-panel">
      <div className="pp-section-header">
        <div className="pp-section-title">
          <div className="dot" style={{ background: 'var(--pp-accent)' }} />
          Fila de Aprovacoes
        </div>
        <div className="pp-filter-group">
          {['Pendentes (7)', 'Aprovados (43)', 'Rejeitados (2)', 'Auto-Lib. (128)'].map((f, i) => (
            <button key={i} className={`pp-filter-btn ${i === 0 ? 'active' : ''}`}>{f}</button>
          ))}
        </div>
      </div>
      <div className="pp-verify-grid">
        <VerifyCard title="Regras de Auto-Aprovacao" titleColor="var(--pp-accent)" rules={AUTO_RULES} />
        <VerifyCard title="Gatilhos para Analise Manual" titleColor="var(--pp-danger)" rules={MANUAL_RULES} />
      </div>
    </div>
  );
}
