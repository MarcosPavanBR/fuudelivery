/**
 * PaymentResponsibility.js
 * Cadeia de responsabilidade e matriz de atribuicao de culpa.
 * Define responsabilidades de cada ator (Restaurante, Entregador, Cliente)
 * e como problemas afetam o pagamento (reter, bloquear, liberar).
 */
import React from 'react';
import '../../styles/payments.css';

/** Cards de responsabilidade para cada ator da cadeia */
const RESP_CARDS = [
  {
    role: 'restaurant', icon: '\uD83C\uDF73', name: 'Restaurante', subtitle: 'Preparo & Qualidade',
    responsibilities: [
      'Confirmar recebimento do pedido corretamente',
      'Preparar itens conforme solicitado',
      'Verificar embalagem e lacracao',
      'Registrar foto do pedido pronto',
      'Responsavel por itens faltantes/errados',
      'Comprovar qualidade em caso de reclamacao',
    ],
    impactNote: 'Reclamacoes de qualidade/item errado podem reter o pagamento ate resolucao',
    impactColor: 'warning',
  },
  {
    role: 'driver', icon: '\uD83D\uDEFC', name: 'Entregador', subtitle: 'Transporte & Entrega',
    responsibilities: [
      'Retirar pedido dentro do prazo',
      'Manter temperatura e integridade',
      'Registrar GPS durante toda rota',
      'Foto de entrega no destino correto',
      'Confirmar entrega com codigo/foto',
      'Responsavel por extravio/dano em transito',
    ],
    impactNote: 'Comprovantes de entrega (GPS + foto) sao obrigatorios para liberar repasse',
    impactColor: 'info',
  },
  {
    role: 'customer', icon: '\uD83D\uDC64', name: 'Cliente', subtitle: 'Recebimento & Validacao',
    responsibilities: [
      'Fornecer endereco correto e acessivel',
      'Estar disponivel para receber',
      'Conferir pedido na retirada/entrega',
      'Registrar reclamacao em ate 48h',
      'Prover evidencias (fotos) da reclamacao',
      'Responsavel por endereco errado/indisponivel',
    ],
    impactNote: 'Reclamacoes sem evidencia ou apos 48h nao afetam pagamento ao parceiro',
    impactColor: 'purple',
  },
];

const LIABILITY = [
  { problem: 'Pedido nao entregue', investigation: 'GPS + Foto + Timestamp', responsible: 'Entregador', action: 'Bloquear repasse entregador', actionColor: 'danger', evidence: 'GPS, foto com timestamp, confirmacao cliente' },
  { problem: 'Item faltante ou errado', investigation: 'Foto embalagem + Pedido', responsible: 'Restaurante', action: 'Reter parcial restaurante', actionColor: 'warning', evidence: 'Foto do pedido, recibo, comparacao' },
  { problem: 'Produto em mau estado', investigation: 'Foto + Embalagem + Rota', responsible: 'Analisar', action: 'Investigar ambos', actionColor: 'warning', evidence: 'Foto do produto, condicao embalagem, tempo rota' },
  { problem: 'Atraso excessivo', investigation: 'Timeline + GPS', responsible: 'Verificar causa', action: 'Nao afeta pagamento', actionColor: 'accent', evidence: 'Logs de tempo, causa raiz' },
  { problem: 'Estorno por fraude cliente', investigation: 'Evidencias + Historico', responsible: 'Cliente', action: 'Liberar parceiro, bloquear cliente', actionColor: 'accent', evidence: 'Entrega confirmada, historico do cliente' },
  { problem: 'Endereco errado/incompleto', investigation: 'Dados pedido + GPS', responsible: 'Cliente', action: 'Liberar todos, custo cliente', actionColor: 'accent', evidence: 'Endereco informado vs GPS entrega' },
];

function StatusBadge({ label }) {
  return <span className="pp-badge compliance"><span className="pp-badge-dot" />{label}</span>;
}

function RespCard({ role, icon, name, subtitle, responsibilities, impactNote, impactColor }) {
  return (
    <div className={`pp-resp-card ${role}`}>
      <div className="pp-resp-header">
        <div className="pp-resp-avatar">{icon}</div>
        <div>
          <div className="pp-resp-name">{name}</div>
          <div className="pp-resp-role">{subtitle}</div>
        </div>
      </div>
      <ul className="pp-resp-items">
        {responsibilities.map((r, i) => <li key={i}>{r}</li>)}
      </ul>
      <div className={`pp-impact-note ${impactColor}`}>
        <strong>Impacto no pagamento:</strong> {impactNote}
      </div>
    </div>
  );
}

export default function PaymentResponsibility() {
  return (
    <div className="pp-panel">
      <div className="pp-section-header">
        <div className="pp-section-title">
          <div className="dot" style={{ background: 'var(--pp-purple)' }} />
          Cadeia de Responsabilidade
        </div>
      </div>

      <div className="pp-resp-grid">
        {RESP_CARDS.map((card) => (
          <RespCard key={card.role} {...card} />
        ))}
      </div>

      <div className="pp-section-header" style={{ marginTop: 8 }}>
        <div className="pp-section-title">
          <div className="dot" style={{ background: 'var(--pp-warning)' }} />
          Matriz de Atribuicao de Culpa
        </div>
      </div>

      <div className="pp-table-wrap">
        <table>
          <thead>
            <tr>
              <th>Tipo de Problema</th><th>Investigacao</th><th>Responsavel</th><th>Acao sobre Pagamento</th><th>Evidencia Necessaria</th>
            </tr>
          </thead>
          <tbody>
            {LIABILITY.map((row, i) => (
              <tr key={i}>
                <td>{row.problem}</td>
                <td><span className="pp-invest-tag">{row.investigation}</span></td>
                <td><StatusBadge label={row.responsible} /></td>
                <td><span className={`pp-liability-action ${row.actionColor}`}>{row.action}</span></td>
                <td className="pp-evidence">{row.evidence}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
