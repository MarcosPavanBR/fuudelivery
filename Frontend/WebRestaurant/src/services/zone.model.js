import api from "./api";

const FALLBACK_ZONE_FEE = {
  has_zone: false,
  current_platform_pct: 5.0,
  current_establishment_pct: 85.0,
  at_target: true,
};

export async function getMyZoneFee() {
  try {
    const { data } = await api.get("/establishments/me/zone");
    return data;
  } catch (e) {
    console.error(e);
    return FALLBACK_ZONE_FEE;
  }
}

export default {
  getMyZoneFee,
};
