// Id da LOJA da sessão (resposta de /auth/session: establishment_id, e o
// objeto establishment quando a loja foi carregada). Nunca o id do usuário:
// usuário e loja são sequências diferentes, e usar user.id criava produto,
// categoria e adicional "na loja" errada — o servidor recusava com 403.
export function establishmentIdOf(user) {
  const id = Number(user?.establishment_id || user?.establishment?.id || 0);
  return id > 0 ? id : null;
}
