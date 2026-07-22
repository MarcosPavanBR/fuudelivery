import React from 'react';
import '../../styles/payments.css';

const WORKFLOW = [
  { icon: '\uD83D\uDCE6', label: '1. Pedido Criado', description: 'Cliente faz pedido\nGateway processa pagamento', color: 'blue' },
  { icon: '\u2713', label: '2. Pedido Confirmado', description: 'Restaurante aceita\nEntregador designado', color: 'green' },
  { icon: '\uD83D\uDEFC', label: '3. Entrega Realizada', description: 'Confirmacao de entrega\nGPS + foto comprovante', color: 'green' },
  { icon: '\u23F3', label: '4. Periodo de Espera', description: '48h anti-fraude\nJanela de reclamacao', color: 'purple' },
  { icon: '\uD83D\uDD0D', label: '5. Verificacao', description: 'Score de risco\nValida pedido + reclamacoes', color: 'yellow' },
  { icon: '\uD83D\uDCB0', label: '6a. Auto-Aprovacao', description: 'Score baixo < 20\nSem reclamacoes\nValor < R$ 1.000', color: 'green' },
  { icon: '\u26A0\uFE0F', label: '6b. Analise Manual', description: 'Alto risco / Alto valor\nCompliance verifica', color: 'red' },
  { icon: '\uD83D\uDCB3', label: '7. Gateway Executa', description: 'PIX ou transferencia\nComprovante gerado', color: 'green' },
];

const ORDER_CHECKS = [
  { label: 'Pedido existe e esta confirmado', status: 'pass', result: 'OK' },
  { label: 'Entrega registrada (GPS + timestamp)', status: 'pass', result: 'OK' },
  { label: 'Pagamento capturado no gateway', status: 'pass', result: 'OK' },
  { label: 'Sem reclamacao ativa do cliente', status: 'warn', result: 'Parcial' },
  { label: 'Sem estorno registrado', status: 'pass', result: 'OK' },
  { label: 'Janela anti-fraude expirada (48h)', status: 'pass', result: 'OK' },
];

const FRAUD_CHECKS = [
  { label: 'Sem multiplos pedidos do mesmo dispositivo', status: 'pass', result: 'OK' },
  { label: 'Score de confiabilidade do parceiro', status: 'fail', result: 'Abaixo' },
  { label: 'KYC do beneficiario verificado', status: 'pass', result: 'OK' },
  { label: 'Historico de chargebacks', status: 'warn', result: '1 anterior' },
  { label: 'Valor dentro da media do parceiro', status: 'pass', result: 'OK' },
  { label: 'IP / Geolocalizacao consistente', status: 'pass', result: 'OK' },
];

const resultColors = { pass: 'var(--pp-accent)', fail: 'var(--pp-danger)', warn: 'var(--pp-warning)' };

function VerifyCard({ title, titleColor, checks }) {
  return (
    <div className="pp-verify-card">
      <h3><span style={{ color: titleColor }}>{'\u25CF'}</span>{title}</h3>
      {checks.map((c, i) => (
        <div key={i} className="pp-verify-item">
          <div className="pp-verify-left">
            <div className={`pp-verify-icon ${c.status}`}>{c.status === 'pass' ? '\u2713' : c.status === 'fail' ? '\u2715' : '~'}</div>
            <span className="pp-verify-label">{c.label}</span>
          </div>
          <span className="pp-verify-value" style={{ color: resultColors[c.status] }}>{c.result}</span>
        </div>
      ))}
    </div>
  );
}

export default function PaymentWorkflow() {
  return (
    <div className="pp-panel">
      <div className="pp-section-header">
        <div className="pp-section-title">
          <div className="dot" style={{ background: 'var(--pp-info)' }} />
          Fluxo de Aprovacao de Pagamentos
        </div>
      </div>

      <div className="pp-workflow">
        <div className="pp-workflow-title">
          <svg width="16" height="16" fill="none" stroke="var(--pp-accent)" strokeWidth="2" viewBox="0 0 24 24">
            <path d="M22 12h-4l-3 9L9 3l-3 9H2" />
          </svg>
          Pipeline de Pagamento \u2014 Do Pedido ao Saque
        </div>
        <div className="pp-workflow-steps">
          {WORKFLOW.map((step, i) => (
            <div key={i} className="pp-wf-step">
              <div className={`pp-wf-node ${step.color}`}>{step.icon}</div>
              <div className="pp-wf-label">{step.label}</div>
              <div className="pp-wf-sub">{step.description}</div>
              {i < WORKFLOW.length - 1 && <div className="pp-wf-connector" />}
            </div>
          ))}
        </div>
      </div>

      <div className="pp-verify-grid">
        <VerifyCard title="Verificacoes do Pedido (Pre-Aprovacao)" titleColor="var(--pp-info)" checks={ORDER_CHECKS} />
        <VerifyCard title="Verificacoes Anti-Fraude" titleColor="var(--pp-warning)" checks={FRAUD_CHECKS} />
      </div>
    </div>
  );
}
