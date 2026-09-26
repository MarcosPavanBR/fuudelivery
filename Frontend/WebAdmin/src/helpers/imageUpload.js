import api from "../services/api";

// Upload de imagem com redução no próprio navegador.
//
// Foto de celular tem 3–8 MB; o site recusava acima de 2 MB (avatar) ou
// 5 MB, e o upload "não funcionava". Aqui a imagem é redimensionada para o
// lado maior caber em `maxSide` e vira JPEG — fica em centenas de KB, sobe
// rápido e o servidor aceita. SVG nunca (o servidor recusa por segurança).

// Tamanho final mantendo a proporção (testado em __tests__/imageUpload.test.js).
export function fitSize(width, height, maxSide) {
  if (!width || !height) return { width: 0, height: 0 };
  const scale = Math.min(1, maxSide / Math.max(width, height));
  return { width: Math.round(width * scale), height: Math.round(height * scale) };
}

export function checkImageFile(file) {
  if (!file) return "Nenhum arquivo selecionado.";
  const type = (file.type || "").toLowerCase();
  if (type.includes("svg")) return "SVG não é aceito. Envie JPG, PNG ou WEBP.";
  if (type && !type.startsWith("image/")) return "Selecione um arquivo de imagem.";
  if (file.size > 25 * 1024 * 1024) return "Imagem muito grande (máx. 25 MB).";
  return null;
}

function loadImage(file) {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(file);
    const img = new Image();
    img.onload = () => {
      URL.revokeObjectURL(url);
      resolve(img);
    };
    img.onerror = () => {
      URL.revokeObjectURL(url);
      reject(new Error("formato"));
    };
    img.src = url;
  });
}

export async function compressImage(file, maxSide = 1200, quality = 0.85) {
  let img;
  try {
    img = await loadImage(file);
  } catch {
    // Ex.: HEIC do iPhone no Chrome/Windows — o navegador não decodifica.
    throw new Error("Não foi possível ler esta imagem. Envie em JPG ou PNG (no iPhone: Ajustes › Câmera › Formatos › Mais Compatível).");
  }
  const { width, height } = fitSize(img.naturalWidth, img.naturalHeight, maxSide);
  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;
  const ctx = canvas.getContext("2d");
  ctx.fillStyle = "#FFFFFF"; // PNG transparente vira fundo branco no JPEG
  ctx.fillRect(0, 0, width, height);
  ctx.drawImage(img, 0, 0, width, height);
  const blob = await new Promise((resolve) => canvas.toBlob(resolve, "image/jpeg", quality));
  if (!blob) throw new Error("Não foi possível processar a imagem.");
  return blob;
}

// Envia para /upload/<path> e devolve a URL pública.
export async function uploadImage(path, file, maxSide = 1200) {
  const problem = checkImageFile(file);
  if (problem) throw new Error(problem);
  const blob = await compressImage(file, maxSide);
  const name = (file.name || "imagem").replace(/\.[^.]+$/, "") + ".jpg";
  const fd = new FormData();
  fd.append("file", blob, name);
  try {
    const { data } = await api.post(`/upload/${path}`, fd, {
      headers: { "Content-Type": "multipart/form-data" },
      timeout: 60000,
    });
    if (!data?.url) throw new Error("O servidor não devolveu o endereço da imagem.");
    return data.url;
  } catch (err) {
    const status = err?.response?.status;
    if (status === 503) throw new Error("Upload de imagens desativado no servidor (armazenamento não configurado). Avise o suporte.");
    throw new Error(err?.response?.data?.error || err.message || "Erro ao enviar a imagem.");
  }
}
