import React, { useState } from 'react';
import '../../styles/payments.css';

const MOCK_GATEWAYS = [
  { name: 'AbacatePay', status: 'online' },
  { name: 'PIX Gateway', status: 'online' },
  { name: 'Asaas', status: 'online' },
];

const MOCK_METRICS = [
  { label: 'Pendentes', value: 7, sub: 'R$ 12.847,50 total', color: 'yellow' },
  { label: 'Aprovados Hoje', value: 43, sub: 'vs ontem', color: 'green', trend: '\u2191 12%', trendDir: 'up' },
  { label: 'Em Analise', value: 3, sub: 'Alto risco', color: 'red' },
  { label: 'Estornos Ativos', value: 3, sub: 'R$ 2.340,00 em disputa', color: 'blue' },
  { label: 'Auto-Aprovados', value: 128, sub: '89% do volume hoje', color: 'purple' },
];

const MOCK_PAYMENTS = [
  { id: '#PAY-04891', type: 'Saque', beneficiary: 'Restaurante Sabor da Terra', amount: 4230, order: '#ORD-8847', risk: 47, chain: { restaurant: 'ok', driver: 'ok', customer: 'warn' }, status: 'PENDING' },
  { id: '#PAY-04892', type: 'Repasse', beneficiary: 'Entregador - Carlos M.', amount: 380.5, order: '#ORD-8851', risk: 12, chain: { restaurant: 'ok', driver: 'ok', customer: 'ok' }, status: 'PENDING' },
  { id: '#PAY-04893', type: 'Saque', beneficiary: 'Restaurante Burger King', amount: 18750, order: '#ORD-8839', risk: 78, chain: { restaurant: 'ok', driver: 'err', customer: 'warn' }, status: 'COMPLIANCE' },
  { id: '#PAY-04894', type: 'Auto', beneficiary: 'Restaurante Pizza Express', amount: 890, order: '#ORD-8855', risk: 8, chain: { restaurant: 'ok', driver: 'ok', customer: 'ok' }, status: 'AUTO_APPROVED' },
  { id: '#PAY-04895', type: 'Repasse', beneficiary: 'Entregador - Ana P.', amount: 520.75, order: '#ORD-8860', risk: 5, chain: { restaurant: 'ok', driver: 'ok', customer: 'ok' }, status: 'AUTO_APPROVED' },
  { id: '#PAY-04896', type: 'Saque', beneficiary: 'Restaurante Sushi Yama', amount: 22100, order: '#ORD-8844', risk: 92, chain: { restaurant: 'err', driver: 'warn', customer: 'err' }, status: 'COMPLIANCE' },
  { id: '#PAY-04897', type: 'Saque', beneficiary: 'Restaurante Acai Premium', amount: 3150, order: '#ORD-8862', risk: 35, chain: { restaurant: 'ok', driver: 'ok', customer: 'warn' }, status: 'PENDING' },
];

const MOCK_DETAIL = {
  id: '#PAY-04891',
  beneficiary: 'Restaurante Sabor da Terra',
  amount: 'R$ 4.230,00',
  type: 'Saque (Saldo Acumulado)',
  gateway: 'PIX \u2014 Banco Inter',
  order: '#ORD-8847',
  riskBadge: '47 \u2014 Medio',
  orderStatus: '\u2713 Entregue com sucesso',
  deliveryProof: '\u2713 Foto + GPS registrados',
  complaints: '1 reclamacao (resolvida)',
  chargebacks: '\u2713 Nenhum estorno',
  respRestaurant: '\u2713 Pedido preparado corretamente',
  respDriver: '\u2713 Entrega confirmada (GPS + foto)',
  respCustomer: '\u26A0 Reclamou de atraso (resolvido)',
  verdict: 'Aprovavel \u2014 Reclamacao nao procede',
  timeline: [
    { time: '2026-07-20 14:32', text: 'Pedido #ORD-8847 criado \u2014 R$ 145,90', status: 'ok' },
    { time: '2026-07-20 14:35', text: 'Pagamento capturado no gateway (PIX)', status: 'ok' },
    { time: '2026-07-20 15:02', text: 'Pedido entregue \u2014 GPS confirmado, foto registrada', status: 'ok' },
    { time: '2026-07-20 18:15', text: 'Cliente abriu reclamacao: "Atraso de 20min"', status: 'warn' },
    { time: '2026-07-21 09:30', text: 'Reclamacao resolvida \u2014 Transito intenso comprovado', status: 'ok' },
    { time: '2026-07-22 10:00', text: 'Solicitacao de saque criada \u2014 R$ 4.230,00', status: 'ok' },
    { time: '2026-07-22 10:00', text: 'Enviado para analise manual \u2014 Valor > R$ 1.000', status: 'warn' },
  ],
};

