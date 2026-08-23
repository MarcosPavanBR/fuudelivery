import React, { useEffect, useState, useRef } from "react";
import MenuLayout from "../../components/Menu";
import { useAuth } from "../../context/AuthContext";
import api from "../../services/api";
import {
  FiLoader,
  FiSave,
  FiUser,
  FiMapPin,
  FiGrid,
  FiLock,
  FiCamera,
} from "react-icons/fi";
import { toast } from "react-toastify";
import Texts from "../../constants/Texts";
import restaurantModel from "../../services/restaurant.model";
import BusinessHoursEditor from "../../components/BusinessHoursEditor";

const inputClass = "input";
const RequiredMark = () => <span className="text-red-500">*</span>;

function Perfil() {
  const { getUser } = useAuth();
  const [establishment, setEstablishment] = useState({});
  const [user, setUser] = useState({});
  const [loading, setLoading] = useState(false);
  const [avatar, setAvatar] = useState(
    () => localStorage.getItem("fuu_restaurant_avatar") || getUser()?.avatar_url || ""
  );
  const fileInputRef = useRef(null);

  const handlerEstablishment = (target) => {
    setEstablishment({ ...establishment, [target.name]: target.value });
  };

  const init = async () => {
    setLoading(true);
    try {
      const userData = getUser();
      if (!userData) { setLoading(false); return; }
      const estId = userData.establishment_id || userData.establishment?.id || userData.sub;
      if (!estId) { setLoading(false); return; }
      const { data } = await api.get("/establishments/" + estId);
      setEstablishment(data);
      setUser({ name: userData.name || "", email: userData.email || "" });

      // Avatar persistido no backend (fonte da verdade), com cache local.
      if (userData.id) {
        try {
          const { data: me } = await api.get(`/users/${userData.id}`);
          if (me?.avatar_url) {
            setAvatar(me.avatar_url);
            localStorage.setItem("fuu_restaurant_avatar", me.avatar_url);
          }
        } catch (e) {
          console.error(e);
        }
      }
    } catch (e) {
      console.error(e);
    }
    setLoading(false);
  };

  const handleAvatarUpload = async (e) => {
    const file = e.target.files?.[0];
    if (!file) return;
    if (!file.type.startsWith("image/")) {
      toast.error("Selecione um arquivo de imagem");
      return;
    }
    if (file.size > 2 * 1024 * 1024) {
      toast.error("Imagem muito grande (máx. 2MB)");
      return;
    }
    try {
      const fd = new FormData();
      fd.append("file", file);
      const { data } = await api.post("/upload/avatars", fd, {
        headers: { "Content-Type": "multipart/form-data" },
      });
      if (!data?.url) throw new Error("Upload sem URL");
      await api.put(`/users/${getUser()?.id}`, { avatar_url: data.url });
      setAvatar(data.url);
      localStorage.setItem("fuu_restaurant_avatar", data.url);
      toast.success("Foto atualizada!");
    } catch (err) {
      toast.error(err?.response?.data?.error || "Erro ao enviar foto");
    }
  };

  async function submit(e) {
    e.preventDefault();
    setLoading(true);
    const estId = getUser()?.establishment_id || getUser()?.establishment?.id || getUser()?.sub;
    const resp = await restaurantModel.updateEstablishment(estId, establishment);
    if (resp) toast.success(Texts.restaurant_update);
    else toast.error(Texts.restaurant_error);
    setLoading(false);
  }

  useEffect(() => { init(); }, []);

  return (
    <MenuLayout>
      {loading && (
        <div className="flex items-center justify-center h-32">
          <FiLoader className="animate-spin h-6 w-6" style={{ color: "#DC2626" }} />
        </div>
      )}

      <form className="space-y-6 animate-fade-in" onSubmit={submit}>
        {/* User Section */}
        <div className="card p-6">
          <div className="flex items-center gap-2 mb-4">
            <div className="p-2 rounded-lg bg-red-50">
              <FiUser className="h-5 w-5" style={{ color: "#DC2626" }} />
            </div>
            <h3 className="text-lg font-bold text-gray-900 dark:text-white">Usuário</h3>
          </div>

          {/* Avatar / foto de perfil */}
          <div className="flex items-center gap-4 mb-4">
            <div className="relative flex-shrink-0">
              <div
                className="w-16 h-16 rounded-full flex items-center justify-center overflow-hidden border-4 border-gray-100 shadow-sm"
                style={{ background: "linear-gradient(135deg, #DC2626, #F59E0B)" }}
              >
                {avatar ? (
                  <img
                    src={avatar}
                    alt="Foto de perfil"
                    className="w-full h-full object-cover"
                  />
                ) : (
                  <span className="text-white font-bold text-2xl">
                    {user.name?.charAt(0) || "R"}
                  </span>
                )}
              </div>
              <button
                type="button"
                onClick={() => fileInputRef.current?.click()}
                className="absolute -bottom-1 -right-1 w-7 h-7 rounded-full bg-gray-900 text-white flex items-center justify-center hover:bg-gray-700 transition-colors shadow"
                title="Alterar foto"
              >
                <FiCamera className="h-3.5 w-3.5" />
              </button>
              <input
                ref={fileInputRef}
                type="file"
                accept="image/*"
                onChange={handleAvatarUpload}
                className="hidden"
              />
            </div>
            <div>
              <p className="font-semibold text-gray-900 dark:text-white">
                {user.name || "Seu nome"}
              </p>
              <p className="text-sm text-gray-500">Foto de perfil</p>
              <button
                type="button"
                onClick={() => fileInputRef.current?.click()}
                className="mt-1 text-sm font-medium text-gray-600 underline underline-offset-2 hover:text-gray-900 dark:hover:text-white"
              >
                Alterar foto
              </button>
            </div>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">Nome</label>
              <input disabled value={user.name} className={inputClass} />
            </div>
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">E-mail</label>
              <input disabled value={user.email} className={inputClass} />
            </div>
          </div>
          <div className="mt-4">
            <a
              href="/#/alterar-senha"
              className="btn btn-ghost"
            >
              <FiLock className="h-4 w-4" />
              Alterar Senha
            </a>
          </div>
        </div>

        {/* Establishment Section */}
        <div className="card p-6">
          <div className="flex items-center gap-2 mb-4">
            <div className="p-2 rounded-lg bg-red-50">
              <FiGrid className="h-5 w-5" style={{ color: "#DC2626" }} />
            </div>
            <h3 className="text-lg font-bold text-gray-900 dark:text-white">Estabelecimento</h3>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">
                Nome <RequiredMark />
              </label>
              <input name="name" maxLength={80} required onChange={({ target }) => handlerEstablishment(target)} value={establishment.name} className={inputClass} />
            </div>
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">
                Descrição <RequiredMark />
              </label>
              <input name="description" maxLength={150} required onChange={({ target }) => handlerEstablishment(target)} value={establishment.description} className={inputClass} />
            </div>
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">
                Cor Primária <RequiredMark />
              </label>
              <input type="color" name="primary_color" required onChange={({ target }) => handlerEstablishment(target)} value={establishment.primary_color}
                className="w-full h-12 rounded-lg border border-gray-200 cursor-pointer dark:border-gray-700" />
            </div>
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">
                Cor Secundária <RequiredMark />
              </label>
              <input type="color" name="secondary_color" required onChange={({ target }) => handlerEstablishment(target)} value={establishment.secondary_color}
                className="w-full h-12 rounded-lg border border-gray-200 cursor-pointer dark:border-gray-700" />
            </div>
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">
                Dist. Máxima (km) <RequiredMark />
              </label>
              <input type="number" min={1} max={100} name="max_distance_delivery" required onChange={({ target }) => handlerEstablishment(target)} value={establishment.max_distance_delivery} className={inputClass} />
            </div>
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">
                Horário Funcionamento <RequiredMark />
              </label>
              <input name="horarioFuncionamento" maxLength={50} required onChange={({ target }) => handlerEstablishment(target)} value={establishment.horarioFuncionamento} className={inputClass} />
            </div>
          </div>

          <div className="mt-4">
            <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">
              URL Logo <RequiredMark />
            </label>
            <input name="image" required onChange={({ target }) => handlerEstablishment(target)} value={establishment.image}
              className={inputClass} placeholder="https://..." />
          </div>
        </div>

        {/* Address Section */}
        <div className="card p-6">
          <div className="flex items-center gap-2 mb-4">
            <div className="p-2 rounded-lg bg-red-50">
              <FiMapPin className="h-5 w-5" style={{ color: "#DC2626" }} />
            </div>
            <h3 className="text-lg font-bold text-gray-900 dark:text-white">Endereço</h3>
          </div>
          <div className="mb-4">
            <label className="block text-xs font-semibold text-gray-500 uppercase mb-1.5">
              Endereço Completo <RequiredMark />
            </label>
            <input name="location_string" maxLength={250} required onChange={({ target }) => handlerEstablishment(target)} value={establishment.location_string} className={inputClass} />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">Latitude</label>
              <input type="number" name="lat" required disabled onChange={({ target }) => handlerEstablishment(target)} value={establishment.lat} className={inputClass} />
            </div>
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">Longitude</label>
              <input type="number" name="long" required disabled onChange={({ target }) => handlerEstablishment(target)} value={establishment.long} className={inputClass} />
            </div>
          </div>
        </div>

        {/* Business Hours */}
        <BusinessHoursEditor establishmentId={getUser()?.establishment_id || getUser()?.establishment?.id || getUser()?.sub} />

        {/* Save Button */}
        <div className="flex justify-end">
          <button
            type="submit"
            disabled={loading}
            className="btn btn-primary"
          >
            {loading ? (
              <FiLoader className="h-5 w-5 animate-spin" />
            ) : (
              <FiSave className="h-5 w-5" />
            )}
            Salvar Alterações
          </button>
        </div>
      </form>
    </MenuLayout>
  );
}

export default Perfil;
