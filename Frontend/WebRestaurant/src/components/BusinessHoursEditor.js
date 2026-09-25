import React from "react";
import { FiClock } from "react-icons/fi";
import { DAYS } from "../pages/perfil/storeSettings";

// Grade semanal de funcionamento. Controlado pela página Ajustes: quem salva
// é o botão único "Salvar alterações" (antes havia um segundo botão só para
// os horários, e um campo de texto "Horário Funcionamento" que repetia isto).
const BusinessHoursEditor = ({ hours, onChange }) => {
  const updateDay = (index, field, value) => {
    const updated = [...hours];
    updated[index] = { ...updated[index], [field]: value };
    onChange(updated);
  };

  return (
    <div className="card p-6">
      <div className="flex items-center gap-2 mb-4">
        <div className="p-2 rounded-lg bg-red-50">
          <FiClock className="h-5 w-5" style={{ color: "#DC2626" }} />
        </div>
        <h3 className="text-lg font-bold text-gray-900 dark:text-white">Horário de Funcionamento</h3>
      </div>

      <div className="space-y-2">
        {hours.map((day, i) => (
          <div
            key={day.day_of_week}
            className="flex flex-wrap items-center gap-3 p-3 rounded-xl bg-gray-50 dark:bg-gray-800"
          >
            <span className="w-20 font-semibold text-sm text-gray-900 dark:text-white">{DAYS[day.day_of_week]}</span>
            <label className="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={!!day.is_open}
                onChange={(e) => updateDay(i, "is_open", e.target.checked)}
                className="w-4 h-4 rounded border-gray-300 accent-red-600"
              />
              <span className="text-sm text-gray-600 dark:text-gray-300">{day.is_open ? "Aberto" : "Fechado"}</span>
            </label>
            {day.is_open && (
              <div className="flex items-center gap-2 ml-auto">
                <input
                  type="time"
                  aria-label={`Abertura ${DAYS[day.day_of_week]}`}
                  value={day.open_time}
                  onChange={(e) => updateDay(i, "open_time", e.target.value)}
                  className="px-3 py-1.5 bg-white border border-gray-200 rounded-lg text-sm"
                />
                <span className="text-gray-400 text-sm">às</span>
                <input
                  type="time"
                  aria-label={`Fechamento ${DAYS[day.day_of_week]}`}
                  value={day.close_time}
                  onChange={(e) => updateDay(i, "close_time", e.target.value)}
                  className="px-3 py-1.5 bg-white border border-gray-200 rounded-lg text-sm"
                />
              </div>
            )}
          </div>
        ))}
      </div>
    </div>
  );
};

export default BusinessHoursEditor;
