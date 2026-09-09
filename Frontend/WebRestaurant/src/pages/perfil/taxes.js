import { FiSave, FiTruck, FiPercent } from "react-icons/fi";
import MenuLayout from "../../components/Menu";
import React, { useEffect, useState } from "react";
import { useAuth } from "../../context/AuthContext";
import deliveryModel from "../../services/delivery.model";
import zoneModel from "../../services/zone.model";
import { toast } from "react-toastify";
import Texts from "../../constants/Texts";

function Taxes() {
  const { getUser } = useAuth();
  const estId = getUser()?.establishment_id || getUser()?.establishment?.id || getUser()?.sub;

  const [body, setBody] = useState({
    establishmentId: estId,
    fixedTaxa: 0,
    perKm: 0,
  });
  const [zoneFee, setZoneFee] = useState(null);

  const start = async () => {
    const [resp, fee] = await Promise.all([
      deliveryModel.getDelivery(estId),
      zoneModel.getMyZoneFee(),
    ]);
    setBody({
      establishmentId: estId,
      fixedTaxa: resp?.FixedTaxa ?? 0,
      perKm: resp?.PerKm ?? 0,
    });
    setZoneFee(fee);
    // Erro real (403/500/network) não pode renderizar números como se
    // fossem a comissão real — avisar é dever de casa, não opcional.
    if (fee?.error) {
      toast.error("Não foi possível carregar a taxa da sua região. Tente recarregar a página.");
    }
  };

  useEffect(() => {
    start();
  }, []);

  const save = async (e) => {
    e.preventDefault();
    const resp = await deliveryModel.saveDelivery(body);
    if (resp) toast.success(Texts.delivery_update);
    else toast.error(Texts.delivery_error);
  };

  return (
    <MenuLayout>
      <div className="animate-fade-in">
        <div className="mb-6">
          <h3 className="text-lg font-bold text-gray-900">{Texts.delivery_conf}</h3>
          <p className="text-sm text-gray-500 mt-1">{Texts.taxes_desc}</p>
        </div>

        <div className="bg-white rounded-xl border border-gray-100 shadow-card p-6">
          <div className="flex items-center gap-2 mb-4">
            <div className="p-2 rounded-lg bg-red-50">
              <FiTruck className="h-5 w-5" style={{ color: "#DC2626" }} />
            </div>
            <h4 className="font-bold text-gray-900">Configurações de Entrega</h4>
          </div>

          <form onSubmit={save} className="space-y-4">
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div>                  <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">
                  Taxa de Serviço - R$ <span className="text-gray-400 normal-case">(Fixo)</span>
                </label>
                <input
                  type="number"
                  required
                  value={body.fixedTaxa}
                  onChange={({ target }) => setBody({ ...body, fixedTaxa: target.value })}
                  className="input"
                  placeholder="0.00"
                />
              </div>
              <div>                  <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">
                  Valor por Quilômetro - R$
                </label>
                <input
                  type="number"
                  required
                  value={body.perKm}
                  onChange={({ target }) => setBody({ ...body, perKm: target.value })}
                  className="input"
                  placeholder="0.00"
                />
              </div>
            </div>

            <div className="flex justify-end pt-4">
              <button
                type="submit"
                className="btn btn-primary"
              >
                <FiSave className="h-5 w-5" />
                Salvar
              </button>
            </div>
          </form>
        </div>

        {zoneFee && (
          <div className="bg-white rounded-xl border border-gray-100 shadow-card p-6 mt-6">
            <div className="flex items-center gap-2 mb-4">
              <div className="p-2 rounded-lg bg-red-50">
                <FiPercent className="h-5 w-5" style={{ color: "#DC2626" }} />
              </div>
              <div>
                <h4 className="font-bold text-gray-900">{Texts.comissao_plataforma}</h4>
                <p className="text-xs text-gray-500 mt-0.5">{Texts.comissao_desc}</p>
              </div>
            </div>

            {zoneFee.error ? (
              <div className="flex items-center gap-2 p-3 rounded-lg bg-red-50">
                <FiPercent className="h-5 w-5 shrink-0" style={{ color: "#DC2626" }} />
                <p className="text-sm text-red-700">
                  Não foi possível carregar a taxa da sua região agora. Os valores abaixo não estão disponíveis.
                </p>
              </div>
            ) : (
              <>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 mb-4">
              <div>
                <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">
                  {Texts.taxa_atual_plataforma}
                </label>
                <p className="text-2xl font-bold text-gray-900">{zoneFee.current_platform_pct}%</p>
              </div>
              <div>
                <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">
                  {Texts.sua_parte_pedido}
                </label>
                <p className="text-2xl font-bold text-gray-900">{zoneFee.current_establishment_pct}%</p>
              </div>
            </div>

            {!zoneFee.has_zone && (
              <p className="text-sm text-gray-500">{Texts.comissao_sem_zona}</p>
            )}

            {zoneFee.has_zone && (
              <>
                <p className="text-sm text-gray-500 mb-2">
                  Região: {zoneFee.zone_name} — {zoneFee.city}
                </p>
                {zoneFee.at_target ? (
                  <p className="text-sm text-gray-500">{Texts.comissao_no_target}</p>
                ) : (
                  <div className="text-sm text-gray-500 space-y-1">
                    <p>
                      {Texts.comissao_meta_final}: {zoneFee.target_platform_pct}% para a plataforma,{" "}
                      {zoneFee.target_establishment_pct}% para você.
                    </p>
                    <p>
                      Como a taxa evolui: a cada {zoneFee.step_months} meses, se a região mantiver pelo
                      menos {zoneFee.min_monthly_orders} pedidos por mês, a taxa da plataforma sobe{" "}
                      {zoneFee.step_platform_pct} ponto(s) percentual(is), até chegar à meta.
                    </p>
                    {zoneFee.last_adjusted_at && (
                      <p>
                        {Texts.ultimo_ajuste}: {new Date(zoneFee.last_adjusted_at).toLocaleDateString("pt-BR")}
                      </p>
                    )}
                  </div>
                )}
              </>
            )}
              </>
            )}
          </div>
        )}
      </div>
    </MenuLayout>
  );
}

export default Taxes;
