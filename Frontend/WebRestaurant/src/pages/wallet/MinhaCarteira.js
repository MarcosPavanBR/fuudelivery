/**
 * MinhaCarteira.js
 * Página simplificada de carteira para o restaurante.
 * Mostra apenas saldo e extrato do restaurante logado.
 * SEM funcionalidade de aprovação — somente visualização.
 */
import React, { useState, useEffect } from 'react';
import { useAuth } from '../../context/AuthContext';
import { toast } from 'react-toastify';

const API_URL = process.env.REACT_APP_API_URL || 'https://fuudelivery-api-8y6l.onrender.com';

/**
 * Busca dados da carteira do restaurante logado
 */
async function fetchWallet(token) {
  const res = await fetch(`${API_URL}/wallet`, {
    headers: { Authorization: `Bearer ${token}` }
  });
  if (!res.ok) throw new Error('Erro ao carregar carteira');
  return res.json();
}

/**
 * Busca transações da carteira do restaurante logado
 */
async function fetchTransactions(token) {
  const res = await fetch(`${API_URL}/wallet/transactions`, {
    headers: { Authorization: `Bearer ${token}` }
  });
  if (!res.ok) throw new Error('Erro ao carregar transações');
  return res.json();
}

/**
 * Solicita saque do saldo disponível
 */
async function requestWithdraw(token, amount) {
  const res = await fetch(`${API_URL}/wallet/withdraw`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
    body: JSON.stringify({ amount })
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || 'Erro ao solicitar saque');
  }
  return res.json();
}

/** Formata valor em Real */
function formatCurrency(value) {
  return `R$ ${(value || 0).toLocaleString('pt-BR', { minimumFractionDigits: 2 })}`;
}

/** Formata data para pt-BR */
function formatDate(dateStr) {
  if (!dateStr) return '-';
  return new Date(dateStr).toLocaleString('pt-BR');
}

/** Badge de status */
function StatusBadge({ status }) {
  const MAP = {
    pending: { label: 'Pendente', bg: '#FEF3C7', color: '#B45309' },
    approved: { label: 'Aprovado', bg: '#ECFDF5', color: '#047857' },
    processing: { label: 'Processando', bg: '#DBEAFE', color: '#1D4ED8' },
    completed: { label: 'Concluído', bg: '#ECFDF5', color: '#047857' },
    rejected: { label: 'Rejeitado', bg: '#FEE2E2', color: '#B91C1C' },
    blocked: { label: 'Bloqueado', bg: '#FEE2E2', color: '#B91C1C' },
  };
  const cfg = MAP[status] || { label: status, bg: '#F3F4F6', color: '#4B5563' };
  return (
    <span style={{ padding: '4px 10px', borderRadius: '12px', fontSize: '12px', fontWeight: 600, background: cfg.bg, color: cfg.color }}>
      {cfg.label}
    </span>
  );
}

/** Card de saldo */
function BalanceCard({ label, value, color, icon }) {
  return (
    <div style={{ background: '#fff', borderRadius: '12px', padding: '20px', border: '1px solid #E5E7EB', flex: 1, minWidth: '200px' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '8px' }}>
        <span style={{ fontSize: '20px' }}>{icon}</span>
        <span style={{ color: '#6B7280', fontSize: '14px', fontWeight: 500 }}>{label}</span>
      </div>
      <div style={{ fontSize: '28px', fontWeight: 700, color }}>{formatCurrency(value)}</div>
    </div>
  );
}