function RiskBadge({ score }) {
  let level = 'low';
  if (score >= 90) level = 'critical';
  else if (score >= 60) level = 'high';
  else if (score >= 20) level = 'medium';
  return <span className={`pp-risk ${level}`}>{score}</span>;
}

function ChainDots({ chain }) {
  const cls = (s) => s === 'ok' ? 'ok' : s === 'warn' ? 'warn' : 'err';
  return (
    <div className="pp-chain">
      <div className={`pp-chain-dot ${cls(chain.restaurant)}`} title="Restaurante">R</div>
      <span className="pp-chain-arrow">\u2192</span>
      <div className={`pp-chain-dot ${cls(chain.driver)}`} title="Entregador">E</div>
      <span className="pp-chain-arrow">\u2192</span>
      <div className={`pp-chain-dot ${cls(chain.customer)}`} title="Cliente">C</div>
    </div>
  );
}

function StatusBadge({ status }) {
  const MAP = {
    PENDING: { label: 'Pendente', cls: 'pending' },
    AUTO_APPROVED: { label: 'Auto-Lib.', cls: 'auto' },
    COMPLIANCE: { label: 'Compliance', cls: 'review' },
    APPROVED: { label: 'Aprovado', cls: 'approved' },
    REJECTED: { label: 'Rejeitado', cls: 'rejected' },
  };
  const config = MAP[status] || { label: status, cls: 'pending' };
  return <span className={`pp-badge ${config.cls}`}><span className="pp-badge-dot" />{config.label}</span>;
}

