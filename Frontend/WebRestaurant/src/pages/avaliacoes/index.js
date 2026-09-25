import React, { useCallback, useEffect, useState } from "react";
import { toast } from "react-toastify";
import { FiLoader, FiMessageSquare, FiStar } from "react-icons/fi";
import MenuLayout from "../../components/Menu";
import api from "../../services/api";
import { useAuth } from "../../context/AuthContext";
import { establishmentIdOf } from "../../helpers/session";

const Stars = ({ n }) => (
  <span className="text-amber-500" aria-label={`${n} de 5`}>
    {"★".repeat(n)}
    <span className="text-gray-300">{"★".repeat(5 - n)}</span>
  </span>
);

// Avaliações dos clientes: nota média, distribuição e resposta pública.
function Avaliacoes() {
  const { getUser } = useAuth();
  const estId = establishmentIdOf(getUser());
  const [rating, setRating] = useState(null);
  const [reviews, setReviews] = useState([]);
  const [loading, setLoading] = useState(true);
  const [drafts, setDrafts] = useState({});
  const [saving, setSaving] = useState(null);

  const load = useCallback(async () => {
    if (!estId) return;
    try {
      const [r, list] = await Promise.all([
        api.get(`/reviews/rating/${estId}`),
        api.get(`/reviews/establishment/${estId}`, { params: { limit: 100 } }),
      ]);
      setRating(r.data);
      setReviews(list.data?.reviews || []);
    } catch (e) {
      toast.error("Não foi possível carregar as avaliações.");
    }
    setLoading(false);
  }, [estId]);

  useEffect(() => {
    load();
  }, [load]);

  const respond = async (id) => {
    const text = (drafts[id] || "").trim();
    if (!text) return;
    setSaving(id);
    try {
      await api.put(`/reviews/respond/${id}`, { response_text: text });
      toast.success("Resposta publicada.");
      setDrafts((d) => ({ ...d, [id]: "" }));
      await load();
    } catch (e) {
      toast.error(e?.response?.data?.error || "Não foi possível responder.");
    }
    setSaving(null);
  };

  if (loading) {
    return (
      <MenuLayout>
        <div className="flex items-center justify-center h-32">
          <FiLoader className="animate-spin h-6 w-6" style={{ color: "#DC2626" }} />
        </div>
      </MenuLayout>
    );
  }

  const total = rating?.total_reviews || 0;
  return (
    <MenuLayout>
      <div className="space-y-6 animate-fade-in">
        <div className="card p-6">
          <div className="flex items-center gap-2 mb-4">
            <div className="p-2 rounded-lg bg-red-50">
              <FiStar className="h-5 w-5" style={{ color: "#DC2626" }} />
            </div>
            <h3 className="text-lg font-bold text-gray-900 dark:text-white">Avaliações</h3>
          </div>
          {total === 0 ? (
            <p className="text-sm text-gray-500">
              Nenhuma avaliação ainda. O cliente avalia pelo app depois que o pedido é entregue.
            </p>
          ) : (
            <div className="flex flex-wrap items-center gap-8">
              <div>
                <p className="text-4xl font-bold text-gray-900 dark:text-white">
                  {Number(rating.average_rating).toFixed(1).replace(".", ",")}
                </p>
                <p className="text-sm text-gray-500">{total} {total > 1 ? "avaliações" : "avaliação"}</p>
              </div>
              <div className="flex-1 min-w-[200px] space-y-1">
                {[5, 4, 3, 2, 1].map((n) => {
                  const c = rating.rating_counts?.[n] || 0;
                  return (
                    <div key={n} className="flex items-center gap-2 text-xs text-gray-600">
                      <span className="w-6">{n}★</span>
                      <div className="h-2 flex-1 rounded bg-gray-100">
                        <div className="h-2 rounded bg-amber-400" style={{ width: `${(c / total) * 100}%` }} />
                      </div>
                      <span className="w-6 text-right">{c}</span>
                    </div>
                  );
                })}
              </div>
            </div>
          )}
        </div>

        {reviews.map((r) => (
          <div key={r.id} className="card p-5">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="flex items-center gap-2">
                <Stars n={r.rating} />
                <span className="text-sm font-medium text-gray-900 dark:text-white">{r.user_name || "Cliente"}</span>
              </div>
              <span className="text-xs text-gray-500">{new Date(r.created_at).toLocaleDateString("pt-BR")}</span>
            </div>
            {r.comment && <p className="mt-2 text-sm text-gray-700 dark:text-gray-300 whitespace-pre-line">{r.comment}</p>}
            {r.response_text ? (
              <div className="mt-3 rounded-lg bg-gray-50 p-3 dark:bg-gray-800">
                <p className="text-xs font-semibold text-gray-500">Sua resposta</p>
                <p className="text-sm text-gray-700 dark:text-gray-300 whitespace-pre-line">{r.response_text}</p>
              </div>
            ) : (
              <div className="mt-3 flex flex-col sm:flex-row gap-2">
                <input
                  className="input flex-1"
                  maxLength={500}
                  placeholder="Responder publicamente (o cliente e outros clientes veem)"
                  value={drafts[r.id] || ""}
                  onChange={(e) => setDrafts((d) => ({ ...d, [r.id]: e.target.value }))}
                />
                <button
                  type="button"
                  className="btn btn-primary"
                  disabled={saving === r.id || !(drafts[r.id] || "").trim()}
                  onClick={() => respond(r.id)}
                >
                  <FiMessageSquare className="h-4 w-4" /> Responder
                </button>
              </div>
            )}
          </div>
        ))}
      </div>
    </MenuLayout>
  );
}

export default Avaliacoes;
