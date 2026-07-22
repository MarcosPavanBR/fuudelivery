import React from 'react';
import '../../styles/payments.css';

const CHARGEBACKS = [
  { id: '#DSP-00231', order: '#ORD-8801', amount: 1230, reason: 'Pedido nao entregue', responsible: 'Entregador', evidenceCount: 3, status: 'investigating' },
  { id: '#DSP-00232', order: '#ORD-8790', amount: 680, reason: 'Item faltante', responsible: 'Restaurante', evidenceCount: 1, status: 'awaiting' },
  { id: '#DSP-00233', order: '#ORD-8785', amount: 430, reason: 'Produto errado', responsible: 'Restaurante', evidenceCount: 5, status: 'investigating' },
];

function StatusBadge({ status, label }) {
  const MAP = {
    PENDING: { label: 'Pendente', cls: 'pending' },
    COMPLIANCE: { label: 'Compliance', cls: 'review' },
    approved: { label: 'Aprovado', cls: 'approved' },
    investigating: { label: 'Investigando', cls: 'pending' },
    awaiting: { label: 'Aguardando', cls: 'review' },
  };
  const config = MAP[status] || { label: status, cls: 'pending' };
  return <span className={`pp-badge ${config.cls}`}><span className="pp-badge-dot" />{label || config.label}</span>;
}

function RiskBadge({ score }) {
  let level = 'low';
  if (score >= 90) level = 'critical';
  else if (score >= 60) level = 'high';
  else if (score >= 20) level = 'medium';
  return <span className={`pp-risk ${level}`}>{score}</span>;
}

export default function PaymentChargebacks() {
  return (
    <div className="pp-panel">
      <div className="pp-section-header">
        <div className="pp-section-title">
          <div className="dot" style={{ background: 'var(--pp-danger)' }} />
          Estornos & Disputas
        </div>
        <div className="pp-filter-group">
          {['Ativos (3)', 'Resolvidos', 'Contestados'].map((f, i) => (
            <button key={i} className={`pp-filter-btn ${i === 0 ? 'active' : ''}`}>{f}</button>
          ))}
        </div>
      </div>

      <div className="pp-cb-cards">
        <div className="pp-cb-card">
          <div className="pp-cb-header">
            <span className="pp-cb-title">Estornos Pendentes</span>
            <RiskBadge score={65} />
          </div>
          <div className="pp-cb-amount" style={{ color: 'var(--pp-danger)' }}>R$ 2.340,00</div>
          <div className="pp-cb-sub">Em processo de verificacao</div>
          <div className="pp-cb-bar"><div className="pp-cb-bar-fill danger" style={{ width: '45%' }} /></div>
        </div>
        <div className="pp-cb-card">
          <div className="pp-cb-header">
            <span className="pp-cb-title">Contestados com Sucesso</span>
            <RiskBadge score={12} />
          </div>
          <div className="pp-cb-amount" style={{ color: 'var(--pp-accent)' }}>R$ 8.920,00</div>
          <div className="pp-cb-sub">Ultimos 30 dias</div>
          <div className="pp-cb-bar"><div className="pp-cb-bar-fill ok" style={{ width: '78%' }} /></div>
        </div>
        <div className="pp-cb-card">
          <div className="pp-cb-header">
            <span className="pp-cb-title">Taxa de Estorno</span>
            <span className="pp-risk medium">1.8%</span>
          </div>
          <div className="pp-cb-amount" style={{ color: 'var(--pp-warning)' }}>1.8%</div>
          <div className="pp-cb-sub">Meta: abaixo de 1%</div>
          <div className="pp-cb-bar"><div className="pp-cb-bar-fill warning" style={{ width: '60%' }} /></div>
        </div>
      </div>

      <div className="pp-table-wrap">
        <div className="pp-table-toolbar">
          <input type="text" className="pp-search-box" placeholder="Buscar disputa..." />
        </div>
        <table>
          <thead>
            <tr>
              <th>ID Disputa</th><th>Pedido</th><th>Valor</th><th>Motivo</th><th>Responsavel</th><th>Evidencias</th><th>Status</th><th>Acoes</th>
            </tr>
          </thead>
          <tbody>
            {CHARGEBACKS.map((cb) => (
              <tr key={cb.id}>
                <td className="pp-mono">{cb.id}</td>
                <td className="pp-mono">{cb.order}</td>
                <td className="pp-amount" style={{ color: 'var(--pp-danger)' }}>R$ {cb.amount.toLocaleString('pt-BR', { minimumFractionDigits: 2 })}</td>
                <td>{cb.reason}</td>
                <td><StatusBadge status={cb.status} label={cb.responsible} /></td>
                <td><span style={{ color: 'var(--pp-accent)', fontSize: 12 }}>{cb.evidenceCount} anexos</span></td>
                <td><StatusBadge status={cb.status} /></td>
                <td>
                  <div className="pp-actions">
                    <button className="pp-btn view">{'\u25CE'}</button>
                    <button className="pp-btn approve">{'\u2713'}</button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
