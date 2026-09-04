// FuuDelivery Load Test — k6
//
// Execução MANUAL (nunca no CI Gate — o alvo é produção):
//   gh workflow run load-test.yml --repo MarcosPavanBR/fuudelivery
//   k6 run scripts/load-test/k6.js            # local (defina API_URL se quiser)
import http from 'k6/http';
import { check, sleep } from 'k6';

export let options = {
  // Carga moderada: produção é free tier do Render.
  stages: [
    { duration: '20s', target: 5 },
    { duration: '30s', target: 20 },
    { duration: '30s', target: 50 },
    { duration: '10s', target: 0 },
  ],
  thresholds: {
    // Realistas para cold-start/free tier: p95 < 1s e <5% de falha.
    http_req_duration: ['p(95)<1000'],
    http_req_failed: ['rate<0.05'],
  },
};

const BASE_URL = __ENV.API_URL || 'https://fuudelivery-api-8y6l.onrender.com';

export default function () {
  let res = http.get(`${BASE_URL}/health`);
  check(res, { 'health OK': (r) => r.status === 200 });
  sleep(0.1);

  res = http.get(`${BASE_URL}/ping`);
  check(res, { 'ping OK': (r) => r.status === 200 });
  sleep(0.05);

  res = http.get(`${BASE_URL}/establishments`);
  check(res, { 'establishments OK': (r) => r.status === 200 });
  sleep(0.1);
}