function PaymentModal({ detail, onClose, onApprove, onReject }) {
  return (
    <div className="pp-modal-overlay" onClick={(e) => e.target === e.currentTarget && onClose()}>
      <div className="pp-modal">
        <div className="pp-modal-header">
          <h2>Detalhes da Solicitacao <span className="pp-mono" style={{ color: 'var(--pp-accent)' }}>{detail.id}</span></h2>
          <button className="pp-modal-close" onClick={onClose}>\u2715</button>
        </div>
        <div className="pp-modal-body">
          <div className="pp-modal-section">
            <div className="pp-modal-section-title">Informacoes do Pagamento</div>
            <div className="pp-detail-grid">
              <div className="pp-detail-item"><div className="label">Beneficiario</div><div className="value">{detail.beneficiary}</div></div>
              <div className="pp-detail-item"><div className="label">Valor Solicitado</div><div className="value" style={{ color: 'var(--pp-warning)', fontFamily: "'JetBrains Mono', monospace" }}>{detail.amount}</div></div>
              <div className="pp-detail-item"><div className="label">Tipo</div><div className="value">{detail.type}</div></div>
              <div className="pp-detail-item"><div className="label">Gateway</div><div className="value" style={{ fontFamily: "'JetBrains Mono', monospace" }}>{detail.gateway}</div></div>
              <div className="pp-detail-item"><div className="label">Pedido Vinculado</div><div className="value" style={{ fontFamily: "'JetBrains Mono', monospace" }}>{detail.order}</div></div>
              <div className="pp-detail-item"><div className="label">Score de Risco</div><div className="value">{detail.riskBadge}</div></div>
            </div>
          </div>
          <div className="pp-modal-section">
            <div className="pp-modal-section-title">Verificacao do Pedido</div>
            <div className="pp-detail-grid">
              <div className="pp-detail-item"><div className="label">Status do Pedido</div><div className="value" style={{ color: 'var(--pp-accent)' }}>{detail.orderStatus}</div></div>
              <div className="pp-detail-item"><div className="label">Comprovante de Entrega</div><div className="value" style={{ color: 'var(--pp-accent)' }}>{detail.deliveryProof}</div></div>
              <div className="pp-detail-item"><div className="label">Reclamacoes</div><div className="value" style={{ color: 'var(--pp-warning)' }}>{detail.complaints}</div></div>
              <div className="pp-detail-item"><div className="label">Estornos</div><div className="value" style={{ color: 'var(--pp-accent)' }}>{detail.chargebacks}</div></div>
            </div>
          </div>
          <div className="pp-modal-section">
            <div className="pp-modal-section-title">Cadeia de Responsabilidade</div>
            <div className="pp-detail-grid">
              <div className="pp-detail-item"><div className="label">Restaurante</div><div className="value" style={{ color: 'var(--pp-accent)' }}>{detail.respRestaurant}</div></div>
              <div className="pp-detail-item"><div className="label">Entregador</div><div className="value" style={{ color: 'var(--pp-accent)' }}>{detail.respDriver}</div></div>
              <div className="pp-detail-item"><div className="label">Cliente</div><div className="value" style={{ color: 'var(--pp-warning)' }}>{detail.respCustomer}</div></div>
              <div className="pp-detail-item"><div className="label">Veredicto</div><div className="value" style={{ color: 'var(--pp-accent)' }}>{detail.verdict}</div></div>
            </div>
          </div>
          <div className="pp-modal-section">
            <div className="pp-modal-section-title">Timeline</div>
            <div className="pp-timeline">
              {detail.timeline.map((evt, i) => (
                <div key={i} className={`pp-tl-item ${evt.status}`}>
                  <div className="pp-tl-time">{evt.time}</div>
                  <div className="pp-tl-text">{evt.text}</div>
                </div>
              ))}
            </div>
          </div>
        </div>
        <div className="pp-modal-footer">
          <button className="pp-modal-btn" onClick={onClose}>Fechar</button>
          <button className="pp-modal-btn danger" onClick={() => { onReject(); onClose(); }}>\u2715 Rejeitar</button>
          <button className="pp-modal-btn success" onClick={() => { onApprove(); onClose(); }}>\u2713 Aprovar Pagamento</button>
        </div>
      </div>
    </div>
  );
}