export default function MinhaCarteira() {
  const { user } = useAuth();
  const [wallet, setWallet] = useState(null);
  const [transactions, setTransactions] = useState([]);
  const [loading, setLoading] = useState(true);
  const [withdrawAmount, setWithdrawAmount] = useState('');
  const [withdrawing, setWithdrawing] = useState(false);

  useEffect(() => {
    loadData();
  }, [user]);

  async function loadData() {
    try {
      const token = localStorage.getItem('token');
      const [walletData, txData] = await Promise.all([
        fetchWallet(token),
        fetchTransactions(token)
      ]);
      setWallet(walletData);
      setTransactions(txData.transactions || txData || []);
    } catch (err) {
      console.error(err);
      toast.error('Erro ao carregar dados da carteira');
    }
    setLoading(false);
  }

  async function handleWithdraw() {
    const amount = parseFloat(withdrawAmount);
    if (!amount || amount <= 0) {
      toast.error('Informe um valor válido');
      return;
    }
    if (amount > (wallet?.available || 0)) {
      toast.error('Saldo insuficiente');
      return;
    }

    setWithdrawing(true);
    try {
      const token = localStorage.getItem('token');
      await requestWithdraw(token, amount);
      toast.success(`Saque de ${formatCurrency(amount)} solicitado com sucesso!`);
      setWithdrawAmount('');
      loadData();
    } catch (err) {
      toast.error(err.message);
    }
    setWithdrawing(false);
  }

  if (loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '300px', color: '#6B7280' }}>
        Carregando carteira...
      </div>
    );
  }

  return (
    <div style={{ padding: '24px', maxWidth: '1200px', margin: '0 auto' }}>
      {/* Header */}
      <div style={{ marginBottom: '24px' }}>
        <h1 style={{ fontSize: '24px', fontWeight: 700, color: '#111827', margin: 0 }}>Minha Carteira</h1>
        <p style={{ color: '#6B7280', marginTop: '4px' }}>Acompanhe seus saldos e extrato financeiro</p>
      </div>

      {/* Saldo Cards */}
      <div style={{ display: 'flex', gap: '16px', marginBottom: '32px', flexWrap: 'wrap' }}>
        <BalanceCard label="Saldo Disponível" value={wallet?.available || 0} color="#047857" icon="💰" />
        <BalanceCard label="Saldo Pendente" value={wallet?.pending || 0} color="#B45309" icon="⏳" />
        <BalanceCard label="Saldo Bloqueado" value={wallet?.blocked || 0} color="#B91C1C" icon="🔒" />
        <BalanceCard label="Total Recebido" value={wallet?.totalReceived || 0} color="#1D4ED8" icon="📊" />
      </div>

      {/* Solicitar Saque */}
      <div style={{ background: '#F9FAFB', borderRadius: '12px', padding: '24px', marginBottom: '32px', border: '1px solid #E5E7EB' }}>
        <h3 style={{ fontSize: '16px', fontWeight: 600, color: '#111827', marginBottom: '12px' }}>Solicitar Saque</h3>
        <div style={{ display: 'flex', gap: '12px', alignItems: 'flex-end' }}>
          <div style={{ flex: 1, maxWidth: '300px' }}>
            <label style={{ display: 'block', fontSize: '14px', color: '#6B7280', marginBottom: '4px' }}>Valor (R$)</label>
            <input
              type="number"
              step="0.01"
              min="0"
              placeholder="0,00"
              value={withdrawAmount}
              onChange={(e) => setWithdrawAmount(e.target.value)}
              style={{ width: '100%', padding: '10px 12px', border: '1px solid #D1D5DB', borderRadius: '8px', fontSize: '14px' }}
            />
          </div>
          <button
            onClick={handleWithdraw}
            disabled={withdrawing || !withdrawAmount}
            style={{
              padding: '10px 20px', background: '#EA1D2C', color: '#fff', border: 'none', borderRadius: '8px',
              fontSize: '14px', fontWeight: 600, cursor: 'pointer', opacity: withdrawing || !withdrawAmount ? 0.5 : 1
            }}
          >
            {withdrawing ? 'Processando...' : 'Solicitar Saque'}
          </button>
        </div>
        <p style={{ fontSize: '12px', color: '#9CA3AF', marginTop: '8px' }}>
          Disponível para saque: {formatCurrency(wallet?.available || 0)}
        </p>
      </div>

      {/* Extrato */}
      <div style={{ background: '#fff', borderRadius: '12px', border: '1px solid #E5E7EB', overflow: 'hidden' }}>
        <div style={{ padding: '16px 20px', borderBottom: '1px solid #E5E7EB' }}>
          <h3 style={{ fontSize: '16px', fontWeight: 600, color: '#111827', margin: 0 }}>Extrato</h3>
        </div>
        <div style={{ overflowX: 'auto' }}>
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr style={{ background: '#F9FAFB' }}>
                <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '12px', fontWeight: 600, color: '#6B7280', textTransform: 'uppercase' }}>Data</th>
                <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '12px', fontWeight: 600, color: '#6B7280', textTransform: 'uppercase' }}>Descrição</th>
                <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '12px', fontWeight: 600, color: '#6B7280', textTransform: 'uppercase' }}>Tipo</th>
                <th style={{ padding: '12px 16px', textAlign: 'left', fontSize: '12px', fontWeight: 600, color: '#6B7280', textTransform: 'uppercase' }}>Status</th>
                <th style={{ padding: '12px 16px', textAlign: 'right', fontSize: '12px', fontWeight: 600, color: '#6B7280', textTransform: 'uppercase' }}>Valor</th>
              </tr>
            </thead>
            <tbody>
              {transactions.length === 0 ? (
                <tr>
                  <td colSpan={5} style={{ padding: '40px', textAlign: 'center', color: '#9CA3AF' }}>
                    Nenhuma transação encontrada
                  </td>
                </tr>
              ) : (
                transactions.map((tx, i) => (
                  <tr key={tx.id || i} style={{ borderTop: '1px solid #F3F4F6' }}>
                    <td style={{ padding: '12px 16px', fontSize: '14px', color: '#374151' }}>{formatDate(tx.createdAt)}</td>
                    <td style={{ padding: '12px 16px', fontSize: '14px', color: '#111827' }}>{tx.description || tx.type || '-'}</td>
                    <td style={{ padding: '12px 16px', fontSize: '14px', color: '#6B7280' }}>
                      {tx.type === 'credit' ? 'Crédito' : tx.type === 'debit' ? 'Débito' : tx.type || '-'}
                    </td>
                    <td style={{ padding: '12px 16px' }}><StatusBadge status={tx.status} /></td>
                    <td style={{
                      padding: '12px 16px', textAlign: 'right', fontWeight: 600, fontSize: '14px',
                      color: tx.type === 'credit' ? '#047857' : '#B91C1C'
                    }}>
                      {tx.type === 'credit' ? '+' : '-'} {formatCurrency(tx.amount)}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
