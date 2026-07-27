import React, { useState, useRef } from "react";
import api from "../../services/api";

interface ImageUploaderProps {
  entity: "products" | "categories" | "restaurants" | "additionals" | "reviews";
  entityId?: number;
  onUploadComplete: (url: string) => void;
  currentImage?: string;
  label?: string;
}

const ImageUploader: React.FC<ImageUploaderProps> = ({
  entity,
  entityId,
  onUploadComplete,
  currentImage,
  label = "Upload de Imagem",
}) => {
  const [uploading, setUploading] = useState(false);
  const [preview, setPreview] = useState(currentImage || "");
  const [error, setError] = useState("");
  const fileInputRef = useRef<HTMLInputElement>(null);

  const handleFileSelect = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    // Valida tipo
    if (!file.type.startsWith("image/")) {
      setError("Apenas imagens são permitidas");
      return;
    }

    // Valida tamanho (5MB)
    if (file.size > 5 * 1024 * 1024) {
      setError("Arquivo muito grande. Máximo: 5MB");
      return;
    }

    setError("");
    setUploading(true);

    try {
      const formData = new FormData();
      formData.append("file", file);

      const url = entityId
        ? `/upload/${entity}/${entityId}`
        : `/upload/${entity}`;

      const response = await api.post(url, formData, {
        headers: { "Content-Type": "multipart/form-data" },
      });

      const imageUrl = response.data.url;
      setPreview(imageUrl);
      onUploadComplete(imageUrl);
    } catch (err: any) {
      setError(
        err.response?.data?.error || "Erro ao fazer upload. Tente novamente."
      );
    } finally {
      setUploading(false);
    }
  };

  return (
    <div className="image-uploader">
      <label className="image-uploader__label">{label}</label>

      <div
        className="image-uploader__preview"
        onClick={() => fileInputRef.current?.click()}
        role="button"
        tabIndex={0}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            fileInputRef.current?.click();
          }
        }}
      >
        {preview ? (
          <img
            src={preview}
            alt="Preview"
            className="image-uploader__img"
          />
        ) : (
          <div className="image-uploader__placeholder">
            {uploading ? (
              <span className="image-uploader__loading">Enviando...</span>
            ) : (
              <>
                <svg
                  width="40"
                  height="40"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                >
                  <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
                  <polyline points="17 8 12 3 7 8" />
                  <line x1="12" y1="3" x2="12" y2="15" />
                </svg>
                <span>Clique para selecionar</span>
              </>
            )}
          </div>
        )}
      </div>

      <input
        ref={fileInputRef}
        type="file"
        accept="image/jpeg,image/png,image/gif,image/webp,image/svg+xml"
        onChange={handleFileSelect}
        style={{ display: "none" }}
      />

      {error && <p className="image-uploader__error">{error}</p>}
    </div>
  );
};

export default ImageUploader;
