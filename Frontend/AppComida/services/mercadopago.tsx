import { MERCADO_PAGO_PUBLIC_KEY } from "@/config/config";

export const generateCardToken = async (
  cardNumber: string,
  cardHolderName: string,
  expMonth: number,
  expYear: number,
  cardCVV: string,
  publicKey?: string
): Promise<string | null> => {
  const pk = publicKey || MERCADO_PAGO_PUBLIC_KEY;
  return `card_token_simulated_${Date.now()}`;
};

export { MERCADO_PAGO_PUBLIC_KEY };
