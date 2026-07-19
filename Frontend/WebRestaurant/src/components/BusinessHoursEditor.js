import React, { useState, useEffect } from "react";
import api from "../services/api";
import { FiClock, FiSave, FiLoader } from "react-icons/fi";

const DAYS = ["Domingo", "Segunda", "Terça", "Quarta", "Quinta", "Sexta", "Sábado"];

const BusinessHoursEditor = ({ establishmentId }) => {
  const [hours, setHours] = useState(
    DAYS.map((_, i) => ({
      day_of_week: i,
      is_open: i !== 0,
      open_time: "08:00",
      close_time: "22:00",
      break_start_time: "",
      break_end_time: "",
    }))
  );
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    loadHours();
  }, []);

  const loadHours = async () => {
    try {
      const { data } = await api.get(`/establishments/${establishmentId}/hours`);
      if (data.length > 0) {
        const merged = hours.map((h) => {
          const existing = data.find((d) => d.day_of_week === h.day_of_week);
          return existing || h;
        });
        setHours(merged);
      }
    } catch (e) {
      console.error(e);
    }
  };

  const updateDay = (index, field, value) => {
    const updated = [...hours];
    updated[index] = { ...updated[index], [field]: value };
    setHours(updated);
  };

  const saveHours = async () => {
    setSaving(true);
    try {
      await api.post(
        "/establishments/hours/bulk",
        hours.map((h) => ({ ...h, establishment_id: establishmentId }))
      );
      alert("Horários salvos!");
    } catch (e) {
      alert("Erro ao salvar");
    }
    setSaving(false);
  };

  return (
    <div className="bg-white rounded-2xl border border-gray-100 shadow-card p-6">
      <div className="flex items-center gap-3 mb-6">
        <div className="p-2.5 rounded-xl bg-red-50">
          <FiClock className="h-5 w-5" style={{ color: "#EA1D2C" }} />
        </div>
        <h3 className="text-lg font-bold text-gray-900">Horário de Funcionamento</h3>
      </div>

      <div className="space-y-3">
        {hours.map((day, i) => (
          <div
            key={i}
            className="flex items-center gap-4 p-4 rounded-xl bg-gray-50 hover:bg-gray-100 transition-colors"
          >
            <div className="w-24">
              <span className="font-semibold text-sm text-gray-900">{DAYS[i]}</span>
            </div>
            <label className="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={day.is_open}
                onChange={(e) => updateDay(i, "is_open", e.target.checked)}
                className="w-4 h-4 rounded border-gray-300 accent-red-600"
              />
              <span className="text-sm text-gray-600">Aberto</span>
            </label>
            {day.is_open && (
              <div className="flex items-center gap-2 ml-auto">
                <input
                  type="time"
                  value={day.open_time}
                  onChange={(e) => updateDay(i, "open_time", e.target.value)}
                  className="px-3 py-1.5 bg-white border border-gray-200 rounded-lg text-sm"
                />
                <span className="text-gray-400 text-sm">às</span>
                <input
                  type="time"
                  value={day.close_time}
                  onChange={(e) => updateDay(i, "close_time", e.target.value)}
                  className="px-3 py-1.5 bg-white border border-gray-200 rounded-lg text-sm"
                />
              </div>
            )}
          </div>
        ))}
      </div>

      <div className="mt-6 flex justify-end">
        <button
          onClick={saveHours}
          disabled={saving}
          className="flex items-center gap-2 px-6 py-2.5 rounded-xl text-white font-semibold text-sm transition-all duration-200 hover:shadow-lg disabled:opacity-50"
          style={{ background: "linear-gradient(135deg, #EA1D2C, #C41420)" }}
        >
          {saving ? (
            <>
              <FiLoader className="animate-spin h-4 w-4" />
              Salvando...
            </>
          ) : (
            <>
              <FiSave className="h-4 w-4" />
              Salvar Horários
            </>
          )}
        </button>
      </div>
    </div>
  );
};

export default BusinessHoursEditor;