export default function PaymentDashboard() {
  const [payments, setPayments] = useState(MOCK_PAYMENTS);
  const [modalOpen, setModalOpen] = useState(false);
  const [toasts, setToasts] = useState([]);
  const [filter, setFilter] = useState('all');
  const [search, setSearch] = useState('');

  const showToast = (type, message) => {
    const id = Date.now();
    setToasts((prev) => [...prev, { id, type, message }]);
    setTimeout(() => setToasts((prev) => prev.filter((t) => t.id !== id)), 3000);
  };

  const handleApprove = (id) => {
    showToast('success', `Pagamento ${id} APROVADO`);
    setPayments((prev) => prev.filter((p) => p.id !== id));
  };

  const handleReject = (id) => {
    showToast('error', `Pagamento ${id} REJEITADO`);
    setPayments((prev) => prev.filter((p) => p.id !== id));
  };

  const filtered = payments.filter((p) => {
    if (filter === 'high-risk' && p.risk < 60) return false;
    if (filter === 'high-amount' && p.amount < 5000) return false;
    if (search) {
      const q = search.toLowerCase();
      return p.id.toLowerCase().includes(q) || p.beneficiary.toLowerCase().includes(q) || p.order.toLowerCase().includes(q);
    }
    return true;
  });

  return (
    <div className="pp-panel">
      <div className="pp-gateway-strip">
        {MOCK_GATEWAYS.map((gw, i) => (
          <div key={i} className="pp-gw-item">
            <div className={`pp-gw-dot ${gw.status}`} />{gw.name}
          </div>
        ))}
        <div className="pp-gw-divider" />
        <div className="pp-gw-sync">Ultima sync: 12s atras</div>
      </div>

      <div className="pp-metrics-row">
        {MOCK_METRICS.map((m, i) => (
          <div key={i} className={`pp-metric-card ${m.color}`}>
            <div className="pp-metric-label">{m.label}</div>
            <div className="pp-metric-value">{m.value}</div>
            <div className="pp-metric-sub">
              {m.trend && <span className={m.trendDir === 'up' ? 'up' : 'down'}>{m.trend}</span>}{' '}{m.sub}
            </div>
          </div>
        ))}
      </div>

      <div className="pp-section-header">
        <div className="pp-section-title">
          <div className="dot" style={{ background: 'var(--pp-warning)' }} />
          Solicitacoes Pendentes de Aprovacao
        </div>
        <div className="pp-filter-group">
          {[{ key: 'all', label: 'Todos' }, { key: 'high-risk', label: 'Alto Risco' }, { key: 'high-amount', label: 'Alto Valor' }].map((f) => (
            <button key={f.key} className={`pp-filter-btn ${filter === f.key ? 'active' : ''}`} onClick={() => setFilter(f.key)}>{f.label}</button>
          ))}
        </div>
      </div>

      <div className="pp-table-wrap">
        <div className="pp-table-toolbar">
          <input type="text" className="pp-search-box" placeholder="Buscar por ID, parceiro, valor..." value={search} onChange={(e) => setSearch(e.target.value)} />
          <div className="pp-filter-group">
            <button className="pp-filter-btn active">{filtered.length} pendentes</button>
          </div>
        </div>
        <table>
          <thead>
            <tr>
              <th>ID</th><th>Tipo</th><th>Beneficiario</th><th>Valor</th><th>Pedido</th><th>Risco</th><th>Cadeia</th><th>Status</th><th>Acoes</th>
            </tr>
          </thead>
          <tbody>
            {filtered.map((p) => {
              const amountClass = p.amount > 5000 ? 'veryHigh' : p.amount > 1000 ? 'high' : '';
              return (
                <tr key={p.id}>
                  <td className="pp-mono">{p.id}</td>
                  <td>{p.type}</td>
                  <td>{p.beneficiary}</td>
                  <td className={`pp-amount ${amountClass}`}>R$ {p.amount.toLocaleString('pt-BR', { minimumFractionDigits: 2 })}</td>
                  <td className="pp-mono">{p.order}</td>
                  <td><RiskBadge score={p.risk} /></td>
                  <td><ChainDots chain={p.chain} /></td>
                  <td><StatusBadge status={p.status} /></td>
                  <td>
                    <div className="pp-actions">
                      <button className="pp-btn view" title="Verificar" onClick={() => setModalOpen(true)}>\u25CE</button>
                      <button className="pp-btn approve" title="Aprovar" onClick={() => handleApprove(p.id)}>\u2713</button>
                      <button className="pp-btn reject" title="Rejeitar" onClick={() => handleReject(p.id)}>\u2715</button>
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {modalOpen && <PaymentModal detail={MOCK_DETAIL} onClose={() => setModalOpen(false)} onApprove={() => handleApprove(MOCK_DETAIL.id)} onReject={() => handleReject(MOCK_DETAIL.id)} />}

      {toasts.length > 0 && (
        <div className="pp-toast-container">
          {toasts.map((t) => (
            <div key={t.id} className={`pp-toast ${t.type}`}>{t.type === 'success' ? '\u2713' : '\u2715'} {t.message}</div>
          ))}
        </div>
      )}
    </div>
  );
}
