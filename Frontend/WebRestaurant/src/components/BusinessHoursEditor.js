import React, { useState, useEffect } from "react";
import api from "../services/api";

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

  useEffect(() => { loadHours(); }, []);

  const loadHours = async () => {
    try {
      const { data } = await api.get(`/api/auth/establishments/${establishmentId}/hours`);
      if (data.length > 0) {
        const merged = hours.map(h => {
          const existing = data.find(d => d.day_of_week === h.day_of_week);
          return existing || h;
        });
        setHours(merged);
      }
    } catch (e) { console.error(e); }
  };

  const updateDay = (index, field, value) => {
    const updated = [...hours];
    updated[index] = { ...updated[index], [field]: value };
    setHours(updated);
  };

  const saveHours = async () => {
    setSaving(true);
    try {
      await api.post("/api/auth/establishments/hours/bulk", hours.map(h => ({
        ...h,
        establishment_id: establishmentId,
      })));
      alert("Horários salvos!");
    } catch (e) { alert("Erro ao salvar"); }
    setSaving(false);
  };

  return (
    <div style={{ padding: 20, maxWidth: 600 }}>
      <h2 style={{ marginBottom: 20 }}>Horário de Funcionamento</h2>
      {hours.map((day, i) => (
        <div key={i} style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 10, padding: 10, background: "#F9FAFB", borderRadius: 8 }}>
          <div style={{ width: 100, fontWeight: 600 }}>{DAYS[i]}</div>
          <label style={{ display: "flex", alignItems: "center", gap: 4, cursor: "pointer" }}>
            <input type="checkbox" checked={day.is_open} onChange={e => updateDay(i, "is_open", e.target.checked)} />
            Aberto
          </label>
          {day.is_open && (
            <>
              <input type="time" value={day.open_time} onChange={e => updateDay(i, "open_time", e.target.value)} style={inputStyle} />
              <span>às</span>
              <input type="time" value={day.close_time} onChange={e => updateDay(i, "close_time", e.target.value)} style={inputStyle} />
            </>
          )}
        </div>
      ))}
      <button onClick={saveHours} disabled={saving} style={{ padding: "10px 24px", background: "#F97316", color: "#FFF", border: "none", borderRadius: 6, cursor: "pointer", fontWeight: 600 }}>
        {saving ? "Salvando..." : "Salvar Horários"}
      </button>
    </div>
  );
};

const inputStyle = {
  padding: "6px 10px",
  border: "1px solid #D1D5DB",
  borderRadius: 4,
  fontSize: 14,
};

export default BusinessHoursEditor;
