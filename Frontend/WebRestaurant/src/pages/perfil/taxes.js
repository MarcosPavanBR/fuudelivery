import { FiSave, FiTruck } from "react-icons/fi";
import MenuLayout from "../../components/Menu";
import React, { useEffect, useState } from "react";
import { useAuth } from "../../context/AuthContext";
import deliveryModel from "../../services/delivery.model";
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

  const start = async () => {
    const resp = await deliveryModel.getDelivery(estId);
    setBody({
      establishmentId: estId,
      fixedTaxa: resp?.FixedTaxa ?? 0,
      perKm: resp?.PerKm ?? 0,
    });
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
              <FiTruck className="h-5 w-5" style={{ color: "#EA1D2C" }} />
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
      </div>
    </MenuLayout>
  );
}

export default Taxes;
