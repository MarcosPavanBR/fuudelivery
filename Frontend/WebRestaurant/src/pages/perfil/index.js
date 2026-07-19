import React, { useEffect, useState } from "react";
import MenuLayout from "../../components/Menu";
import { useAuth } from "../../context/AuthContext";
import api from "../../services/api";
import { FiLoader, FiSave, FiUser, FiMapPin, FiImage, FiPalette } from "react-icons/fi";
import { toast } from "react-toastify";
import Texts from "../../constants/Texts";
import restaurantModel from "../../services/restaurant.model";
import BusinessHoursEditor from "../../components/BusinessHoursEditor";

function Perfil() {
  const { getUser } = useAuth();
  const [establishment, setEstablishment] = useState({});
  const [user, setUser] = useState({});
  const [loading, setLoading] = useState(false);

  const handlerEstablishment = (target) => {
    setEstablishment({ ...establishment, [target.name]: target.value });
  };

  const init = async () => {
    setLoading(true);
    try {
      const { data } = await api.get("/establishments/" + getUser().id);
      setEstablishment(data);
      const usert = getUser();
      setUser({ name: usert.name, email: usert.email });
    } catch (e) {
      console.log(e);
    }
    setLoading(false);
  };

  async function submit(e) {
    e.preventDefault();
    setLoading(true);
    const resp = await restaurantModel.updateEstablishment(getUser().establishment_id, establishment);
    if (resp) toast.success(Texts.restaurant_update);
    else toast.error(Texts.restaurant_error);
    setLoading(false);
  }

  useEffect(() => { init(); }, []);

  return (
    <MenuLayout>
      {loading && (
        <div className="flex items-center justify-center h-32">
          <FiLoader className="animate-spin h-6 w-6" style={{ color: "#EA1D2C" }} />
        </div>
      )}

      <form className="space-y-6 animate-fade-in" onSubmit={submit}>
        {/* User Section */}
        <div className="bg-white rounded-2xl border border-gray-100 shadow-card p-6">
          <div className="flex items-center gap-3 mb-5">
            <div className="p-2.5 rounded-xl bg-red-50">
              <FiUser className="h-5 w-5" style={{ color: "#EA1D2C" }} />
            </div>
            <h3 className="text-lg font-bold text-gray-900">Usuário</h3>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Nome</label>
              <input disabled value={user.name} className="block w-full px-4 py-2.5 bg-gray-100 border border-gray-200 rounded-xl text-sm text-gray-500" />
            </div>
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">E-mail</label>
              <input disabled value={user.email} className="block w-full px-4 py-2.5 bg-gray-100 border border-gray-200 rounded-xl text-sm text-gray-500" />
            </div>
          </div>
        </div>

        {/* Establishment Section */}
        <div className="bg-white rounded-2xl border border-gray-100 shadow-card p-6">
          <div className="flex items-center gap-3 mb-5">
            <div className="p-2.5 rounded-xl bg-red-50">
              <FiPalette className="h-5 w-5" style={{ color: "#EA1D2C" }} />
            </div>
            <h3 className="text-lg font-bold text-gray-900">Estabelecimento</h3>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Nome</label>
              <input name="name" maxLength={80} required onChange={({ target }) => handlerEstablishment(target)} value={establishment.name}
                className="block w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" />
            </div>
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Descrição</label>
              <input name="description" maxLength={150} required onChange={({ target }) => handlerEstablishment(target)} value={establishment.description}
                className="block w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" />
            </div>
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Cor Primária</label>
              <input type="color" name="primary_color" required onChange={({ target }) => handlerEstablishment(target)} value={establishment.primary_color}
                className="w-full h-12 rounded-xl border border-gray-200 cursor-pointer" />
            </div>
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Cor Secundária</label>
              <input type="color" name="secondary_color" required onChange={({ target }) => handlerEstablishment(target)} value={establishment.secondary_color}
                className="w-full h-12 rounded-xl border border-gray-200 cursor-pointer" />
            </div>
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Dist. Máxima (km)</label>
              <input type="number" min={1} max={100} name="max_distance_delivery" required onChange={({ target }) => handlerEstablishment(target)} value={establishment.max_distance_delivery}
                className="block w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" />
            </div>
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Horário Funcionamento</label>
              <input name="horarioFuncionamento" maxLength={50} required onChange={({ target }) => handlerEstablishment(target)} value={establishment.horarioFuncionamento}
                className="block w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" />
            </div>
          </div>

          <div className="mt-4">
            <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">URL Logo</label>
            <input name="image" required onChange={({ target }) => handlerEstablishment(target)} value={establishment.image}
              className="block w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" placeholder="https://..." />
          </div>
        </div>

        {/* Address Section */}
        <div className="bg-white rounded-2xl border border-gray-100 shadow-card p-6">
          <div className="flex items-center gap-3 mb-5">
            <div className="p-2.5 rounded-xl bg-red-50">
              <FiMapPin className="h-5 w-5" style={{ color: "#EA1D2C" }} />
            </div>
            <h3 className="text-lg font-bold text-gray-900">Endereço</h3>
          </div>
          <div className="mb-4">
            <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Endereço Completo</label>
            <input name="location_string" maxLength={250} required onChange={({ target }) => handlerEstablishment(target)} value={establishment.location_string}
              className="block w-full px-4 py-2.5 bg-gray-50 border border-gray-200 rounded-xl text-sm focus:bg-white" />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Latitude</label>
              <input type="number" name="lat" required disabled onChange={({ target }) => handlerEstablishment(target)} value={establishment.lat}
                className="block w-full px-4 py-2.5 bg-gray-100 border border-gray-200 rounded-xl text-sm text-gray-500" />
            </div>
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">Longitude</label>
              <input type="number" name="long" required disabled onChange={({ target }) => handlerEstablishment(target)} value={establishment.long}
                className="block w-full px-4 py-2.5 bg-gray-100 border border-gray-200 rounded-xl text-sm text-gray-500" />
            </div>
          </div>
        </div>

        {/* Business Hours */}
        <BusinessHoursEditor establishmentId={getUser().establishment_id} />

        {/* Save Button */}
        <div className="flex justify-end">
          <button
            type="submit"
            disabled={loading}
            className="flex items-center gap-2 px-6 py-3 rounded-xl text-white font-semibold text-sm transition-all duration-200 hover:shadow-lg disabled:opacity-50"
            style={{ background: "linear-gradient(135deg, #EA1D2C, #C41420)" }}
          >
            <FiSave className="h-5 w-5" />
            Salvar Alterações
          </button>
        </div>
      </form>
    </MenuLayout>
  );
}

export default Perfil;
