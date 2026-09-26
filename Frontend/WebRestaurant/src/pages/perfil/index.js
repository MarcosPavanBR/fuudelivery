import React, { useEffect, useState, useRef } from "react";
import { Link } from "react-router-dom";
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
  FiImage,
  FiCrosshair,
} from "react-icons/fi";
import { toast } from "react-toastify";
import Texts from "../../constants/Texts";
import restaurantModel from "../../services/restaurant.model";
import { uploadImage } from "../../helpers/imageUpload";
import BusinessHoursEditor from "../../components/BusinessHoursEditor";
import {
  defaultHours,
  establishmentPayload,
  geocodeAddress,
  hoursPayload,
  mergeHours,
  needsGeocode,
  validateHours,
} from "./storeSettings";
import ConectarMercadoPago from "../../components/ConectarMercadoPago";
import { establishmentIdOf } from "../../helpers/session";

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
  const logoInputRef = useRef(null);
  const [hours, setHours] = useState(defaultHours);
  const [savedAddress, setSavedAddress] = useState("");
  const [locating, setLocating] = useState(false);
  const [uploadingLogo, setUploadingLogo] = useState(false);
  const [uploadingAvatar, setUploadingAvatar] = useState(false);
  const sessionUser = getUser();
  const estId = establishmentIdOf(sessionUser);

  const handlerEstablishment = (target) => {
    setEstablishment({ ...establishment, [target.name]: target.value });
  };

  const init = async () => {
    setLoading(true);
    try {
      const userData = getUser();
      if (!userData) { setLoading(false); return; }
      if (!estId) { setLoading(false); return; }
      const [{ data }, hoursResp] = await Promise.all([
        api.get("/establishments/" + estId),
        api.get(`/establishments/${estId}/hours`).catch(() => ({ data: [] })),
      ]);
      setEstablishment(data);
      setSavedAddress(data?.location_string || "");
      setHours(mergeHours(hoursResp.data));
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

  // Avatar e logo: reduzidos no navegador (helpers/imageUpload.js) e gravados
  // NA HORA. Antes a logo esperava o "Salvar alterações" — que o navegador
  // bloqueava em silêncio com campo obrigatório vazio, e a logo se perdia.
  const handleAvatarUpload = async (e) => {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    setUploadingAvatar(true);
    try {
      const url = await uploadImage("avatars", file, 512);
      await api.put(`/users/${sessionUser?.id}`, { avatar_url: url });
      setAvatar(url);
      localStorage.setItem("fuu_restaurant_avatar", url);
      toast.success("Foto atualizada!");
    } catch (err) {
      toast.error(err.message || "Erro ao enviar foto");
    }
    setUploadingAvatar(false);
  };

  const handleLogoUpload = async (e) => {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    setUploadingLogo(true);
    try {
      const url = await uploadImage(`restaurants/${estId}`, file, 512);
      const next = { ...establishment, image: url };
      const ok = await restaurantModel.updateEstablishment(estId, establishmentPayload(next, hours));
      if (!ok) throw new Error("A logo foi enviada, mas não foi possível salvar na loja.");
      setEstablishment((prev) => ({ ...prev, image: url }));
      toast.success("Logo atualizada!");
    } catch (err) {
      toast.error(err.message || "Erro ao enviar a logo");
    }
    setUploadingLogo(false);
  };

  const locate = async (est = establishment) => {
    setLocating(true);
    const coords = await geocodeAddress(est.location_string);
    setLocating(false);
    if (coords) setEstablishment((prev) => ({ ...prev, ...coords }));
    return coords;
  };

  async function submit(e) {
    e.preventDefault();
    const hoursError = validateHours(hours);
    if (hoursError) {
      toast.error(hoursError);
      return;
    }
    setLoading(true);

    let est = establishment;
    if (needsGeocode(savedAddress, est)) {
      const coords = await locate(est);
      if (coords) {
        est = { ...est, ...coords };
      } else {
        toast.warn("Não encontramos o endereço no mapa. Confira rua, número e cidade — a distância de entrega usa essa localização.");
      }
    }

    const [okEst, okHours] = await Promise.all([
      restaurantModel.updateEstablishment(estId, establishmentPayload(est, hours)),
      api.post("/establishments/hours/bulk", hoursPayload(hours, estId)).then(() => true, () => false),
    ]);
    if (okEst) setSavedAddress(est.location_string || "");
    if (okEst && okHours) toast.success(Texts.restaurant_update);
    else if (!okEst) toast.error(Texts.restaurant_error);
    else toast.error("Dados salvos, mas houve erro ao salvar os horários.");
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
                {uploadingAvatar ? <FiLoader className="h-3.5 w-3.5 animate-spin" /> : <FiCamera className="h-3.5 w-3.5" />}
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
              <input disabled value={user.name ?? ""} className={inputClass} />
            </div>
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">E-mail</label>
              <input disabled value={user.email ?? ""} className={inputClass} />
            </div>
          </div>
          <div className="mt-4">
            {/* BrowserRouter: href="/#/..." navegava para a raiz — usar Link. */}
            <Link to="/alterar-senha" className="btn btn-ghost">
              <FiLock className="h-4 w-4" />
              Alterar Senha
            </Link>
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
              <input name="name" maxLength={80} required onChange={({ target }) => handlerEstablishment(target)} value={establishment.name ?? ""} className={inputClass} />
            </div>
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">
                Descrição
              </label>
              <input name="description" maxLength={150} placeholder="Ex.: Pizzas artesanais no forno a lenha" onChange={({ target }) => handlerEstablishment(target)} value={establishment.description ?? ""} className={inputClass} />
            </div>
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">
                Cor Primária <RequiredMark />
              </label>
              <input type="color" name="primary_color" required onChange={({ target }) => handlerEstablishment(target)} value={establishment.primary_color || "#DC2626"}
                className="w-full h-12 rounded-lg border border-gray-200 cursor-pointer dark:border-gray-700" />
            </div>
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">
                Cor Secundária <RequiredMark />
              </label>
              <input type="color" name="secondary_color" required onChange={({ target }) => handlerEstablishment(target)} value={establishment.secondary_color || "#F59E0B"}
                className="w-full h-12 rounded-lg border border-gray-200 cursor-pointer dark:border-gray-700" />
            </div>
            <div>
              <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">
                Dist. Máxima (km) <RequiredMark />
              </label>
              <input type="number" min={1} max={100} name="max_distance_delivery" required onChange={({ target }) => handlerEstablishment(target)} value={establishment.max_distance_delivery ?? ""} className={inputClass} />
            </div>
          </div>

          <div className="mt-4">
            <label className="block text-xs font-semibold text-gray-500 uppercase mb-2">Logo</label>
            <div className="flex items-center gap-4">
              <div className="w-20 h-20 rounded-xl border border-gray-200 bg-gray-50 flex items-center justify-center overflow-hidden dark:border-gray-700 dark:bg-gray-800">
                {establishment.image ? (
                  <img src={establishment.image} alt="Logo da loja" className="w-full h-full object-cover" />
                ) : (
                  <FiImage className="h-7 w-7 text-gray-300" />
                )}
              </div>
              <div>
                <button type="button" className="btn btn-ghost" disabled={uploadingLogo} onClick={() => logoInputRef.current?.click()}>
                  {uploadingLogo ? <FiLoader className="h-4 w-4 animate-spin" /> : <FiCamera className="h-4 w-4" />}
                  {establishment.image ? "Trocar logo" : "Enviar logo"}
                </button>
                <p className="mt-1 text-xs text-gray-500">JPG, PNG ou WEBP. Foto do celular pode: reduzimos automaticamente. Aparece para o cliente no app.</p>
              </div>
              <input ref={logoInputRef} type="file" accept="image/*" onChange={handleLogoUpload} className="hidden" />
            </div>
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
            <input name="location_string" maxLength={250} required onChange={({ target }) => handlerEstablishment(target)} value={establishment.location_string ?? ""} className={inputClass} />
          </div>
          <div className="flex flex-wrap items-center gap-3">
            <button type="button" className="btn btn-ghost" disabled={locating || !establishment.location_string}
              onClick={async () => {
                const c = await locate();
                if (c) toast.success("Localização encontrada. Salve para aplicar.");
                else toast.warn("Não encontramos o endereço no mapa. Confira rua, número e cidade.");
              }}>
              {locating ? <FiLoader className="h-4 w-4 animate-spin" /> : <FiCrosshair className="h-4 w-4" />}
              Localizar no mapa
            </button>
            {Number(establishment.lat) && Number(establishment.long) ? (
              <a
                className="text-sm text-gray-600 underline underline-offset-2"
                href={`https://www.openstreetmap.org/?mlat=${establishment.lat}&mlon=${establishment.long}#map=17/${establishment.lat}/${establishment.long}`}
                target="_blank"
                rel="noreferrer"
              >
                Ver no mapa ({Number(establishment.lat).toFixed(5)}, {Number(establishment.long).toFixed(5)})
              </a>
            ) : (
              <span className="text-sm text-amber-700">Loja ainda sem localização — o cálculo de distância não funciona até localizar.</span>
            )}
          </div>
          <p className="mt-2 text-xs text-gray-500">A localização é buscada pelo endereço ao salvar, quando ele muda.</p>
        </div>

        {/* Conta de recebimento (split na origem) */}
        <ConectarMercadoPago />

        {/* Business Hours */}
        <BusinessHoursEditor hours={hours} onChange={setHours} />

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
            Salvar alterações
          </button>
        </div>
      </form>
    </MenuLayout>
  );
}

export default Perfil;
