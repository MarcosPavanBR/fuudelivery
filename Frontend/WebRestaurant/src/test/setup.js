// =============================================================
// src/test/setup.js — Setup global do vitest (WebRestaurant)
// =============================================================
// O jsdom não tem o elemento #root que o index.html cria em produção.
//
// IMPORTANTE: a injeção acontece no ESCOPO DO MÓDULO (não em beforeEach)
// porque react-modal executa `Modal.setAppElement("#root")` no momento em
// que o módulo é importado — se o #root só existisse dentro de beforeEach,
// o import de CardapioEditModal/ModalAddItens quebraria antes do teste rodar.
import "@testing-library/jest-dom/vitest";

if (!document.getElementById("root")) {
  const root = document.createElement("div");
  root.id = "root";
  document.body.appendChild(root);
}
