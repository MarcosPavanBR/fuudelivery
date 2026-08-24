import React from "react";
import { FiChevronLeft, FiChevronRight } from "react-icons/fi";

// Barra de paginacao server-side (Design System Fuu):
// info "X–Y de Z" + botoes Anterior/Proxima + seletor de tamanho de pagina.
// Props: { page, totalPages, total, pageSize, onPageChange, onPageSizeChange }
const PAGE_SIZES = [10, 20, 50];

export default function Pagination({ page, totalPages, total, pageSize, onPageChange, onPageSizeChange }) {
  if (!total) return null;

  const from = total === 0 ? 0 : (page - 1) * pageSize + 1;
  const to = Math.min(page * pageSize, total);
  const canPrev = page > 1;
  const canNext = page < totalPages;

  return (
    <div className="flex flex-col sm:flex-row items-center justify-between gap-3 px-4 py-3 border-t border-gray-100">
      <p className="text-sm text-gray-500">
        <span className="font-semibold text-gray-900">
          {from}–{to}
        </span>{" "}
        de {total}
      </p>

      <div className="flex items-center gap-2">
        <select
          value={pageSize}
          onChange={(e) => onPageSizeChange(Number(e.target.value))}
          className="px-3 py-2 bg-gray-50 border border-gray-200 rounded-lg text-sm text-gray-700 focus:bg-white focus:border-fuu-red transition-all outline-none"
          title="Itens por página"
        >
          {PAGE_SIZES.map((n) => (
            <option key={n} value={n}>
              {n} / página
            </option>
          ))}
        </select>

        <span className="text-sm text-gray-500 px-1">
          Página <span className="font-semibold text-gray-900">{page}</span> de {totalPages || 1}
        </span>

        <button
          onClick={() => canPrev && onPageChange(page - 1)}
          disabled={!canPrev}
          className="inline-flex items-center gap-1 px-3 py-2 rounded-lg text-sm font-medium border border-gray-200 text-gray-700 bg-white transition-all duration-150 hover:bg-gray-50 disabled:opacity-40 disabled:cursor-not-allowed"
        >
          <FiChevronLeft className="h-4 w-4" />
          Anterior
        </button>
        <button
          onClick={() => canNext && onPageChange(page + 1)}
          disabled={!canNext}
          className="inline-flex items-center gap-1 px-3 py-2 rounded-lg text-sm font-medium text-white transition-all duration-150 hover:bg-fuu-red-dark disabled:opacity-40 disabled:cursor-not-allowed"
          style={{ background: "#EA1D2C", boxShadow: "0 2px 8px rgba(234,29,44,0.25)" }}
        >
          Próxima
          <FiChevronRight className="h-4 w-4" />
        </button>
      </div>
    </div>
  );
}
